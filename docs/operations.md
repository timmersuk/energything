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
one connection per process, with immediate transactions serializing cross-process queue writes.
SIGTERM/interrupt cancels the server context, drains requests within
ENERGYTHING_SHUTDOWN_TIMEOUT, closes HTTP if the timeout expires and closes SQLite
last. A listen failure returns a nonzero process exit.

Keep credentials in private process/Compose configuration. No credentials enter
SQLite, logs or status cards. Automatic collection starts after asynchronous discovery.
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
docker compose -p energything -f docker-compose.yml -f .cache/preview-compose.override.json config --quiet
# Stop and make a clean private data-directory backup first for a schema upgrade.
docker compose -p energything -f docker-compose.yml -f .cache/preview-compose.override.json up -d --no-deps --force-recreate energything
```

A plain `docker restart` does not adopt a rebuilt image. After recreation, confirm
that the container is running on the original mount, public `/healthz` responds,
and authenticated `/api/v1/status` reports the expected task build ID. Verify any
changed UI against that build. Starting task06 intentionally enables automatic
collection for existing selections, including bounded historical exploration.
The aggregate ledger survives restart and continues to govern quota.
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
being used by a live server. Use cmd/enqueue for safe cross-process repair; it performs no startup recovery.
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

## Explicit bounded daily aggregate collection (task05a)

`go run ./cmd/growatt-aggregate -data-dir <stopped-or-private-directory>
-discovery <existing-private-growatt-discovery.json> -from 2026-01-12` collects
up to seven inclusive dates for the confirmed plant in that report. Requires runtime
GROWATT_API_TOKEN; never loads .env. No device totals, discovery calls, automatic
scan, timer, refresh endpoint or bootstrap. Maximum CLI duration2m plus bounded
5s failure-audit cleanup. Do not open any live server database with this CLI.

Back up the entire cleanly stopped original data directory before migration005.
Use the saved Compose project/pair/mount procedure above and the approved pinned
Alpine image. Task06 starts automatic collection when the server starts. The explicit CLI may populate one
older day while stopped, then recreate the preview from the exact same setup.
Private/browser databases and evidence stay under ignored .cache; never copy
synthetic fixtures into preview.

This endpoint documents five-minute spacing **and 10 calls/day**. A durable local
ledger counts every reserved transmission, including failures, and enforces ten
per rolling 24h. It does not see calls made by other databases/clients; coordinate
manual probes accordingly. Failures require a later explicit invocation; no
inline aggregate retries. A page deferred by the ledger rejects the complete
unit, preserving accepted data and completion. Repeated calls without waiting
produce durable rate-limit diagnostics, no cloud transmissions.

### Seven-date acquisition amendment (15 October 2026)

`-from YYYY-MM-DD` retrieves seven inclusive calendar dates by default, capped at
today's UTC date to avoid requesting future dates; `-to YYYY-MM-DD` explicitly
selects a shorter inclusive window. `-day` remains a one-day diagnostic option and
cannot be combined with range arguments. No scan/chunk loop or scheduler is added.
Example: `go run ./cmd/growatt-aggregate -data-dir <stopped-or-private-directory>
-discovery <private-report.json> -from 2026-01-12 -to 2026-01-18`.

Each returned date has one canonical record; a correction overwrites its value. One
quota reservation covers the single HTTP page, rather than seven separate daily
calls. All rows/pages validate and commit atomically; missing dates remain absent.
Existing one-day jobs and stored rows remain valid; no schema upgrade is required.

## Task06 scheduling handoff

See [collection contracts](collection.md) for the current automatic routine/bootstrap, repair CLI,
retry/quota and qualified time policy. ONCE/BACKFILL_START/startup RANGE_REFRESH
controls are obsolete and ignored. Migration006 retains independent schedules;
OpenQueue deliberately performs no startup recovery. The authenticated status
includes plans and explicit search-limit/boundary-unresolved state. A short search
is not an archive. Seasonal clock and stable repeated-hour identity remain gated;
old evidence and accepted observations are retained. Task06a must consume these
stored states without inferring history completion or adding another scheduler.
Older one-shot procedures above describe prior task verification, not current startup.

For a container server, run the enqueue binary inside the container or another
Linux process sharing its filesystem/locking semantics. Do not open its live SQLite
WAL from a Windows host executable through Docker Desktop's bind mount. The CLI
cross-process guarantee is for processes using the same OS SQLite lock domain.
Use scripts/task06-preview.ps1 for clean backup/recreation and
scripts/task06-preview-browser.ps1 for private browser checks. Earlier task05
preview helpers preserve historical verification procedures and are superseded.

The task06 image ships `/usr/local/bin/energything-enqueue` for same-OS repair:
`docker exec energything-preview energything-enqueue -data-dir /data -identity <id> -from <date> -to <date> -operation-id <id>`.
This action only inserts immutable durable work; the existing server performs it.

## Whole-history correction (15 October2026)

Migration007 removes the fixed 120-day stop. Persisted backfill plans with
`search_policy=calendar` continue into the preceding year whenever their current
annual target is fetched. The status field `search_limit` is then a current calendar
target, not an archive boundary; the page labels that distinction. No empty/zero
cutoff or initialization/backfill setting is required. Old stopped plans resume
from the same frontier and current unit, preserving accepted data, versions,
checkpoints, retries and quota reservations. Unit sizes, routine priority, repair
binding, clock qualifications and the aggregate ten-per-rolling24h limit remain.
Earlier passages describing a120-day exploration limit are superseded.


### Configured historical boundary (15 October 2026)

Superseding unlimited calendar acquisition: `ENERGYTHING_HISTORY_START` is the inclusive earliest date for automatic Octopus and Growatt history, default `2022-01-01` (ISO `YYYY-MM-DD`). Invalid and future dates reject startup. Octopus uses midnight UTC; Growatt uses provider calendar dates with qualified timezone. A verified later provider oldest reading remains the effective boundary.

Restart after changing the setting. Existing plans adopt it without resetting progress or deleting records. Moving it earlier resumes older acquisition; moving it later stops older queued automatic units. An in-flight unit may finish. `configured_boundary_reached` does not claim the provider archive is exhausted. Recent routine refresh and explicit repair remain independent.

## Private audit follow-up

Use authenticated `/audit` to filter repeated failures, compare accepted measurement
coverage and inspect stored retry/frontier state before taking operational action.
The daily generation page explains unfinished backwards history and quota deferrals.
Download only the selected attempt evidence needed for follow-up; it remains private
telemetry with owner-visible source IDs. Browsing performs no provider request or
startup recovery. Do not open the live preview database with a host CLI.
Task06a introduces no schema migration and retains existing records/evidence/private
runtime files and the aggregate reservation ledger during the standard image update.

`GROWATT_AGGREGATE_DAILY_CAP_ENABLED` is a boolean, default `false`. It controls
only the optional local ten-per-rolling-24h aggregate cap. Set it to `true` to
restore that cap and restart. Five-minute spacing and the existing reservation
ledger always remain active. Compose passes the flag through; no private runtime
file edit is required to use the disabled default.


## Schema008 canonical readings

Stop the server and take a complete private copy of DATA_DIR, including SQLite
WAL/SHM companions, before upgrading. Startup migration008 replaces attempt-based
samples with a composite primary key from source identity/date/reported time,
keeping the latest accepted retrieval for repeated periods. It removes sample,
interval and aggregate value-version tables. Existing audit/evidence, credentials,
source selection, schedules, accepted checkpoints and quota reservations remain.
Legacy rows lacking a usable reported timestamp are not invented canonical
readings; their retrieval evidence remains available. Downgrade requires restoring
the complete stopped backup and the older binary together. GET handlers never
migrate or recover storage. Restarting the old binary against schema008 is refused.
