# 01 — Runnable scaffold and SQLite build gate

Status: implemented with automated verification on 14 October 2026.
The user approved execution of task 01. Depends on: none. Unlocks: 02,
but task 02 has not started. See [execution handoff](01-verification.md)
for platform evidence and unverified human/host checks.
Read: [execution contract](execution-contract.md), [milestone](../../plans/milestone-1-service-and-collection.md), [template evidence](template-evidence.md).

## Objective and boundary

Produce an authenticated empty application and a reproducible non-root Docker
build. Establish SQLite/toolchain viability before other tasks depend on it.
Do not implement provider requests, scheduling or the final database schema.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| go.mod, go.sum | Energything module, pinned Go compatibility and SQLite dependency |
| cmd/server/main.go | Process entry, configuration, wiring, error exit and cancellation |
| internal/config/config.go | Typed environment parsing, global vs provider validation |
| internal/web/router.go, auth.go | Route dispatch, Basic Auth, liveness and safe errors |
| internal/web/assets.go | go:embed and template/static filesystem setup |
| internal/web/templates/, static/ | Empty status layout and locally embedded assets |
| internal/storage/open.go | SQLite opening/configuration and close lifecycle; no observation schema yet |
| Dockerfile, docker-compose.yml, Makefile | Prebuilt platform binaries and single-container packaging |
| .gitignore, .dockerignore, .env.example | Private/runtime exclusions and placeholder configuration |
| README.md, docs/operations.md | Reproducible build/run and baseline permissions |

Reference logthing's cmd/server/main.go for signal lifecycle, internal/api/auth.go
for credential checks, Dockerfile/Compose/Makefile/CI for packaging. Read these;
do not copy its syslog, incidents, frontend build or default Compose passwords.

## Implementation checkpoints

1. Check repository remote/module naming; derive the module path from this repo,
   not logthing. Establish the Go version and driver compatibility with actual
   builds. Record exact versions and commands; a version in logthing is a starting
   reference rather than proof that SQLite supports it.
2. Implement typed configuration following the shared contract. Required global
   values fail startup without echoing secrets. Parse provider settings into
   independently valid/disabled/error states. Do not load sibling .env files.
3. Extract a cancellable run path. Open required resources before accepting
   requests; use http.Server with bounded header timeout. SIGTERM/interrupt stops
   work, drains HTTP within the configured timeout and closes storage last.
   No helper calls os.Exit; a listen/bind failure returns an error.
4. Embed templates/static assets. Implement public GET /healthz returning a small
   fixed liveness response and authenticated GET / showing unconfigured sources.
   Protect assets too. Every other unimplemented path is 404, not a misleading
   successful placeholder API. Include a build ID without private information.
5. Open SQLite at DATA_DIR/energything.sqlite with an actual write/reopen probe
   in integration tests. Verify WAL, foreign keys and busy timeout on every
   connection used. Keep the probe out of production data semantics.
6. Adapt logthing's prebuilt bin/linux/{amd64,arm64}/energything layout and
   TARGETPLATFORM copy. Provide make build, test, build-go-local and docker-build;
   remove frontend prerequisites. Local go build ./cmd/server must also work.
   Plain docker build requires prebuilt binaries: document that prerequisite
   and make docker-build perform it. Do not leave a broken standalone command.
7. Use an Alpine image (user-approved replacement for Debian slim on
   14 October 2026), UID 10001 non-root user, only port 8080, /data,
   CA certificates and UK timezone support. Compose requires credentials, has
   configurable host port/image/local directory and restart unless-stopped.
8. Add exclusions before producing artifacts. Exclude .env except .env.example,
   SQLite companions, runtime directory, exports, private evidence and build caches.
   Compose examples contain placeholders only; images never COPY a runtime directory.

## Executable verification matrix

| Case | Expected observation |
| --- | --- |
| Missing username/password | Startup error, no HTTP listener, no secret echo |
| Invalid timezone/address/data permissions | Clear global startup failure |
| No sources / one partially configured source | Authenticated empty/error card, UI and liveness work |
| Missing/wrong Basic Auth on page/asset | 401 with challenge; no records/source detail |
| Embedded binary launched outside source tree | Page and CSS load without disk templates |
| Port already bound | Nonzero clean exit; no orphan worker |
| Cancel run during request | Bounded shutdown, no forced exit from helper |
| SQLite write, close, reopen on amd64/arm64 Linux | Same test value; cgo disabled |
| Non-root container with writable/non-writable mount | Success / actionable failure respectively |

Run go test ./... and native build, cross-build both Linux targets, then run the
SQLite integration probe on both via native container or emulation. Cross-compiling
alone does not prove runtime compatibility. Record unavailable Docker/emulation as
an unmet gate; do not claim it passed. CI implementation belongs to task 07.

## Handoff and stop conditions

Hand off exact versions, environment names, route/auth behavior, build commands,
UID/path conventions and verified platforms. Storage may proceed only once its
build/runtime gate passes. If driver compatibility fails, report the failure and
revise that choice explicitly; do not substitute JSONL or a database container.
