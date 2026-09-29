package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/Layerrail/openstead-mcp/internal/api"
	"github.com/Layerrail/openstead-mcp/internal/auth"
	"github.com/Layerrail/openstead-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Run(ctx context.Context, args []string, version, commit string) error {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	o := auth.Options{APIURL: auth.Env("API_URL"), Workspace: auth.Env("WORKSPACE"), Profile: auth.Env("PROFILE"), TokenFile: auth.Env("TOKEN_FILE"), Token: auth.Env("API_KEY"), ConfigDir: auth.Env("CONFIG_DIR")}
	flags := flag.NewFlagSet("openstead-mcp", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	for name, target := range map[string]*bool{"MCP_ALLOW_WRITES": &o.AllowWrites, "MCP_ALLOW_EXEC": &o.AllowExec, "MCP_ALLOW_SECRETS": &o.AllowSecrets} {
		if value := auth.Env(name); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("OPENSTEAD_%s must be true or false", name)
			}
			*target = parsed
		}
	}
	flags.StringVar(&o.APIURL, "api-url", o.APIURL, "Openstead HTTPS API origin")
	flags.StringVar(&o.Workspace, "workspace", o.Workspace, "Workspace UUID; must match the credential")
	flags.StringVar(&o.Profile, "profile", o.Profile, "Existing Openstead CLI profile")
	flags.StringVar(&o.TokenFile, "token-file", o.TokenFile, "Read a credential from a private file")
	flags.BoolVar(&o.AllowWrites, "allow-writes", o.AllowWrites, "Expose mutation tools; requires a write-scoped key")
	flags.BoolVar(&o.AllowExec, "allow-exec", o.AllowExec, "Also expose one-off command execution (requires writes)")
	flags.BoolVar(&o.AllowSecrets, "allow-secrets", o.AllowSecrets, "Also expose secret-setting tools (requires writes)")
	showVersion := flags.Bool("version", false, "Print version and exit")
	check := flags.Bool("check", false, "Verify authentication and print non-secret connection information, then exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments; use --help")
	}
	if *showVersion {
		fmt.Printf("openstead-mcp %s (%s)\n", version, commit)
		return nil
	}
	resolved, err := auth.Resolve(o)
	if err != nil {
		return err
	}
	client, err := api.New(resolved.APIURL, resolved.Token, version)
	if err != nil {
		return err
	}
	identity, err := auth.Verify(ctx, client, resolved)
	if err != nil {
		return err
	}
	if *check {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"apiURL": client.BaseURL, "workspace": identity.Workspace, "scope": identity.Scope, "writes": o.AllowWrites, "execution": o.AllowExec, "secretWrites": o.AllowSecrets})
	}
	s := server.New(client, identity.Workspace.ID, server.Options{Version: version, AllowWrites: o.AllowWrites, AllowExec: o.AllowExec, AllowSecrets: o.AllowSecrets})
	return s.Run(ctx, &mcp.StdioTransport{})
}
