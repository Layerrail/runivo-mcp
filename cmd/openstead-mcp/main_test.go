package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioProcess(t *testing.T) {
	if os.Getenv("OPENSTEAD_MCP_TEST_CHILD") == "1" {
		if err := run(context.Background(), nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	const workspace = "11111111-1111-4111-8111-111111111111"
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rnv_test_process" {
			t.Error("missing bearer")
		}
		switch r.URL.Path {
		case "/api/v1/cli/session":
			io.WriteString(w, `{"scope":"read","workspace":{"id":"`+workspace+`","name":"test","role":"viewer"}}`)
		case "/api/v1/workspaces/" + workspace + "/services":
			io.WriteString(w, `{"services":[],"page":{"hasMore":false}}`)
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer apiServer.Close()
	command := exec.Command(os.Args[0], "-test.run=^TestStdioProcess$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "RUNIVO_") && !strings.HasPrefix(entry, "OPENSTEAD_") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "OPENSTEAD_MCP_TEST_CHILD=1", "OPENSTEAD_API_KEY=rnv_test_process", "OPENSTEAD_API_URL="+apiServer.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "process-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "openstead_list_services", Arguments: map[string]any{"limit": 1}})
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if !tool.Annotations.ReadOnlyHint {
			t.Fatal("write capability exposed in default process")
		}
	}
}
