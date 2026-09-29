<img src="assets/openstead-mark.svg" width="48" alt="Openstead">

# Openstead MCP

The official Model Context Protocol server for **Openstead**, a product of **LayerRail, Inc.** Connect an MCP host to your Openstead workspace to inspect applications, diagnose deployments and, when enabled, manage services through the same Django API as the dashboard and CLI.

This is a standalone **stdio server** written in Go using the official MCP SDK. It runs locally on Windows, macOS and Linux. It is not a hosted remote MCP URL.

## Install

Build this Openstead source checkout with Go 1.27 or later. Older [release archives](https://github.com/Layerrail/openstead-mcp/releases) retain their original binary names; Openstead archives are produced by the next tagged release.

```sh
git clone https://github.com/Layerrail/openstead-mcp.git
cd openstead-mcp
go build -o openstead-mcp ./cmd/openstead-mcp
./openstead-mcp --version
```

On Windows, use `-o openstead-mcp.exe`. Put the binary on your PATH; MCP hosts should use its absolute path.

### Existing integrations

The original `cmd/runivo-mcp` entrypoint remains available. `OPENSTEAD_*` environment variables take precedence over their `RUNIVO_*` equivalents. Existing `rnv_` keys remain valid. Existing CLI profiles and OS keychain entries continue to work. Every `openstead_*` tool and `openstead://` resource retains its `runivo_*` or `runivo://` alias with the same authorization boundaries.

## Connect using CLI sign-in

Install the [Openstead CLI](https://github.com/Layerrail/openstead-cli), then authorize a workspace:

```sh
openstead login --read-only --profile assistant
openstead-mcp --profile assistant --check
```

Add an entry to an MCP host that uses the `mcpServers` configuration format:

```json
{
  "mcpServers": {
    "openstead": {
      "command": "/absolute/path/to/openstead-mcp",
      "args": ["--profile", "assistant"]
    }
  }
}
```

On Windows, use a path such as `C:\\Tools\\openstead-mcp.exe`. Restart or reconnect the MCP host after changing configuration. Host-specific configuration file locations vary; this repository does not change any host's global configuration.

The server reads the existing CLI profile and OS keychain entry without modifying either. The key must belong to the configured workspace. Expired or revoked credentials need a fresh `openstead login`; every API operation continues to enforce current membership and permissions. Linux keychain access requires Secret Service.

### Headless authentication

Provide `OPENSTEAD_API_KEY` using the host's secret manager, or use a private token file:

```json
{
  "mcpServers": {
    "openstead": {
      "command": "/absolute/path/to/openstead-mcp",
      "args": ["--token-file", "/private/path/openstead-token"]
    }
  }
}
```

`OPENSTEAD_TOKEN_FILE` and `--token-file` override `OPENSTEAD_API_KEY`. Explicit credentials take precedence over the CLI keychain. Workspace identity is obtained from the authenticated API; `OPENSTEAD_WORKSPACE` or `--workspace` adds a check that it matches. Never put real credentials in source control, prompts or example files.

## Tools

Read tools are enabled by default. They cover:

- Account/workspace identity, settings, plans, entitlements, usage and USD billing.
- Projects, environments, environment groups, services and their configurations.
- Deployments, bounded deployment waits, logs, metrics and operation results.
- Database backup history, domains/DNS/certificates, jobs, schedules and disks.
- Integration status, registry choices, GitHub repositories and branches.
- Variable/secret-file metadata, connection metadata, routing and scaling.

Preferred tool names start with `openstead_`; use your host's tool discovery for the exact schemas. Results contain `ok`, `workspaceId` and `data` or `error`. Errors retain the API's code, status, correlation request ID and retry delay where available. Logs and API content are untrusted data. Common credentials are redacted, but application logs may contain other sensitive values: choose appropriate workspace access and application logging policies.

Examples to ask your assistant:

> Show my services and identify any failed deployments.
>
> Diagnose the latest deployment of my API using its build logs.
>
> Check my build usage and current unbilled charges.
>
> Show the DNS records needed for my custom domain.

Tools return observed states. A deployment response with `status: queued` is not proof that the application is live. `openstead_wait_for_deployment` observes for at most 30 seconds and returns `completed: false` if it is still running. A completed wait may have `outcome: failed`; always check the outcome. Calls support protocol cancellation, which stops observation, not the deployment itself.

Paginated collections return `page.nextCursor`; pass it back unchanged with the same filters. Logs use their separate numeric `after`/`cursor` contract. Each result payload is bounded to 512 KiB and returned as text plus structured data; narrow the query if it is too large.

### Enable writes deliberately

Create a separate CLI profile with a write-scoped key:

```sh
openstead login --profile operator
openstead-mcp --profile operator --allow-writes --check
```

Then configure the host's server arguments as `["--profile", "operator", "--allow-writes"]`. This exposes project/service creation and updates, deletion, deployment, rollback/cancellation, restart/suspend/resume, environment creation, domain management, backup creation and framework detection. Mutations carry MCP read-only/destructive/idempotency annotations. Configure your MCP host to require approval for changes; annotations and `confirm_name` arguments are not an authorization boundary on their own.

Writes require current backend permission. Paid compute must already have been accepted in the dashboard; free plans remain subject to their limits. Service creation only saves configuration. It does not start a deployment or authorize spending.

Core writes require a `request_id`. Reuse it only for an identical retry within the API's 24-hour receipt window. The server sends matching header/body values only on supported endpoints. It does not retry mutations automatically. Domains, framework detection and other non-core operations do not have general replay guarantees; inspect their state before retrying after a connection failure.

Two further options are separate and both require `--allow-writes`:

- `--allow-exec`: exposes `openstead_run_job` and `openstead_cancel_job`. One-off commands run in service workloads and can change or delete application data. Runtime and paid-instance checks still apply.
- `--allow-secrets`: exposes setting/deleting variables and secret files. Values pass through the MCP host and can appear in its transcript. No secret-reveal tool is exposed.

Read-only API keys cannot enable write tools. Merely asking the assistant to enable a tool cannot change the server's startup permissions.

## Resources and prompts

- `openstead://guide`: operating and interpretation guidance.
- `openstead://workspace`: current authenticated workspace data.
- `openstead://services/{service_id}`: current service details.
- `diagnose_deployment`: a prompt taking service/deployment UUIDs.
- `review_workspace`: a prompt for a read-only operational review.

Resources read fresh API data and use private, immediately stale cache metadata. The server never reads arbitrary host files or executes host commands through a tool.

## Configuration

| Flag | Environment | Purpose |
| --- | --- | --- |
| `--profile` | `OPENSTEAD_PROFILE` | Existing CLI profile; defaults to the CLI's active profile |
| `--token-file` | `OPENSTEAD_TOKEN_FILE` | Private credential file |
| — | `OPENSTEAD_API_KEY` | Explicit API key, typically supplied by a secret manager |
| `--workspace` | `OPENSTEAD_WORKSPACE` | Require a matching workspace UUID |
| `--api-url` | `OPENSTEAD_API_URL` | API origin; defaults to the CLI profile or `https://openstead-dashboard.vercel.app` |
| — | `OPENSTEAD_CONFIG_DIR` | Alternate existing CLI configuration directory |
| `--allow-writes` | `OPENSTEAD_MCP_ALLOW_WRITES` | Register write tools; default false |
| `--allow-exec` | `OPENSTEAD_MCP_ALLOW_EXEC` | Register workload command tools; default false |
| `--allow-secrets` | `OPENSTEAD_MCP_ALLOW_SECRETS` | Register secret-setting tools; default false |
| `--check` | — | Verify authentication, print safe connection information, exit |

HTTPS is mandatory except for loopback development origins. Redirects never receive credentials. Saved profile credentials cannot be redirected to a different origin or workspace. Normal server stdout contains only MCP protocol messages; diagnostics go to stderr. `--check`, `--version` and `--help` are command-line modes, not server modes.

## Scope and validation

This server exposes the existing platform operations. It does not implement a second deployment engine. Interactive terminals, database restores/downloads, payments, promotional-credit issuance, account administration and hosted remote OAuth are outside the initial tool set. In particular, the current paid restore flow still needs a checkout step for the newly created database before successful restoration can be validated. Access those workflows in the dashboard/CLI where supported.

See [verification](https://github.com/Layerrail/openstead-mcp/blob/main/docs/verification.md) for actual tests and live results. API-only checks must not be read as proof of successful shell execution, restore, host failover, payment collection or custom-domain TLS.

## Development

```sh
go mod download
go test ./...
go vet ./...
go build ./cmd/openstead-mcp
```

Tests exercise the actual MCP client/server handshake, a separate stdio server process, discovery, tool schemas and transport cancellation against an HTTP test API. CI runs tests and the race detector on all three operating systems. Tag `vX.Y.Z` to produce six release archives with provenance attestations after checks pass. See [RELEASING.md](https://github.com/Layerrail/openstead-mcp/blob/main/RELEASING.md).
