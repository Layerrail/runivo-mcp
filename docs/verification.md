# Verification

## Local protocol and API tests

Verified September 25, 2026 using the official MCP SDK and Go 1.27.

- Real in-memory MCP initialization, tool discovery, schemas, resources and prompts, plus a separate stdio server process.
- Read-only default registration; independent write, execution and secret-setting options.
- Invalid resource IDs, path-injection attempts and workspace override arguments rejected before HTTP requests.
- Workspace-scoped API paths and cursor/query encoding.
- Matching idempotency header/body values on core writes; unsupported headers absent from backup/job requests.
- API permission errors, support request IDs and retry delays preserved without credential leakage.
- Deployment waits correctly distinguish live, failed, cancelled, superseded and still-running states.
- MCP cancellation reaches the in-flight HTTP request.
- Origin/redirect protection, bounded responses, profile-origin/workspace checks and read-key write denial.
- CLI profile files and token files remain unchanged during credential loading.

The live test caught a workspace-root route mismatch (a trailing slash that Django does not accept). The route was fixed and a regression test added for both the workspace tool and resource.

## Live stdio acceptance

The compiled Windows server connected to the production Runivo API using a temporary one-hour key belonging to an existing verified workspace owner. A separate test client used actual stdin/stdout JSON-RPC and negotiated MCP `2025-11-25`.

- All 34 default tools advertised read-only behavior; write/execution tools were absent.
- Real identity, catalog, service pagination, service details, deployment history/state, logs, metrics, usage, USD billing and backup history returned through MCP.
- The deployment wait observed a previously completed real deployment. This MCP run did not enqueue another deployment.
- Static guidance, live workspace/service resources and the diagnostic prompt worked.
- With writes and secret-setting explicitly enabled, 54 tools were advertised; execution tools remained absent. Enabling all supported opt-ins exposes 56 tools.
- A temporary project was created, read, updated and deleted. Replaying the exact create request returned the same project ID.
- A temporary free draft service was created, updated and deleted. No deployment or compute purchase was initiated.
- A non-secret test marker was set through the encrypted-variable API, listed only as metadata with `value: null`, and deleted.
- Temporary project/service fixtures were removed.
- The temporary key was imported into an isolated Runivo CLI profile and the Windows OS keychain. MCP successfully reused that real CLI keychain credential.
- CLI logout revoked the key, removed its keychain/profile entry and deleted its token file. A still-running MCP server then returned HTTP 401 for that revoked key.

## Limits of this verification

MCP tool coverage is not evidence that every cloud operation completed successfully. This test did not run a new build, execute a workload command, restore a database, change real DNS, exercise custom-domain HTTPS, collect a payment, test host failover or validate multiple independent customers. Runtime and plan gates remain enforced by the API.

Interactive terminal and paid database-restore validation remain platform prerequisites from the CLI work. The restore endpoint needs a consent-aware checkout step for its newly created target. This MCP release does not expose restoration or claim that gap is fixed. Hosted remote MCP/OAuth is not included; the released transport is local stdio.

Release CI gates publication on unit/protocol tests, static checks and race detection on Linux, macOS and Windows, followed by six cross-compiled archives, checksums and GitHub provenance.
