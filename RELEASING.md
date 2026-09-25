# Releases

1. Update CHANGELOG.md and docs/verification.md with the scope actually verified.
2. Run `go test ./...`, `go vet ./...` and `gofmt -l cmd internal`.
3. Commit and push the reviewed source to `main`.
4. Create and push a new immutable semantic tag, for example `v0.1.0`.

GitHub Actions runs tests/race detection on Linux, macOS and Windows. The tag's release job cross-compiles amd64/arm64 archives, produces SHA-256 checksums, adds GitHub provenance attestations and publishes the release. Never move an existing tag after release; use a patch version. The workflow needs only this repository's GitHub token. No npm, PyPI or infrastructure secrets are needed.

Before announcing the release, download an archive, compare its checksum, and check the executable's embedded version/commit. Separate protocol/API tests from real workload acceptance and name any remaining prerequisites explicitly.
