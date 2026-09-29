# Security

Report vulnerabilities privately through GitHub's [security advisory form](https://github.com/Layerrail/openstead-mcp/security/advisories/new). Do not include live keys or customer logs in public issues.

The server runs as one local stdio process for one Openstead workspace. The API key is loaded from an explicit environment variable/private file or the existing CLI keychain. It is never an MCP tool argument or resource. The API origin and workspace are fixed at startup; there is no arbitrary HTTP or filesystem tool. HTTP redirects are not followed. Responses and input bodies are bounded. Current workspace membership, role, MFA policy and entitlements are enforced by Django on every API request.

Write tools are absent by default. Workload execution and secret-setting need additional startup opt-ins. These are capability boundaries, not substitutes for host approval. An assistant can supply an exact-name confirmation itself, so the MCP host must obtain human authorization for changes. Use a read-only API key for observation.

Treat logs, repository metadata, configuration and other returned content as untrusted data. The server does not evaluate it. Known credential fields and Openstead API keys are redacted, but arbitrary application secrets cannot be reliably identified in free-form logs. Secret-setting arguments are visible to the MCP host.

Revoke a reused CLI key with `openstead logout` or the dashboard. Stop the MCP server when changing accounts or trust contexts. No credentials, payment methods or billing exemptions are created by the server. Do not expose stdio through an unauthenticated network bridge; hosted remote MCP requires a separate OAuth/resource-server design.
