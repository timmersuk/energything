# Release and operations

Current task07 procedures below supersede historical task handoffs later in this
file. See [all configuration](configuration.md) and [verification](tasks/milestone-1/07-verification.md).

## Release mechanism

CI uses synthetic tests, vet and race on Ubuntu, then QEMU/Buildx runtime checks
for linux/amd64 and linux/arm64. Production binaries use Go1.26.4, CGO_ENABLED=0,
embedded assets and the platform-specific `bin/linux/<arch>` layout. Race testing
uses cgo separately. Every action is pinned to a reviewed commit; update pins
deliberately. The Alpine base is digest-pinned; APK CA/tz packages are resolved at
build time, so rebuilding a version is not promised byte-for-byte reproducible.

Only `release.published` with draft=false, prerelease=false and a `vMAJOR.MINOR.PATCH`
tag can publish, after both architecture gates pass. PR, branch, tag-only and
workflow_dispatch builds never log in or push. No task07 verification creates a
tag/release. Release creation remains a separately authorized operator action.

Before enabling releases, configure repository variable DOCKERHUB_IMAGE as
`<dockerhub-namespace>/energything`. Create environment `dockerhub-release` with
required reviewers and release-tag deployment restrictions. Store DOCKERHUB_USERNAME
and DOCKERHUB_TOKEN there, with a repository-scoped Docker Hub write token for this
image only. Do not put provider credentials in GitHub. Workflow permissions are
contents:read; no package write or PR write token is required. Configure Docker Hub
immutable tags for `v*` (including build tags). Never delete/reassign a released
version to bypass immutability. A rerun of an existing version should fail rather
than overwrite it; issue a new version after review.

Images carry full version and `<version>-sha-<commit>-run-<run>-<attempt>` tags,
OCI metadata and the same embedded build ID. No moving latest/minor aliases are
published. Record the resulting manifest digest and build ID with each deployment.
Consumers pin `namespace/energything@sha256:<manifest-digest>` so tag reassignment
cannot change an installed release. Registry publishing itself is not locally
verified by a dry build.

