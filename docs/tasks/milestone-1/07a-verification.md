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

## Executed evidence (in progress)

- Full Windows Go tests passed; vet passed after rerunning a probe import check
  that overlapped an edit. Final regression rerun is recorded below.
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
