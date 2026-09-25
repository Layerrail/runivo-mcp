package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Layerrail/runivo-mcp/internal/api"
)

const workspace = "11111111-1111-4111-8111-111111111111"

func TestExplicitTokenAndCapabilityDependencies(t *testing.T) {
	resolved, err := Resolve(Options{Token: "rnv_test", Workspace: workspace})
	if err != nil || resolved.APIURL != api.DefaultURL {
		t.Fatal(resolved.APIURL, err)
	}
	for _, options := range []Options{{Token: "rnv_test", AllowExec: true}, {Token: "rnv_test", AllowSecrets: true}, {Token: "rnv_test", Workspace: "other"}, {Token: "rnv_test\ninjected"}} {
		if _, err := Resolve(options); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
func TestCLIProfileKeychainCompatibilityAndOriginIsolation(t *testing.T) {
	dir := t.TempDir()
	profile := Profile{KeyID: "key-id", APIURL: "https://api.example.com", Workspace: workspace}
	raw, _ := json.Marshal(map[string]any{"active": "test", "profiles": map[string]Profile{"test": profile}})
	os.WriteFile(filepath.Join(dir, "config.json"), raw, 0600)
	calls := 0
	lookup := func(service, key string) (string, error) {
		calls++
		if service != "Runivo CLI" || len(key) != 64 {
			t.Error(service, key)
		}
		return "rnv_profile", nil
	}
	o, err := resolve(Options{ConfigDir: dir}, lookup)
	if err != nil || o.Workspace != workspace || o.Token != "rnv_profile" {
		t.Fatal(err)
	}
	for _, options := range []Options{{ConfigDir: dir, APIURL: "https://other.example.com"}, {ConfigDir: dir, Workspace: "22222222-2222-4222-8222-222222222222"}} {
		if _, err := resolve(options, lookup); err == nil {
			t.Fatal("profile boundary bypassed")
		}
	}
	if calls != 1 {
		t.Fatal("keychain read despite mismatch")
	}
	actual, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if string(actual) != string(raw) {
		t.Fatal("changed CLI configuration")
	}
}
func TestTokenFileTakesPrecedenceAndIsNotChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	os.WriteFile(path, []byte("rnv_file\n"), 0600)
	o, err := Resolve(Options{Token: "rnv_env", TokenFile: path})
	if err != nil || o.Token != "rnv_file" {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "rnv_file\n" {
		t.Fatal("credential file changed")
	}
}
func TestLiveIdentityScopeAndWorkspaceValidation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"scope":"read","workspace":{"id":"`+workspace+`","name":"test","role":"owner"}}`)
	}))
	defer s.Close()
	client, _ := api.New(s.URL, "rnv_test", "test")
	if _, err := Verify(context.Background(), client, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), client, Options{AllowWrites: true}); err == nil {
		t.Fatal("read key enabled writes")
	}
	if _, err := Verify(context.Background(), client, Options{Workspace: "22222222-2222-4222-8222-222222222222"}); err == nil {
		t.Fatal("cross-workspace key accepted")
	}
}
