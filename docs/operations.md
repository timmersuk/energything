# Release and operations

Task07a uses an ephemeral live database and persistent online dumps. The only
persistent database representation is `/data/latest.tar`; it contains a standalone
SQLite snapshot and manifest. The runtime directory is in the container writable
layer and must never be mounted. Existing preview storage is legacy and requires
the deliberate conversion below. Historical notes later in this file do not
supersede this storage contract.

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
`timmersuk/energything`. As in logthing, store DOCKERHUB_USERNAME and
DOCKERHUB_TOKEN as GitHub repository Actions secrets. Use Docker ID `timmersuk`
and a Docker Hub personal access token with Read & Write permission; Delete is
not needed. A separately named token can be revoked independently, but its name
does not restrict it to this image: a personal token can access other repositories
the account manages. No GitHub environment or environment approval is required.
Do not put provider credentials in GitHub. Workflow permissions are
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

For reproducible synthetic checks run the commands in `.github/workflows/ci.yml`.
On Windows after building an image, `pwsh -File scripts/verify-container.ps1
-Image energything:local -Platform linux/amd64 -BuildID local` builds the isolated
probe and runs the same lifecycle check. Repeat with an arm64 image/platform.
Context inspection additionally requires pinned Gitleaks: `python
scripts/verify-image-context.py --image <image> --scanner <gitleaks-executable>`.
The old task01 probe's unimplemented-API and second-server assumptions are removed.

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
listen. Keep `unless-stopped`, use a 6m grace period (increase beyond HTTP drain plus collector cleanup plus backup timeout), and inspect exit code rather than assuming a stop was graceful. The
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

## Online backup and isolated restore

The background worker publishes every 15 minutes after the previous publication
finishes, with a 5-minute deadline. A synchronous startup dump must succeed before
HTTP or collection starts. Graceful shutdown cancels and joins collection, HTTP
mutations and the periodic worker, then publishes with a fresh bounded context
before closing SQLite. A failed final dump returns exit1. The default Compose 6m
grace covers 10s HTTP drain, five-second collector cleanup and the 5m backup.

Back up **only `latest.tar`**, while Energything continues running. Exclude
`.energything-stage-*` and `.energything-marker-*`; `initialized` is application
publication state, not required to interpret a copied archive. Never copy the
runtime directory. Linux atomic replacement lets a reader finish copying its open
old generation. The external tool must open once per copy, not reopen the pathname
for individual chunks. Verify the actual OMV backup tool in task08. Do not assume
arbitrary live directory-copy tools pin a generation. Use a pinned-file export
protocol if the selected tool cannot do this; routine backup never stops Energything.

```sh
# Source remains running. Destination must be a new private retained file.
cp --no-clobber /srv/local/energything/data/latest.tar /separate-storage/backup-<unique>.tar
# New isolated mount, with only the copied archive; omit ALL provider credentials.
sudo install -d -m 0700 -o 10001 -g 100 /srv/local/energything/restore-<unique>
sudo cp /separate-storage/backup-<unique>.tar /srv/local/energything/restore-<unique>/latest.tar
sudo chown 10001:100 /srv/local/energything/restore-<unique>/latest.tar
sudo chmod 600 /srv/local/energything/restore-<unique>/latest.tar
docker run -d --name energything-restore-check --network none \
  -v /srv/local/energything/restore-<unique>:/data \
  -e ENERGYTHING_USERNAME=restore-check -e ENERGYTHING_PASSWORD='<private-password>' \
  namespace/energything@sha256:<matching-backup-digest>
```

Inspect authenticated records, audit/evidence, identities, jobs, completions,
schedules and reservations using a private client inside that isolated container.
Remove/recreate the container with the same dump mount and compare again. Preserve
the external original. Never run restored collectors with real credentials beside
the active service. Inspect exit status after `docker stop -t 360`.

## Restore authority and failures

Every startup, including `docker restart`, restores the completed archive into a
new private runtime generation, validates SHA-256/length, archive members, SQLite
integrity, foreign keys and supported schema, then applies normal migrations and
interrupted-attempt recovery exactly once. It never trusts writable-layer leftovers
or arbitrarily selects staging files. Unsupported/corrupt dumps fail startup.
Missing latest on a nonempty/initialized mount fails rather than resetting history.
A new empty mount creates schema8 directly and publishes before collecting. An
entirely empty wrong mount is indistinguishable from installation: check the actual
mount source before starting. Restore an older retained archive deliberately into
a new mount; never delete initialization state to force an empty start.

