# Task07a verification

Implementation date: 18 October 2026. Branch `feat/task07a-online-backups`
started from fetched origin/main `ff846aa`; PR #11 was open and its planning
commit `db4bc2e` was incorporated as `78a2fac`. No applicable AGENTS.md found in
repository or ancestors. Spec: [07a plan](07a-online-backups.md).

Live SQLite remains only in the ephemeral container writable layer. Persistent
`latest.tar` contains the complete standalone SQLite snapshot and manifest;
publication uses private staging, file sync, atomic replacement and Linux directory
sync. Restore validates archive membership/format/length/hash/integrity/foreign
keys/schema without migration, then server storage performs normal migration and
recovery once. Startup publication precedes serving/collection. Shutdown joins
writers and the periodic backup before a bounded final export, closing SQLite last.

The pinned modernc.org/sqlite v1.60.1 implementation was inspected directly:
`NewBackup` opens destination, `Step(n)` returns true with more pages, false on
DONE, and `Finish` releases backup and closes destination. A dedicated read-only
source connection pins a stable read transaction, leaving the normal serialized
write pool available. No dependency or snapshot-mechanism deviation.

Defaults: 15m wait after completed publication, 5m operation deadline, 6m Compose
grace. Every restored startup holds aggregate transmissions five minutes, or 24h
when the optional local daily cap is enabled, because lost calls could have
occurred immediately before restart even with an old dump. Captured provider
backoffs remain lower bounds. History start, canonical keys/corrections, spacing,
cap default false and pagination contracts remain unchanged.

## Executed evidence

- Full Windows Go tests passed; vet passed after rerunning a probe import check
  that overlapped an edit. Subsequent full regression suite and vet passed.
- Slow incremental snapshot synthetic test passed: normal commits and stored reads
  complete while copying; observed pinned WAL 4,556,752 bytes for the fixture;
  checkpoint truncates WAL to zero after backup, proving the read pin is released.
- Linux amd64 and emulated arm64 images built under isolated task07a tags. Both
  architecture backup/storage test executables passed, including held-open archive
  replacement, complete table restoration, fault injection at sync/rename
  boundaries, stale runtime rejection, missing established/corrupt dumps and
  cancellation. The Windows held-open-inode test is skipped because Windows open
  sharing forbids replacement; production copy guarantees are Linux-specific.
- Initial full lifecycle runs reached the final denied-directory diagnostic
  assertion after timer, external copy, replacement, isolated restore, crash and
  failed final-export checks. That assertion expected the previous error wording;
  it was corrected to the new sanitized startup diagnostic. Final reruns pending.
- Python release-policy tests passed. No release/tag/image publication attempted.

## Boundaries and retained preview

Preview observed running as `energything-preview`, image `energything:local`.
No second process opened its database. It was not stopped, recreated or migrated;
original mount, private Compose override, credentials, data and backups remain.
No provider probes, intentional rate limits, OMV deployment, alerts or registry
push occurred. Actual HTTP429 shape remains unestablished from retained evidence;
synthetic retries are not live-provider verification.

Operations includes an explicit offline `cmd/dump` conversion of a complete
cleanly stopped directory copy into a new dump directory, followed by isolated
restore and comparison of all tables/declared identities. Conversion must precede
a deliberate authorized preview cutover; no cutover was performed. Startup refuses
legacy DB/WAL/SHM in the dump mount. Unsnapshotted state is explicitly lost on crash;
application dumps are neither off-device backup nor historical retention.

Actual OMV backup-tool single-open copying, host filesystem/permissions, realistic
archive restore duration/capacity and private deployed-provider acceptance remain
task08 gates. Empty wrong mounts cannot be distinguished from a first install.

## Final checks and review

- `go test -timeout 3m ./...` and `go vet ./...` passed on Windows. Added/changed
  backup, conversion, server and storage checks were rerun after review.
- Full `CGO_ENABLED=1 go test -race -timeout 5m ./...` passed in
  `golang:1.26.4-bookworm` from a committed-source-only archive. After review,
  race checks of backup/storage/dump/server passed again. Native Windows race is
  unavailable without a cgo compiler; production binaries remain cgo-disabled.
