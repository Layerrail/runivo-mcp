package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"

	"github.com/Layerrail/runivo-mcp/internal/api"
	"github.com/Layerrail/runivo-mcp/internal/auth"
	"github.com/Layerrail/runivo-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"
var commit = "unknown"

func run(ctx context.Context, args []string) error {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	o := auth.Options{APIURL: os.Getenv("RUNIVO_API_URL"), Workspace: os.Getenv("RUNIVO_WORKSPACE"), Profile: os.Getenv("RUNIVO_PROFILE"), TokenFile: os.Getenv("RUNIVO_TOKEN_FILE"), Token: os.Getenv("RUNIVO_API_KEY"), ConfigDir: os.Getenv("RUNIVO_CONFIG_DIR")}
	flags := flag.NewFlagSet("runivo-mcp", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	for name, target := range map[string]*bool{"RUNIVO_MCP_ALLOW_WRITES": &o.AllowWrites, "RUNIVO_MCP_ALLOW_EXEC": &o.AllowExec, "RUNIVO_MCP_ALLOW_SECRETS": &o.AllowSecrets} {
		if value := os.Getenv(name); value != "" {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("%s must be true or false", name)
			}
			*target = parsed
		}
	}
	flags.StringVar(&o.APIURL, "api-url", o.APIURL, "Runivo HTTPS API origin")
	flags.StringVar(&o.Workspace, "workspace", o.Workspace, "Workspace UUID; must match the credential")
	flags.StringVar(&o.Profile, "profile", o.Profile, "Existing Runivo CLI profile")
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
		fmt.Printf("runivo-mcp %s (%s)\n", version, commit)
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

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil && err != flag.ErrHelp && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Runivo MCP:", err)
		os.Exit(1)
	}
}