Status/UI reports backup running/idle/error, last success, snapshot time/age,
duration, bytes and next due. `/healthz` remains process-only. Publication writes
private staging files, flushes the archive and replaces latest atomically, then
syncs the parent directory on Linux and records initialization. Failure before
replacement leaves the old dump intact; a later sync/state failure can leave the
new complete generation published but is reported as failure. Nothing overwrites
or unlinks latest first. No provider secrets enter archives.

Committed rows live only as long as the ephemeral database until captured in a
published dump. Crash/SIGKILL can lose readings, evidence, audit, checkpoints and
quota reservations since that snapshot. Healthy recovery gap includes interval,
scheduling and snapshot duration, not a hard 15-minute maximum. Repeated failures
make it unbounded. The external backup schedule adds a separate host-disk-loss gap.
After every restore, aggregate calls wait five minutes from startup, or 24 hours
if the optional daily cap is enabled. Lost calls might have occurred just before
restart even if the dump is old. Captured provider deadlines remain lower bounds.
No unsnapshotted calls/audit are invented. Other collection semantics are unchanged.

## Deliberate one-time legacy conversion

The retained preview must not be switched automatically. Its original mount and
private override stay unchanged until the owner authorizes cutover. Readiness for
conversion is separate from task08 deployment. New startup refuses any legacy
`energything.sqlite`, WAL or SHM in the persistent mount.

1. Record the exact old image/build/Compose invocation and stop the original
   cleanly. Verify stopped state and exit0. This maintenance stop is only for the
   storage-layout transition, never for routine backup.
2. Preserve a new complete private copy of the entire stopped directory including
   companions, plus the original mount/override and all previous backups. Work only
   on another copy. Never migrate/recover the live directory with a second process.
3. With Go1.26.4 and this checkout, run the offline conversion against that copy:

   `go run ./cmd/dump -mode convert -stopped-copy -source <copied-stopped-directory> -destination <new-private-dump-directory>`

   Destination must not exist. The tool validates without source migration/recovery,
   snapshots through the incremental API, publishes an archive and restores it in
   isolation, comparing all tables and declared identities to the stopped source.
   A partial failed destination must not be deployed; preserve it for diagnosis.
4. Optionally inspect offline with `go run ./cmd/dump -mode restore -source
   <new-private-dump-directory>/latest.tar -destination <new-private-inspection-directory>`.
   Never use the extracted SQLite directory as the persistent server mount.
5. Restore into an isolated new container with providers disabled and compare all
   retained state/keys/evidence. Verify mount ownership UID10001/GID100, directories
   0700 and archive 0600. Only then deliberately change the private Compose override
   to mount the new dump directory at `/data`, leaving runtime unmounted. Preserve
   old backups and override for rollback with the matching old image.

## Space and storage failures

Budget disk for the live database and WAL in the writable layer, plus a staging
SQLite snapshot, staging archive and previous completed archive on the dump mount.
A stable read transaction pins WAL during page-stepped backup; writers continue
but WAL can grow for up to the backup deadline. Monitor both filesystems and
measure real household restore time/capacity at task08. Synthetic timings are not
a realistic archive-size benchmark. No readings/evidence/audit are pruned for space.

Check free space with `df` for both container runtime and dump storage. On storage
failure, backup status becomes error and latest remains a completed generation.
Reads/collection can continue if live storage permits. Preserve the dump, correct
capacity/permissions and confirm a subsequent successful publication. Restarting
before a successful dump loses unsnapshotted changes. Never delete live WAL/SHM to
repair the running database. Local Linux filesystems only, not SMB/NFS.

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

Task07a exception: do not run this legacy update procedure for the retained preview.
Use the deliberate conversion above only after authorized cutover.

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
For task07a, run enqueue inside the container against its current private runtime generation: `-data-dir /var/lib/energything-runtime/.energything-runtime-<current>`. Resolve exactly one current generation after startup; never point enqueue at `/data`. Enqueued work becomes persistent only after a successful dump.
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
