# Task 01 execution handoff

Implemented and automatically verified on 14 October 2026. Task 02 was not
started. No provider calls, scheduling, observation schema, migrations, audit
or placeholder success APIs were added. All edits are in energything; logthing
was read only. Existing CONTEXT.md and unrelated planning files were retained.

## Versions and build gate

- Repository/module: github.com/timmersuk/energything, derived from origin.
- Executed Go: go1.26.4 windows/amd64; go.mod compatibility 1.26.0 and
  toolchain go1.26.4.
- SQLite driver modernc.org/sqlite v1.60.1; driver-required libc v1.77.1.
  All transitive versions and checksums are pinned in go.mod/go.sum.
  Driver DSN behavior was checked against [official driver documentation](https://pkg.go.dev/modernc.org/sqlite@v1.60.1).
- Alpine 3.23 multiarchitecture base is pinned to digest
  sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0.
  The user approved replacing Debian on 14 October 2026. Image installs only
  ca-certificates and tzdata through apk (observed 20260909-r0 and 2026e-r0).
  This is a reproducible build procedure; apk package repositories are not
  snapshotted, so byte-for-byte image reproducibility is not claimed.
- Docker Desktop 4.95.0; Engine 29.8.2, linux/amd64. ARM64 executed via emulation,
  confirmed by a Debian linux/arm64 container reporting aarch64.

The SQLite build/runtime gate passed on Linux amd64 and emulated arm64.
Both test binaries were compiled with CGO_ENABLED=0, then actually executed.
After the base-image change, both probes also passed as UID 10001 inside the
actual Alpine application images, without installing libc compatibility packages.
They verified WAL, foreign_keys=1, busy_timeout=5000 on replacement physical
connections; wrote a synthetic value; closed/reopened; and recovered that value.
Tests also confirmed that production Open creates no application tables.

## Files and established contracts

- go.mod/go.sum: pinned module/toolchain/dependencies.
- cmd/server/main.go: configuration, resource wiring, signal cancellation,
  listen errors, bounded HTTP shutdown and storage close last.
- internal/config: typed global and provider configuration. Credentials are
  required without defaults; invalid global configuration fails before serving.
  Providers have disabled/valid/error states and sanitized messages.
  Valid is configuration validity only, not proof of provider authentication.
- internal/web: embedded templates/CSS, hash-based constant-time credential
  comparison, safe JSON errors, public GET /healthz, authenticated GET / and
  /static/style.css. Unknown paths are 404; wrong implemented-route methods 405.
  No assets or templates need files beside the deployed binary.
- internal/storage/open.go: escaped file URI DSN, startup directory write check,
  DATA_DIR/energything.sqlite, one connection maximum, per-connection pragmas,
  close lifecycle. Synthetic probe tables exist only in temporary test databases.
- Dockerfile/Compose/Makefile/scripts/build.go: prebuilt bin/linux/{amd64,arm64},
  TARGETPLATFORM copy, no frontend prerequisites, UID 10001/GID 100, /data,
  only 8080, configurable image/host port/mount, restart unless-stopped.
  Compose requires both credentials and passes the published configuration names.
- .gitignore/.dockerignore/.env.example: runtime/private exclusions before builds,
  placeholder configuration, Docker context allowlisting only deployable binaries.
- README.md/docs/operations.md: build/run instructions, mount permissions and
  baseline lifecycle. scripts/verify-container.ps1: repeatable synthetic acceptance.

Environment names/defaults match the execution contract and .env.example.
ENERGYTHING_HTTP_ADDR accepts an empty host, IP or localhost with numeric port
1–65535. Username cannot contain a colon because Basic Auth uses it as separator.
Timezone data is also embedded in the Go binary for operation outside the image.
Provider dates are parsed in the configured location, reject future dates,
require paired ordered range bounds and a range request ID. Configuration is
parsed only; no range is persisted/enqueued at this stage.

## Checks actually executed

| Check | Observed result |
| --- | --- |
| go mod tidy | Passed after allowing dependency/cache access |
| go test ./... | Passed on native Windows; final suite includes cancellation during request, shutdown deadline, bind failure, config isolation, auth/routes, empty page and template escaping |
| go vet ./... | Passed |
| go build ./cmd/server | Passed without external assets |
| make build; make build-go-local | Passed; both Linux targets, CGO_ENABLED=0 |
| make test | Passed |
| Linux cross-builds below | Both server and storage test executables built |
| Linux storage probes below | All three storage tests passed on amd64 and emulated arm64 |
| make docker-build | Passed and built binaries before the amd64 image |
| docker build -t energything:local . | Passed with prebuilt binaries and pinned base |
| docker build --platform linux/arm64 -t energything:task01-arm64 . | Passed |
| Container acceptance below | Passed on both architectures |
| docker compose config --quiet | Passed with synthetic credentials; missing password was correctly rejected |
| git diff --check | Passed |
| git check-ignore | Runtime/private/build/cache paths ignored; .env.example and cmd/server/main.go remain includable |

Cross-build commands were executed in PowerShell with these environment values
(changing GOARCH between targets):

```powershell
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64' # then arm64
go build -trimpath -o bin/linux/amd64/energything ./cmd/server
go test -c -o bin/linux/amd64/storage.test ./internal/storage
# Repeat with arm64 paths and GOARCH=arm64.
```

Executed runtime commands:

```powershell
docker run --rm --platform linux/amd64 --entrypoint /probe --mount "type=bind,source=D:\projects\energything\bin\linux\amd64\storage.test,target=/probe,readonly" energything:local '-test.v'
docker run --rm --platform linux/arm64 --entrypoint /probe --mount "type=bind,source=D:\projects\energything\bin\linux\arm64\storage.test,target=/probe,readonly" energything:task01-arm64 '-test.v'
./scripts/verify-container.ps1
./scripts/verify-container.ps1 -Image energything:task01-arm64 -Platform linux/arm64
```

Container acceptance actually exercised missing username/password, invalid
timezone/address, a read-only bind mount, a writable bind mount, UID 10001,
fixed public liveness, missing/wrong page and asset auth with challenges,
authenticated partial/empty source cards, embedded CSS without source files,
unknown API 404, installed CA bundle/timezone files, second-process occupied-port
nonzero exit, and SIGTERM exit 0. Temporary containers were removed; synthetic
database directories remain only under ignored .cache/container-verification.

Initial attempts encountered restricted Go cache/network/loopback access and
were rerun successfully with elevated tool access. An initial test directory
contained a Windows-invalid question mark; corrected to an escaped URI test
using spaces, ampersand and hash. PowerShell initially split an unquoted
-test.v argument; quoted reruns passed. No failed attempt is counted as a pass.

## Unverified and outside this task

- Actual ARM64 hardware, OMV filesystem ownership and deployment behavior
  remain unverified; Linux ARM64 evidence here is emulated.
- Human visual review has not been performed. Automated checks exercised the
  actual built page/CSS in both images.
- CA bundle installation was verified; an external TLS handshake was not run.
- No live credentials/provider API access, release publication, CI, restore
  acceptance or task 02 schema work was performed.

No fallback storage or contract-semantic substitution was made.

## Approved Alpine follow-up

The user requested Alpine after the initial Debian verification. Dockerfile,
README, operations, task 01 and template-adaptation documentation now reflect
the approved base-image change. Logthing's historical Debian evidence remains
accurate. UID 10001/GID 100, /data, port 8080, CA roots and UK timezone support
are retained.

Executed both Docker build commands and both runtime probe/acceptance commands
above again using Alpine. All passed, including non-root SQLite write/reopen,
connection pragmas, auth, embedded assets, configuration failures, writable and
read-only bind mounts, occupied-port nonzero exit and SIGTERM exit 0.
No Go source changed, so the previously passing Go suite was not rerun for
this packaging-only change. No task 02 work was added.

Docker image inspect .Size measurements (bytes, as reported by Docker Desktop):

| Architecture | Previous Debian | Alpine | Reduction |
| --- | ---: | ---: | ---: |
| amd64 | 145,141,820 | 34,795,114 | 76.0% |
| arm64 | 166,375,615 | 34,523,196 | 79.2% |

These are Docker's local image-size measurements, not registry download sizes.
The unverified checks above still apply.