- Added actual authenticated HTTP status browsing plus a committed reservation
  during a deliberately large online snapshot; both finish before backup does.
  Tests also cover cancellation during copying, released WAL pins, a real SQLite
  exclusive source lock with bounded wait, schema7 snapshot then normal schema8
  upgrade, and future-schema refusal. Physical online copy preserves all rows;
  conversion compares all tables, declared identities, schema and row counts.
- Archive tests reject wrong checksum, length, format/schema, duplicate/missing
  entries, traversal/unexpected paths and symlinks. A refused pre-existing restore
  destination remains untouched. Crash tests explicitly lose unsnapshotted later
  aggregates while restoring the exact previously published complete state.
- ENOSPC is injected at pre-sync/pre-rename/post-rename boundaries; no real host
  filesystem was filled. Before rename latest stays byte-for-byte old; after rename
  it is a complete new generation. Actual unwritable-mount tests show visible
  authenticated failure, preserved last dump and shutdown exit1.
- Go1.26 testing temporary directories needed explicit0700 after Linux enforcement
  was added. Initial platform tests exposed these permissive synthetic fixtures;
  fixtures were fixed and both architecture backup suites then passed.
- Both final platform lifecycle probes passed: CA/timezone, UID10001, authenticated
  API and secret redaction, timer publication, no mounted live database, container
  removal/recreation, slow held-open external copy during repeated publication,
  isolated external-only restoration, SIGKILL recovery, denied permissions and
  graceful/failing shutdown. Tests use disposable synthetic volumes only.
- Python release-policy matrix and actionlint passed. Gitleaks scanned the reviewed
  tracked-source archive (about1.11MB) with no leaks. Both image/context inspections
  passed exact allowlisting and private-file sentinel/layer exclusion. Application
  image tags used only locally: energything:task07a-amd64 and task07a-arm64.
- Fetched origin/main again: zero commits missing from the implementation branch.

### Standards review

No actionable findings. SQL ownership, module boundaries, diagnostics and cleanup
follow repository conventions; no baseline smell required changes.

### Spec review

Two findings fixed: enforce existing Linux storage directory ownership/private
modes; validate ancestor links/overlap for offline restore as well as conversion.
A destination-parent regression was also fixed: ordinary0755 parents are allowed
while the actual new destination is0700. Added regression tests cover permissions,
links, separate runtime mounts and initial-crash staging cleanup. Final reviewer
recheck reported no remaining actionable findings.

Standards:0 findings. Spec:2 findings fixed,0 remaining. No material plan deviation.
Preview cutover remains deliberately unperformed; conversion/isolated-restore
procedure and tool are ready for separate authorized transition.

## Fixed-filename simplification follow-up

The user selected fixed runtime and backup working paths. The live database is
now `/var/lib/energything-runtime/energything.sqlite`; backup work uses
`/data/snapshot.tmp` and `/data/latest.tar.tmp`, then replaces `/data/latest.tar`.
Startup restores through `energything.sqlite.tmp`. The `initialized` marker also
uses a fixed `.tmp` name. Random runtime/staging directories and their discovery
and recursive cleanup have been removed. Cleanup touches only known working files.

The local runtime default is `./energything-runtime`, keeping legacy
`./energything-data` out of the disposable location. Published dump format, online
copying, validation, atomic replacement, shutdown ordering and quota holds remain.
Other audit recommendations are deferred; no replacement system was added.

Targeted backup/server/configuration/conversion checks and the full Go test suite
passed. The final checks below also passed.
Standing instructions are installed in repository AGENTS.md and the user's global
Codex AGENTS.md. The immediate next task is a read-only simplicity review of the
earlier tasks01-07, including05a/06a, before task08; changes need concrete findings
and discussion first. No preview cutover or deployment was performed.

Fixed-filename follow-up results: full Go tests and vet passed; Linux race checks
for backup/server passed. Both amd64 and emulated arm64 backup suites passed,
including fixed saved filenames, no temporary files left after completion, stale
working-file cleanup and restart into the same runtime path. Both full container
probes passed with fixed-path/no-generated-subdirectory checks, live external copy,
restoration, crash recovery and failed shutdown export. Build ID:
`task07a-fixed-paths`. Standards review: no findings. Spec review: no findings.
