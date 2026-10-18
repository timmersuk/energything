# 07a — Ephemeral SQLite and persistent online dumps

Status: implemented 18 October 2026; see [verification](07a-verification.md) for
executed checks and remaining private host gates. Depends on merged 07; required
before 08. This document does not authorize preview cutover or deployment.
Read: [execution contract](execution-contract.md), [operations](../../operations.md),
[configuration](../../configuration.md), [task07 evidence](07-verification.md).

## Required outcome and authority

The live SQLite database WILL NOT be persistent. This is an explicit user decision,
superseding the earlier persistent `/data/energything.sqlite` design and the
stopped-container backup requirement. Do not substitute a persistent live database.

The running service periodically publishes a complete dump to its mounted volume.
The host backup service copies that dump while Energything continues running; it
never needs to stop the Energything container. On startup the service restores the
latest valid dump into a new ephemeral live database. On graceful shutdown it
publishes a final dump after collection has stopped changing the database.

One overwritten public-to-the-backup-service pathname is sufficient; publication
must be atomic, never an in-place overwrite of the only good dump. The export is
private household data, not a public download or CI artifact. No provider secrets
enter its contents. No automatic private-host access or alerts are added.

## Storage layout and dump format

- Live database: a service-owned directory in the container's writable layer,
  proposed `/var/lib/energything-runtime/energything.sqlite`. No Docker VOLUME,
  bind mount or named volume covers it. Companions and temporary SQLite files stay
  there. A disk-backed container layer avoids requiring the entire archive in RAM.
- Persistent mount: `ENERGYTHING_DATA_DIR` (`/data` in Docker), containing published `latest.tar` and publication
  state. It contains no live SQLite database or live WAL/SHM files after cutover.
- `latest.tar` contains a standalone SQLite snapshot named `energything.sqlite`
  and `manifest.json`. The manifest records format version, schema version,
  producing build ID/image reference when supplied, snapshot start/completion UTC
  times, database byte length and SHA-256. No separate sidecar is required to
  interpret a copied archive. SQLite is the dump encoding, not a mounted live DB.
- The snapshot includes ALL application tables: canonical readings, identities,
  discovery state, attempts/audit, evidence/retrievals, completions, jobs, schedules,
  boundaries, repair requests and aggregate reservations. Never export just readings.
- The host backup service backs up `/data/latest.tar`, not the live database or
  the whole working directory. It retains historical versions on separate storage.
  Application overwrite of latest is not an off-device backup or history policy.
- Temporary snapshots/archives use fixed `snapshot.tmp` and `latest.tar.tmp` files
  beside the published archive, excluded from external backups. Plan capacity for staging plus
  the previous completed archive and the separate runtime database/WAL.

## Online snapshot mechanism

Use SQLite's incremental Online Backup API through the existing pinned
modernc.org/sqlite driver. The driver exposes `NewBackup`, `Step` and `Finish`;
verify those exact v1.60.1 semantics in implementation. No sqlite CLI, SQL text
dump, second server, dependency upgrade or shell-out is required.

Storage owns a narrow snapshot method; a single background worker owns timing
and publication. A dedicated connection to the ephemeral database must not call
`storage.Open` or perform migration/startup recovery. Keep the existing normal
write pool serialized. Bound backup page steps, waits, retries and overall time;
use the driver's raw connection safely and always finish/close resources.
Do not hold the application's sole pooled connection for an entire large backup.

Prove collection commits and authenticated reads can complete while a deliberately
slow snapshot is running. Handle busy/locked/restarted snapshots under continuous
writes without spinning or starving writes. If a stable read transaction is used,
measure WAL growth and cancellation. If the API restarts instead, measure bounded
completion under ongoing writes. Do not claim the backup captures shutdown-end
state unless writers were joined before that backup began.

