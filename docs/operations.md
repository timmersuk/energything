# Baseline operations

Use a local filesystem directory for /data. WAL is unsuitable for SMB/NFS.
On Linux create the bind mount before Compose starts:

```sh
sudo install -d -m 0700 -o 10001 -g 100 ./energything-data
```

For an existing directory, grant UID 10001 access to the directory and existing
database files. Mount ownership overrides image ownership. A read-only or
unwritable directory fails startup with an actionable message before listening.
Windows Docker Desktop maps host permissions differently; validate on the actual
Linux deployment host in task 08.

The application opens SQLite before binding HTTP. Each connection applies
journal_mode=WAL, foreign_keys=1 and busy_timeout=5000. The pool currently has
one connection, establishing a serialized storage seam without application tables.
SIGTERM/interrupt cancels the server context, drains requests within
ENERGYTHING_SHUTDOWN_TIMEOUT, closes HTTP if the timeout expires and closes SQLite
last. A listen failure returns a nonzero process exit.

Keep credentials in private process/Compose configuration. No credentials enter
SQLite, logs or status cards. No provider calls or background jobs exist in task 01.
Do not put runtime files in the image or commit .env, database companions,
exports, private evidence or caches.

Stop the service cleanly before backing up the entire data directory, including
SQLite companions. Do not copy only the main database during live writes.
Restore validation, release CI and actual host acceptance belong to later tasks.

Linux SQLite runtime probes (after cross-compiling storage test executables):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o bin/linux/amd64/storage.test ./internal/storage
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o bin/linux/arm64/storage.test ./internal/storage
docker run --rm --platform linux/amd64 --entrypoint /probe -v "$PWD/bin/linux/amd64/storage.test:/probe:ro" energything:local -test.v
docker run --rm --platform linux/arm64 --entrypoint /probe -v "$PWD/bin/linux/arm64/storage.test:/probe:ro" energything:task01-arm64 -test.v
```

The synthetic probe creates tables only in a temporary test database, verifies
pragmas on replacement connections, writes, closes and reopens the same value.
Cross-builds alone do not satisfy the runtime gate.

## Standard local preview update

After implementing and verifying a task, update and restart the local preview as
part of the normal handoff so localhost:8080 serves the current task build.
Record the actual build and restart result in that task's verification document.
Keep synthetic/browser fixture databases separate from the preview database.

Before changing it, inspect only safe container metadata: image, state, Compose
project/files and mounts. This workspace's preview is `energything-preview`,
Compose project `energything`, with its original bind mount
`D:\projects\energything\.cache\preview-data` at `/data`. Always use both
`docker-compose.yml` and ignored `.cache/preview-compose.override.json`.
Never use plain Compose against a different/default data mount, print resolved
Compose configuration, or display runtime credentials.

Build the current task with a public task build ID, then rebuild `energything:local`.
Validate the exact Compose setup with `config --quiet`. For schema changes, stop
cleanly and back up the entire original data directory, including companions,
to a new ignored private backup directory before recreating the container.
Preserve the container name, project, port and original mount:

```powershell
go run ./scripts/build.go -build-id <task-build-id>
docker build -t energything:local .
$env:ENERGYTHING_IMAGE='energything:local'
$env:ENERGYTHING_OCTOPUS_ONCE='false'
docker compose -p energything -f docker-compose.yml -f .cache/preview-compose.override.json config --quiet
# Stop and make a clean private data-directory backup first for a schema upgrade.
docker compose -p energything -f docker-compose.yml -f .cache/preview-compose.override.json up -d --no-deps --force-recreate energything
```

A plain `docker restart` does not adopt a rebuilt image. After recreation, confirm
that the container is running on the original mount, public `/healthz` responds,
and authenticated `/api/v1/status` reports the expected task build ID. Verify any
changed UI against that build. Keep one-shot live collection disabled during an
ordinary preview update; enable it only for an explicitly intended collection run.
If a restart cannot be performed, report the reason and stale preview build.

## Explicit Growatt verification

Build with `go build -o bin/growatt-verify ./cmd/growatt-verify`. Credentials come
from process environment, never automatic .env loading. `-report` is an explicit
private JSON output path, containing full candidate identifiers. Discovery ambiguity
is resolved with GROWATT_PLANT_ID/GROWATT_DEVICE_SERIAL and another bounded invocation.
Do not publish the report or use the CLI as a scheduled job.

```sh
./bin/growatt-verify -report .cache/private-growatt/discovery.json
./bin/growatt-verify -report .cache/private-growatt/discovery.json -collect \
  -data-dir .cache/private-growatt/database -day 2026-10-13 -timezone Europe/London
```

`-collect` always verifies latest; `-day` adds one history unit. No historical start
environment variable is required. Use a separate private database: this verification
CLI calls storage.Open, including startup recovery, and must never open a database
being used by a live server. It is not the future cross-process enqueue CLI.
The process is cancellable and has a 15-minute total bound, in addition to client
request/page/row/body bounds. The CLI prints only generic outcome messages; full
identities and selected telemetry remain in the opted-in report/database.

Task 05 adds migration 003. Back up the complete original preview directory after
a clean stop before recreating it. `scripts/task05-preview.ps1` applies the exact
saved Compose pair and verifies build/health/mount/retained counts without printing
credentials or resolved configuration. Synthetic browser data uses a separate mount.

Task 05 also adds migration 004 for discovery failure diagnostics. Apply the same
clean-stop/full-directory backup procedure before upgrading from intermediate schema003.
Run private preview browser checks with `scripts/task05-preview-browser.ps1`; it
keeps generated credential-bearing code/output ignored and removes them after checking.
