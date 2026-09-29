# Openstead MCP

Use the official MCP Go SDK and the existing Django API. Keep credentials out of tools, resources, prompts, stdout, logs, and committed fixtures. Stdout is exclusively MCP protocol traffic. Default to read-only tools; write and command-execution capabilities require explicit startup options. Preserve workspace boundaries, backend permissions and billing requirements. Never bypass engine authorization or add direct database/SSH/Docker control.

Run `go test ./...`, `go vet ./...` and `gofmt` before release. Test the actual MCP handshake and tool calls, not only handler functions. Live test documentation must distinguish API acknowledgments from completed workload operations.