References: [SQLite online backup](https://www.sqlite.org/backup.html),
[pinned driver API](https://pkg.go.dev/modernc.org/sqlite@v1.60.1#Backup).
VACUUM INTO is an alternative live-copy mechanism, but is not the selected default:
the incremental API offers bounded page steps and preserves physical row identity.
Any change of mechanism requires evidence and a documented plan amendment.

## Atomic publication and external-copy contract

1. Create the fixed private working files `snapshot.tmp` and `latest.tar.tmp`; never modify published latest. Only one backup runs at a time.
2. Produce a complete, closed standalone SQLite snapshot. Verify integrity,
   foreign keys and supported schema without migration or recovery; ensure no
   companion file is needed to read it. Compute its hash and manifest.
3. Write a complete archive containing snapshot and manifest. Flush/sync files,
   then atomically rename/replace the archive as `latest.tar` on the same local
   filesystem. Sync the parent directory on supported production Linux filesystems.
4. Record last-success state only after publication succeeds. The previous latest
   remains intact on snapshot, validation, disk-full, permission or rename failure.
   Never unlink latest first. Clean only application-owned incomplete staging files.

On Linux a reader holding the old file open may finish copying that immutable
generation while a new inode becomes latest. Test this explicitly, with a slow
concurrent copier and repeated publications. The external tool must copy a file
from one open generation, not concatenate chunks across pathname reopens. Document
and test the actual OMV backup tool in 08; generic arbitrary live directory-copy
behavior is not assumed safe. If the tool cannot provide this, use an explicit
generation-pinning/export protocol rather than stopping the application.

Reject symlinks/path traversal and source/destination overlap. Enforce UID10001
ownership/access, private directory/file modes and exclusive creation of staging
files. Temporary filenames are fixed and cleaned before retry; no random generation directories. Archive restore accepts only the two expected regular files, with bounded
manifest parsing and validated lengths/hashes; never extract arbitrary archive paths.

## Lifecycle and restore

### Startup

1. Validate paths, writable mount, runtime isolation and supported configuration.
2. Restore `latest.tar` through `energything.sqlite.tmp` into the fixed ephemeral runtime database path. Validate format,
   checksum, database integrity/foreign keys and schema compatibility before
   making it the runtime database. Never mutate the mounted dump during validation.
3. Open the restored runtime database using normal server migration/recovery, once,
   then start HTTP/collectors. Existing upgrade paths and the final-v8 fresh
   baseline remain unchanged. Retained running attempts become interrupted normally.
4. For a genuinely new installation with no dump or initialized-state marker,
   create the final fresh baseline and successfully publish its initial dump
   before starting collection. Maintain an atomic initialized marker so a missing
   dump on an established mount fails startup rather than silently resets history.
   An entirely empty wrong mount cannot be distinguished from a first install;
   document that deployment must verify the expected mount before starting.
5. Corrupt, unsupported or missing established dumps fail with an actionable,
   sanitized error. Never silently start empty, reuse container leftovers, reset
   quotas or choose an arbitrary staging file. Recover an older externally retained
   archive explicitly. Each startup restores authoritative published state, even
   after `docker restart`, rather than trusting writable-layer leftovers.

### Runtime and shutdown

- One goroutine publishes after startup and on a timer; no overlapping snapshots
  or unbounded queued ticks. Proposed default interval: 15 minutes. Expose interval,
  persistent dump directory and backup deadline in validated configuration/Compose.
  Document defaults as proposed until implementation establishes measured limits.
- Use a bounded backup deadline (proposed 5 minutes) and cancellable page steps.
  Errors retain latest and retry at a bounded interval; HTTP and collection remain
  available if storage capacity permits. An initial/restore failure is distinct
  from a later backup failure. Do not prune readings/evidence to make backup fit.
- On graceful shutdown: cancel/join collection and HTTP mutations, cancel/join any
  periodic backup, then take a final consistent snapshot with an independent bounded
  cleanup context while the database remains open. Close storage last. Set Compose
  stop grace to cover collector cleanup, HTTP drain and final backup timeout.
- If the final backup fails or times out, preserve the last good dump, emit a
  sanitized failure and return a nonzero shutdown result. Do not report latest
  changes as durable in the dump. SIGKILL/power loss cannot depend on final backup.

## Recovery guarantees and visibility

Every committed row is durable only for the lifetime of its ephemeral runtime
database until included in a successfully published dump. A crash/replacement can
lose changes since that snapshot. With healthy scheduled backups the recovery gap
includes the interval, scheduling delay and snapshot duration; persistent backup
failure makes it unbounded. State the actual last successful snapshot time, not a
promise of a hard 15-minute maximum. The external backup schedule adds another
recovery gap for host-disk loss. Measure startup/restore time with realistic data.

Expose backup state through authenticated status/UI: running/idle/error, last
success and snapshot time, age, duration/size, next due and sanitized last failure.
No filesystem paths, telemetry, secrets or downloaded dump in public `/healthz`.
The liveness endpoint remains process-only. A stale/failing backup must be visible
without adding Discord/alert integration.

Restoring an older dump also rewinds progress, audit and quota reservations to its
snapshot. Preserve everything captured; never invent post-snapshot history. Add a
conservative restart safety hold for aggregate calls (five-minute spacing and,
when enabled, the rolling daily cap) because calls made after the snapshot may be
absent from its ledger. Derive/document the hold conservatively; provider-directed
deadlines present in the dump remain lower bounds. Do not claim a periodic export
can preserve unsnapshotted audit or quota decisions after a crash.

## Existing preview/data cutover

The retained preview currently uses `.cache/preview-data` mounted at `/data` and
its private Compose override. Preserve it and every private backup unchanged while
implementing and testing. No automatic migration of the live preview by a second
process; no provider probes or intentional rate limiting.

Provide an explicit one-time conversion procedure from a legacy persistent DB to
the first validated dump. A maintenance stop for that storage-layout transition is
separate from routine backups and must be reported/authorized as deployment work.
Use a copy of the complete stopped legacy directory, verify all tables and natural
keys against the exported dump, and preserve the original. Do not automatically
delete/overwrite a legacy DB found in the persistent volume. New startup should
detect that legacy layout and require deliberate conversion rather than start empty.
Switch the mount contents/layout only after validated export and isolated restore;
keep the original preview mount/override until a deliberate cutover is authorized.

## Implementation slices and responsibilities

1. **Snapshot primitive:** `internal/storage` owns incremental copy, resource
   cleanup and read-only validation. Synthetic concurrent-write and WAL tests first.
2. **Published dump:** a small concrete backup module owns archive/manifest,
   fsync/atomic replacement, staging cleanup, format validation and extraction.
   No provider logic or second primary database architecture.
3. **Lifecycle:** `cmd/server` restores before opening the server DB; owns worker,
   joins and final snapshot. Config owns path/interval/deadline validation. Normal
   startup recovery and existing baselines/migrations remain storage-owned.
4. **Packaging/status:** Dockerfile/Compose use ephemeral runtime path and persistent
   dump mount; update configuration examples and authenticated backup status/UI.
   Preserve logthing's registry/CI conventions; extend existing platform probes.
5. **Cutover/docs:** add explicit conversion/isolated restore tooling and procedures,
   external backup selection/exclusions, recovery-gap/capacity guidance and task08
   checks. Update old stopped-backup instructions as superseded for routine backups.

## Verification and acceptance gates

| Check | Required evidence |
| --- | --- |
| Online snapshot under writes | Accepted windows remain atomic; reads/writes complete during slow page-stepped copy |
| Complete contents | All table rows and relevant row identities survive snapshot/archive/restore, including evidence/progress/quotas |
| Concurrent external copy | Every copied latest archive is one valid generation during repeated replacement |
| Publication failure | Inject faults before/after sync/rename; latest is old or new complete archive, never partial or missing |
| Resource failures | Busy/locked, ENOSPC, unwritable mount, cancellation and timeout preserve latest and visible failure state |
| Startup | Fresh baseline, valid restore, supported upgrade, newer schema refusal, corrupt/hash mismatch, missing established dump and legacy layout |
| Shutdown/crash | Final export includes last accepted commit; SIGKILL restores last published state and documents losses/holds |
| Ephemeral database | Remove container AND runtime writable layer; reconstruct solely from mounted dump with no live SQLite on mount |
| External-only recovery | Copy latest while service runs, recreate from that copy in a different volume with providers disabled, verify all records/audit/evidence/progress |
| Restart safety | No overlapping collectors, no false completion, captured provider deadlines retained and lost-reservation safety hold enforced |
| Linux matrix | Both cgo-disabled amd64/arm64 images execute restore/timer/publication/shutdown/external-copy tests |
| Regression/privacy | Full Go tests/vet/race where supported; no private files in Git/context/images/logs/public status |

Use deterministic timers/fault injection and synthetic data; do not wait real
15-minute intervals or touch retained preview data in tests. Record tools, commands,
actual successes/failures and unverified host behavior in `07a-verification.md`.
Task08 must test the actual external backup service copying published dumps while
Energything stays running, then restore from that backup with providers disabled.
Do not declare operational acceptance complete using only an internal snapshot test.
