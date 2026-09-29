package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Layerrail/openstead-mcp/internal/command"
)

var version = "dev"
var commit = "unknown"

func run(ctx context.Context, args []string) error { return command.Run(ctx, args, version, commit) }

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil && err != flag.ErrHelp && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Openstead MCP:", err)
		os.Exit(1)
	}
}
