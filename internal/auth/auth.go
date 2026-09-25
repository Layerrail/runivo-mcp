// Package auth reads existing CLI credentials without creating or changing them.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Layerrail/runivo-mcp/internal/api"
	"github.com/zalando/go-keyring"
)

var UUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Options struct {
	APIURL, Workspace, Profile, TokenFile, Token, ConfigDir string
	AllowWrites, AllowExec, AllowSecrets                    bool
}
type Profile struct {
	KeyID     string `json:"key_id"`
	APIURL    string `json:"api_url"`
	Workspace string `json:"workspace"`
}
type Identity struct {
	Workspace struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"workspace"`
	Scope string `json:"scope"`
}

func readFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not read credential/configuration file")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("credential/configuration file is unreadable or too large")
	}
	return b, nil
}

func Resolve(o Options) (Options, error) { return resolve(o, keyring.Get) }

func resolve(o Options, lookup func(string, string) (string, error)) (Options, error) {
	if (o.AllowExec || o.AllowSecrets) && !o.AllowWrites {
		return o, errors.New("--allow-exec and --allow-secrets require --allow-writes")
	}
	var p Profile
	if o.Profile != "" || (o.Token == "" && o.TokenFile == "") {
		dir := o.ConfigDir
		if dir == "" {
			base, err := os.UserConfigDir()
			if err != nil {
				return o, errors.New("cannot locate CLI configuration")
			}
			dir = filepath.Join(base, "runivo")
		}
		raw, err := readFile(filepath.Join(dir, "config.json"), 1<<20)
		if err != nil {
			return o, errors.New("sign in with runivo login --read-only, or supply RUNIVO_API_KEY or --token-file")
		}
		var saved struct {
			Active   string             `json:"active"`
			Profiles map[string]Profile `json:"profiles"`
		}
		if json.Unmarshal(raw, &saved) != nil {
			return o, errors.New("invalid Runivo CLI configuration")
		}
		name := o.Profile
		if name == "" {
			name = saved.Active
		}
		var exists bool
		p, exists = saved.Profiles[name]
		if !exists {
			return o, errors.New("CLI profile was not found; sign in with runivo login first")
		}
		if o.APIURL == "" {
			o.APIURL = p.APIURL
		}
		if o.Workspace == "" {
			o.Workspace = p.Workspace
		}
	}
	if o.APIURL == "" {
		o.APIURL = api.DefaultURL
	}
	canonical, err := api.ValidateURL(o.APIURL)
	if err != nil {
		return o, err
	}
	o.APIURL = canonical
	if o.TokenFile != "" {
		raw, err := readFile(o.TokenFile, 4096)
		if err != nil {
			return o, err
		}
		o.Token = strings.TrimSpace(string(raw))
	} else if o.Token == "" {
		if canonical != p.APIURL || o.Workspace != p.Workspace {
			return o, errors.New("saved credentials cannot be used for a different API origin or workspace")
		}
		sum := sha256.Sum256([]byte(p.APIURL + "\n" + p.Workspace + "\n" + p.KeyID))
		token, err := lookup("Runivo CLI", hex.EncodeToString(sum[:]))
		if err != nil {
			return o, errors.New("CLI keychain credential unavailable; sign in again or use --token-file")
		}
		o.Token = token
	}
	o.Token = strings.TrimSpace(o.Token)
	if !strings.HasPrefix(o.Token, "rnv_") || len(o.Token) > 4096 || strings.ContainsAny(o.Token, "\r\n\t ") {
		return o, errors.New("invalid Runivo API key format")
	}
	if o.Workspace != "" && !UUID.MatchString(o.Workspace) {
		return o, errors.New("workspace must be a UUID")
	}
	return o, nil
}

func Verify(ctx context.Context, client *api.Client, o Options) (Identity, error) {
	var identity Identity
	if err := client.Do(ctx, "GET", "/api/v1/cli/session", nil, &identity, ""); err != nil {
		return identity, err
	}
	if !UUID.MatchString(identity.Workspace.ID) {
		return identity, errors.New("Runivo returned an invalid workspace identity")
	}
	if o.Workspace != "" && o.Workspace != identity.Workspace.ID {
		return identity, errors.New("API key belongs to a different workspace")
	}
	if identity.Scope != "read" && identity.Scope != "write" {
		return identity, errors.New("Runivo returned an unknown API key scope")
	}
	if o.AllowWrites && identity.Scope != "write" {
		return identity, errors.New("write tools require a write-scoped API key")
	}
	return identity, nil
}
