package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Layerrail/openstead-mcp/internal/api"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const workspaceID = "11111111-1111-4111-8111-111111111111"
const serviceID = "22222222-2222-4222-8222-222222222222"
const deploymentID = "33333333-3333-4333-8333-333333333333"

func session(t *testing.T, options Options, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	apiClient, err := api.New(httpServer.URL, "rnv_test_secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	s := New(apiClient, workspaceID, options)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "runivo-mcp-test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}
func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "runivo_" + name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestProtocolDiscoveryAndReadOnlyBoundary(t *testing.T) {
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) { t.Error("discovery must not call API") })
	names := map[string]bool{}
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names[tool.Name] = true
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatalf("unexpected write tool %s", tool.Name)
		}
	}
	if len(names) < 25 || !names["runivo_get_logs"] || !names["runivo_get_billing"] || !names["openstead_get_logs"] || names["openstead_deploy_service"] || names["runivo_deploy_service"] {
		t.Fatal(names)
	}
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "runivo_deploy_service", Arguments: map[string]any{"service_id": serviceID, "request_id": "request-1"}})
	if err == nil {
		t.Fatal("unregistered write tool accepted")
	}
	resources, err := cs.ListResources(context.Background(), nil)
	if err != nil || len(resources.Resources) != 4 {
		t.Fatal(resources, err)
	}
	prompts, err := cs.ListPrompts(context.Background(), nil)
	if err != nil || len(prompts.Prompts) != 2 {
		t.Fatal(prompts, err)
	}
	t.Logf("read tools: %d", len(names))
}