Secret scanning uses Gitleaks v8.30.1 (security-maintained upstream), installed
through Go's checksum-verified module download. CI scans an archive of tracked
source/fixtures, offline with redacted findings; it does not scan private host
directories or verify credentials with providers. Upstream:
[Gitleaks](https://github.com/gitleaks/gitleaks). The Docker context is an explicit
allowlist of Dockerfile and the four release binaries. `verify-image-context.py`
exports the actual context, verifies that exact file set and inspects saved image
layers with synthetic private-file sentinels. Probe/test binaries never enter the
release image. Keep runtime files out of allowed binary paths.

## Reproducible OMV installation and pinned update

Host acceptance belongs to task08. These commands are generic Linux examples;
replace paths, image digest, credentials and backup names before executing.
Use local disk, never SMB/NFS. Run exactly one server per database. Never use a
second server or verification CLI to migrate/recover a live database.

```sh
sudo install -d -m 0700 -o 10001 -g 100 /srv/local/energything/data
sudo install -d -m 0700 /srv/local/energything/backups
# Place docker-compose.yml and a private mode-0600 .env in this project directory.
# .env: ENERGYTHING_IMAGE=namespace/energything@sha256:<manifest-digest>
# ENERGYTHING_DATA_PATH=/srv/local/energything/data
# ENERGYTHING_HTTP_PORT=8080 and your chosen required username/password.
docker compose -p energything config --quiet
docker compose -p energything pull
docker compose -p energything up -d --no-deps energything
curl --fail http://127.0.0.1:8080/healthz
curl --fail --user "$ENERGYTHING_USERNAME" http://127.0.0.1:8080/api/v1/status
```

The last command prompts for the password; do not paste passwords into shell
history. Compose substitution does not export variables into the shell. Set the
username separately for curl. Inspect build_id, provider configuration/discovery,
last accepted attempt, requested bounds/frontier, retry state and authenticated
audit. Verify records and selected evidence through the local authenticated API/UI.
Provider errors do not make `/healthz` fail; it is process liveness only. Collection
resumes from retained state only while the service runs. Do not assume overnight
progress. A fetched empty window is not zero generation or complete measurements.

Mount ownership overrides image ownership. UID10001/GID100 needs directory search
and write access plus access to every existing SQLite file and companion. On a
restored directory, apply ownership recursively only to that explicitly verified
application data path. Do not chmod777. An unwritable directory fails before HTTP
listen. Keep `unless-stopped`, use a 30s grace period (increase with the shutdown
timeout), and inspect exit code rather than assuming a stop was graceful. The
image includes CA roots, timezone data and the enqueue CLI and runs UID10001.
Restrict the published port using host firewall/LAN rules or a trusted HTTPS
reverse proxy. Basic Auth needs trusted transport; never expose port8080 publicly.

For updates, record the old digest/build ID and schema version (currently8), stop
and back up as below, then change only the private pinned image reference, pull
and recreate against the same mount. Confirm health, expected build ID, records,
audit/evidence and resumed checkpoints/frontiers. Compare accepted row counts and
audit to pre-update state; ordinary collection can add/update data. Never reset a
quota ledger or checkpoint to make an update appear successful. If startup fails,
leave the data intact and inspect sanitized logs. Do not run an older incompatible
binary on a migrated database; restore its matching backup/image together.

## Supported stopped backup and isolated restore

```sh
docker compose -p energything stop -t 30 energything
cid=$(docker compose -p energything ps -a -q energything)
test "$(docker inspect -f '{{.State.Running}}' "$cid")" = false
test "$(docker inspect -f '{{.State.ExitCode}}' "$cid")" = 0
# Choose a new backup path; fail rather than overwrite an existing backup.
backup=/srv/local/energything/backups/backup-<unique-date-build>
sudo mkdir -m 0700 "$backup"
sudo cp -a /srv/local/energything/data "$backup/data"
# Write the recorded old image digest, build ID and schema version into metadata.
docker inspect -f '{{.Config.Image}} {{.Image}}' "$cid" | sudo tee "$backup/image.txt" >/dev/null
printf '%s\n' 'schema=8' 'build_id=<recorded-build-id>' | sudo tee "$backup/schema-build.txt" >/dev/null
docker compose -p energything start energything

# Restore into a NEW isolated directory, never over the active data mount.
restore=/srv/local/energything/restore-<unique-id>
sudo mkdir -m 0700 "$restore"
sudo cp -a "$backup/data/." "$restore/"
sudo chown -R 10001:100 "$restore"
# Use the matching recorded image; omit ALL provider credentials, network isolated.
docker run -d --name energything-restore-check --network none \
  -v "$restore:/data" -e ENERGYTHING_USERNAME=restore-check \
  -e ENERGYTHING_PASSWORD='<temporary-private-password>' \
  namespace/energything@sha256:<matching-backup-digest>
docker exec energything-restore-check wget -qO- http://127.0.0.1:8080/healthz
# Inspect authenticated records, audit/evidence and progress from inside this
# isolated container using a private HTTP client/config; compare to saved state.
docker stop -t 30 energything-restore-check
docker inspect -f '{{.State.ExitCode}}' energything-restore-check
docker rm energything-restore-check
```

The copy includes the whole directory and any WAL/SHM companions after confirmed
exit. Never copy just a live energything.sqlite. Keep backups private, on separate
storage, with digest/schema/build metadata. Test restoration before relying on a
backup. Isolated restore must retain records, source identities, canonical keys,
audit/evidence, jobs/completion checkpoints, schedules and quota reservations. With
credentials omitted, collectors cannot contact providers and local browsing still
works. Preserve the original backup while reviewing a restore. A controlled host
rollout is a separate authorized step.

## Space and storage failures

Use `df -h /srv/local/energything/data` and `sudo du -sh
/srv/local/energything/data` to inspect free space and total size, including SQLite
companions. All selected evidence and audit are retained; no automatic deletion,
pruning or aggregation recovers disk space. Review growth and expand local storage
before pressure becomes an outage. On disk-full/permission/IO failure an accepted
transaction cannot advance partially; failure audit itself may also fail. Stored
reads may return503; process liveness alone is insufficient. Stop cleanly, preserve
the complete directory and backups, correct capacity/permissions, then inspect
audit/progress after restart. Never delete WAL/SHM files to repair a running DB.

## Historical implementation notes

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
Restore validation and release CI are covered by task07 above; actual host acceptance remains task08.

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

## Fresh database initialization and upgrades

The server owns schema creation and updates. It opens DATA_DIR/energything.sqlite
when starting, before collection. A new database executes the embedded final
version-8 baseline.sql directly, in one transaction. It does not run development
migrations 001–008 or create/drop their superseded tables. Their versions are
recorded as satisfied by the baseline for compatibility with existing databases.
Only migrations newer than that baseline execute afterwards.

An existing versioned database skips baseline creation and executes only its
missing migrations. An existing version-8 preview receives no schema changes.
Schema SQL and version bookkeeping commit together or roll back together; a
newer-than-supported schema is refused. The queue CLI checks compatibility but
does not migrate or recover. HTTP browsing never initializes or upgrades storage.

Before updating an existing deployment, stop the service and back up the entire
data directory as described above. Updates run automatically on the next server
start with the replacement image. Restoring an older incompatible binary requires
its matching stopped database backup. Post-release migrations remain necessary
for real schema changes; pre-release history is not replayed on a first install.
