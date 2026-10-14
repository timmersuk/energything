# Logthing template evidence

Inspected 14 October 2026 in the local sibling logthing workspace. These are
reference observations; all energything changes remain in this repository.

- `.github/workflows/ci.yml`: Linux amd64/arm64 images; Docker Hub image variable,
  synthetic Go tests before image build, release/tag publishing and read-only
  default workflow permissions.
- `Dockerfile`: Debian slim, non-root UID 10001, target-platform binary layout,
  persistent /data and exposed HTTP port.
- `docker-compose.yml`: one service, configurable image/host data directory,
  local /data bind mount and restart unless-stopped.
- `README.md`: local build/run, Compose and multiarchitecture publishing conventions.

Apply that deployment pattern to energything. The milestone plan deliberately
uses Alpine instead of the template's Debian image following user approval on
14 October 2026. The SQLite runtime gate is reverified in both Alpine images. It
uses embedded Go templates without logthing's Node/pnpm frontend build; exposes
only the needed HTTP port; requires credentials without default passwords; and
uses SQLite rather than logthing's NDJSON storage. Validate TLS roots, timezone
data and SQLite compatibility explicitly instead of assuming the template supplies them.

The release matrix supplies the architecture direction, so no architecture choice
is needed in this interview. Actual host runtime verification belongs in acceptance.
Docker Hub is the inherited registry convention; image naming and credentials are
implementation/runtime configuration. Local-filesystem suitability is a deployment
check, not a reason to request private host paths in this planning session.

## Additional inspected references

- `Makefile`: prebuilt bin/{os}/{arch} layout, build ID linker value and Docker
  wrapper targets. Remove frontend prerequisites and syslogsend in energything;
  ensure documented docker-build creates its required binaries first.
- `cmd/server/main.go`: signal cancellation, http.Server, bounded shutdown and
  resource lifecycle. Adapt with explicit collector joins and storage close ordering.
- `internal/api/auth.go`: required nonempty credentials and Basic Auth checks.
- `internal/notification/discord.go` and incident worker: inspected to ground the
  optional alert discussion. Human decision is to defer Discord entirely; no
  notifier code, webhook configuration or delivery state is adopted in milestone 1.

Workbench examples inspected for execution-brief detail: task schema and the
Docker/CI and graceful-shutdown tasks' context.yaml/plan.yaml. Useful structure
is explicit files/roles, ordered work, concrete verification, risks and unresolved
gates. Their workflow storage and application architecture are unrelated here.