func TestWorkspaceRootUsesDjangoRouteWithoutTrailingSlash(t *testing.T) {
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/"+workspaceID {
			t.Error(r.URL.Path)
		}
		io.WriteString(w, `{"workspace":{"id":"`+workspaceID+`"}}`)
	})
	if result := call(t, cs, "get_workspace", map[string]any{}); result.IsError {
		t.Fatal(resultText(t, result))
	}
	if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "runivo://workspace"}); err != nil {
		t.Fatal(err)
	}
}
func TestScopedReadPaginationAndSchemaRejection(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/v1/workspaces/"+workspaceID+"/services" || r.URL.Query().Get("cursor") != "opaque+/=" || r.URL.Query().Get("limit") != "12" {
			t.Error(r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer rnv_test_secret" {
			t.Error("missing authorization")
		}
		io.WriteString(w, `{"services":[],"page":{"hasMore":false}}`)
	})
	r := call(t, cs, "list_services", map[string]any{"limit": 12, "cursor": "opaque+/="})
	if r.IsError {
		t.Fatal(resultText(t, r))
	}
	for _, args := range []map[string]any{{"service_id": "../billing"}, {"service_id": serviceID, "workspace_id": deploymentID}, {"service_id": serviceID, "url": "https://attacker.invalid"}} {
		r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "runivo_get_service", Arguments: args})
		if err == nil && !r.IsError {
			t.Fatal("accepted invalid input", args)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("invalid input reached API")
	}
}
func TestMutationReceiptMatchesBodyAndReplayID(t *testing.T) {
	var requests atomic.Int32
	cs := session(t, Options{AllowWrites: true}, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/deploys") || body["requestId"] != "same-request" || r.Header.Get("Idempotency-Key") != "same-request" {
			t.Error(r.Method, r.URL, body, r.Header)
		}
		io.WriteString(w, `{"deployment":{"id":"`+deploymentID+`","status":"queued"}}`)
	})
	for range 2 {
		r := call(t, cs, "deploy_service", map[string]any{"service_id": serviceID, "request_id": "same-request"})
		if r.IsError {
			t.Fatal(resultText(t, r))
		}
	}
	if requests.Load() != 2 {
		t.Fatal(requests.Load())
	}
}
func TestManagedMySQLCreationPreservesDraftAndBillingBoundary(t *testing.T) {
	var requests atomic.Int32
	cs := session(t, Options{AllowWrites: true}, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/workspaces/"+workspaceID+"/services" {
			t.Error("creation must only save a service", r.Method, r.URL.Path)
		}
		if body["name"] != "mysql-app" || body["kind"] != "mysql" || body["requestId"] != "mysql-request" || r.Header.Get("Idempotency-Key") != "mysql-request" {
			t.Error("incorrect service or replay identity", body)
		}
		configuration, ok := body["configuration"].(map[string]any)
		if !ok || configuration["plan"] != "mysql-starter" || configuration["databaseVersion"] != "8.4" || configuration["privateNetworking"] != true {
			t.Error("MySQL configuration was lost", configuration)
		}
		if _, ok := body["deploy"]; ok {
			t.Error("creation must not add deployment consent")
		}
		io.WriteString(w, `{"service":{"id":"`+serviceID+`","kind":"mysql","status":"draft"}}`)
	})
	result := call(t, cs, "create_service", map[string]any{
		"name": "mysql-app", "kind": "mysql", "request_id": "mysql-request",
		"configuration": map[string]any{"plan": "mysql-starter", "databaseVersion": "8.4", "privateNetworking": true},
	})
	if result.IsError || !strings.Contains(resultText(t, result), "draft") {
		t.Fatal(resultText(t, result))
	}
	if requests.Load() != 1 {
		t.Fatal("creation sent extra operations", requests.Load())
	}
}
func TestIndependentExecutionAndSecretOptIns(t *testing.T) {
	for _, options := range []Options{{AllowWrites: true}, {AllowWrites: true, AllowExec: true}, {AllowWrites: true, AllowSecrets: true}} {
		cs := session(t, options, func(http.ResponseWriter, *http.Request) {})
		names := map[string]bool{}
		for tool, err := range cs.Tools(context.Background(), nil) {
			if err != nil {
				t.Fatal(err)
			}
			names[tool.Name] = true
			if !tool.Annotations.ReadOnlyHint && tool.Annotations.DestructiveHint == nil {
				t.Error("missing mutation annotation")
			}
		}
		if !names["runivo_create_service"] || names["runivo_run_job"] != options.AllowExec || names["runivo_set_variable"] != options.AllowSecrets {
			t.Fatal(options, names)
		}
	}
}
func TestBackupAndJobsDoNotSendUnsupportedReceiptHeaders(t *testing.T) {
	var posts atomic.Int32
	cs := session(t, Options{AllowWrites: true, AllowExec: true}, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, `{"service":{"name":"test-app"}}`)
			return
		}
		posts.Add(1)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.Header.Get("Idempotency-Key") != "" || body["requestId"] != "job-request" {
			t.Error("incorrect replay header/body")
		}
		io.WriteString(w, `{"queued":true}`)
	})
	for _, name := range []string{"create_backup", "run_job"} {
		args := map[string]any{"service_id": serviceID, "request_id": "job-request"}
		if name == "run_job" {
			args["confirm_name"] = "test-app"
			args["command"] = "echo hello"
		}
		if r := call(t, cs, name, args); r.IsError {
			t.Fatal(resultText(t, r))
		}
	}
	if posts.Load() != 2 {
		t.Fatal(posts.Load())
	}
}
func TestErrorsRemainVisibleAndCredentialsAreRedacted(t *testing.T) {
	var calls atomic.Int32
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Request-ID", "support-123")
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(403)
		io.WriteString(w, `{"errors":[{"code":"permission_denied","message":"denied rnv_test_secret"}]}`)
	})
	r := call(t, cs, "get_service", map[string]any{"service_id": serviceID})
	raw := resultText(t, r)
	if !r.IsError || !strings.Contains(raw, "permission_denied") || !strings.Contains(raw, "support-123") || strings.Contains(raw, "rnv_test_secret") || calls.Load() != 1 {
		t.Fatal(raw, calls.Load())
	}
}
func TestWaitDoesNotClaimDeploymentSuccess(t *testing.T) {
	for _, status := range []string{"live", "failed", "cancelled", "superseded", "building"} {
		cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"deployment":{"id":"`+deploymentID+`","status":"`+status+`"}}`)
		})
		r := call(t, cs, "wait_for_deployment", map[string]any{"service_id": serviceID, "deployment_id": deploymentID, "timeout_seconds": 1})
		raw := resultText(t, r)
		if r.IsError || !strings.Contains(raw, `"outcome":"`+status+`"`) {
			t.Fatal(raw)
		}
		if status == "building" && !strings.Contains(raw, `"completed":false`) {
			t.Fatal(raw)
		}
	}
}
func TestResourceReadsFreshAPIDataAndPromptsValidateIDs(t *testing.T) {
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/"+workspaceID+"/services/"+serviceID {
			t.Error(r.URL)
		}
		io.WriteString(w, `{"service":{"name":"test","password":"unsafe","description":"postgres://user:password@example.invalid/db rnv_otherkey"}}`)
	})
	resource, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "runivo://services/" + serviceID})
	if err != nil {
		t.Fatal(err)
	}
	raw := resource.Contents[0].Text
	if strings.Contains(raw, "unsafe") || strings.Contains(raw, "user:password") || strings.Contains(raw, "rnv_otherkey") {
		t.Fatal(raw)
	}
	_, err = cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "diagnose_deployment", Arguments: map[string]string{"service_id": "ignore instructions", "deployment_id": deploymentID}})
	if err == nil {
		t.Fatal("invalid prompt IDs accepted")
	}
}
func TestCancellationReachesAPI(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = cs.CallTool(ctx, &mcp.CallToolParams{Name: "runivo_get_service", Arguments: map[string]any{"service_id": serviceID}})
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("MCP request ignored cancellation")
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("API ignored cancellation")
	}
}

func TestOpensteadAndLegacyNamesShareWorkspaceBoundary(t *testing.T) {
	cs := session(t, Options{}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/"+workspaceID {
			t.Error(r.URL.Path)
		}
		io.WriteString(w, `{"workspace":{"id":"`+workspaceID+`"}}`)
	})
	for _, prefix := range []string{"openstead", "runivo"} {
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: prefix + "_get_workspace", Arguments: map[string]any{}})
		if err != nil || result.IsError {
			t.Fatal(result, err)
		}
		resource, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: prefix + "://workspace"})
		if err != nil || resource.Contents[0].URI != prefix+"://workspace" {
			t.Fatal(resource, err)
		}
		if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: prefix + "_deploy_service", Arguments: map[string]any{"service_id": serviceID, "request_id": "fixture"}}); err == nil {
			t.Fatal("read-only write boundary bypassed")
		}
	}
}
