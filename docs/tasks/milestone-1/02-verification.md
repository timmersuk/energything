# Task 02 execution handoff

Implemented and verified with synthetic fixtures on 14 October 2026. Task 01 and
the existing planning files are preserved. No task 03 work, provider requests,
timers, browsing routes or new web pages were added. No .env contents were read
or displayed. The user-approved pinned Alpine base is unchanged.

## Files changed

- `internal/storage/open.go`: returns a concrete Store with private SQL; migrates
  and reconciles interrupted attempts before successful startup; close lifecycle
  and task 01 per-connection pragmas retained.
- `internal/storage/migrations/001_initial.sql`: source identities, immutable
  bounded jobs/range requests, attempts, deduplicated evidence, retrieval links,
  current intervals/samples, retained versions and window completions, with foreign
  keys and identity/time, audit and due-work indexes.
- `internal/storage/schema.go`: transactional ordered version upgrade, contiguous
  version validation, newer-schema refusal and startup recovery.
- `internal/storage/records.go`: typed identities, microsecond instants, decimal
  strings, nullable Growatt keys/time/values/units, coverage, jobs, audit/results.
- `internal/storage/jobs.go`: atomic identity upserts, idempotent bounded work,
  immutable range-ID bindings, nonoverlapping attempt claims, due work, explicit
  job resume and interrupted-attempt recovery.
- `internal/storage/commit.go`: full-window validation, accepted transactions,
  rejected/failed/cancelled audit transactions, selected evidence validation,
  canonical SHA-256 deduplication and retrieval/version retention.
- `internal/storage/query.go`: bounded indexed record and audit pages, query-bound
  keyset cursors, unresolved quality view/omission counts, sanitized audit detail,
  filtered incident counts and aggregate source/job/coverage status.
- `internal/storage/open_test.go`: retained lifecycle/physical connection pragma
  probes; schema expectation advances from task 01's empty DB to task 02's schema.
- `internal/storage/storage_test.go`, `query_test.go`: real temporary SQLite
  integration tests and realistic synthetic query-plan fixtures.
- `docs/data-semantics.md`: examples authored before migrations; logical schema,
  exact typed method contract, example commit/query usage and remaining protocol gate.
- `README.md`: current implementation scope and handoff links.
- This handoff.

## Established contracts

All SQL is private to storage. Consumers use the signatures and request/result
types published in [data semantics](../../data-semantics.md); no raw database,
row iterator or generic SQL escape hatch is exported. The existing server wiring
continues to open storage before serving and close it last.

Absolute time is integer UTC Unix microseconds; original provider fields remain
separate. Requested bounds are years 0001–9999. Electricity is validated exact
decimal TEXT, keyed by source/start, aligned to 30-minute slots and bounded to
14 days. Missing data never deletes earlier observations. Growatt values and units
can both be null. Keys and normalized time may independently remain unresolved.
Unknown telemetry fields stay in selected evidence; credentials, headers, account
and address fields are rejected by the additional storage guard.

Successful acceptance atomically writes current observations, retained versions,
evidence/retrieval links, fetched and measurement coverage, completion checkpoint,
terminal success/empty audit and next due. Evidence digesting occurs before the
transaction. Replay retains one current resolved reading and deduplicates content
bytes, while preserving separate attempts and retrievals. Corrections retain prior
values and evidence. Empty accepted responses also require complete evidence.

Failure/rejection transactions retain suspect or incomplete evidence, structured
sanitized errors, retry/page details and retry/pause state. They never alter accepted
observations, coverage or checkpoints. Error messages are derived from enumerated
classes/codes; arbitrary provider error bodies/URLs are not stored. Successful
qualified samples can carry a timestamp-quality notice without becoming failures.

Jobs have history/latest kinds and routine/backfill/range policies. Each historical
window is immutable bounded work. Completed backfill/range units do not become due
again; future scheduling ensures the next unit separately. Range request IDs bind
their entire ordered work list and cannot change bounds. New IDs deliberately create
repeat work. A running job cannot be claimed twice. Startup marks prior running
attempts interrupted and makes the same unfinished window eligible without advancing
its checkpoint. Explicit ResumeJob supports later resolved configuration/discovery
gates; this task implements no scheduler or retry timing algorithm.

Queries default to 100 rows, cap at 500, bind opaque cursors to validated filters
and page size, and use deterministic instant/key or descending occurrence/ID order.
Absolute Growatt queries report omitted null-time samples; quality views can use
provider date and retrieval ranges. Audit returns local/masked source references,
sanitized context and evidence references, without blob content or raw stable keys.
Status separates latest attempt, latest accepted attempt, retrieval and measurement
times and latest fetched-window coverage. Job/window detail lists cap at 500 per
identity with full retained counts. No retention or pruning is implemented.

## Checks actually run

| Check | Result |
| --- | --- |
| `go test ./internal/storage -v` | Passed against real temporary SQLite files |
| `go test ./...` | Final suite passed with loopback-capable execution |
| `go vet ./...` | Passed on final source |
| `gofmt -w internal/storage` | Applied |
| Linux amd64/arm64 `go test -c` and service `go build -trimpath`, `CGO_ENABLED=0` | Both passed |
| amd64 storage executable inside existing non-root Alpine application image | Entire suite passed |
| arm64 storage executable inside existing non-root Alpine application image, emulated | Entire suite passed |
| `EXPLAIN QUERY PLAN` with 20,000+ synthetic intervals, 20,000 samples and 20,000 attempts | Expected identity/time, quality/date and audit indexes used; checked queries required no temporary sort |
| `git diff --check` | Passed |
| Preview state-only inspection | Still running; unchanged start time `2026-10-14T11:46:44.406688528Z` |

Integration assertions cover two-interval reopen/checkpoint and exact decimal
strings, identical replay, 0.100 to 0.120 correction, rejected invalid/conflicting
windows, missing slots preserving history, deselection, empty expired Growatt
history, null versus zero and null-unit JSON serialization, repeated unresolved
autumn clock text without guessed UTC, resolved sample correction/replay,
interrupted recovery, job nonoverlap, stable interval/sample/audit pages,
provider/outcome/time filters and counts, masked output, incomplete retrieval links,
range-ID coalescing/conflict/repetition, migration rollback and newer-schema refusal.

Faults injected after evidence, after observations and immediately before commit
leave no partial accepted transaction or terminal success. Cancellation during an
actual transaction and during connection acquisition also leaves no accepted state.
Physical replacement connections retain WAL, foreign_keys=1 and busy_timeout=5000.
A second connection holding BEGIN IMMEDIATE exercises the actual five-second busy
timeout. PRAGMA query_only exercises actual SQLite write denial; max_page_count
with a large synthetic evidence blob produces actual SQLITE_FULL, leaving no false
terminal audit or partial evidence. This is a database capacity simulation, not a
claim that the host filesystem was physically filled.

Repeatable Linux commands (PowerShell; repeat builds for both architectures):

```powershell
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64' # then arm64
go test -c -o bin/linux/amd64/storage-task02.test ./internal/storage
go build -trimpath -o bin/linux/amd64/energything ./cmd/server
docker run --rm --platform linux/amd64 --entrypoint /probe --mount 'type=bind,source=D:\projects\energything\bin\linux\amd64\storage-task02.test,target=/probe,readonly' energything:local '-test.v'
docker run --rm --platform linux/arm64 --entrypoint /probe --mount 'type=bind,source=D:\projects\energything\bin\linux\arm64\storage-task02.test,target=/probe,readonly' energything:task01-arm64 '-test.v'
```

The initial full-suite attempt inside the sandbox failed task 01 loopback request
startup tests; the rerun with loopback access passed, without changing those tests.
An initial Git staging attempt lacked .git write access and was rerun with approved
tool access. Initial cross-builds emitted Go stat-cache permission warnings but
exited successfully; final cross-builds with cache access passed without warnings.
These unsuccessful/restricted attempts are not counted as verification passes.

## Remaining gates and review

Task 02's synthetic storage verification is complete. Actual ARM64 hardware and
OMV filesystem/deployment remain unverified. The Linux ARM64 result is emulated.
No real provider interpretation or real database/preview upgrade was exercised.
The preview was not restarted, migrated or reconfigured. No new image packaging,
web/browser/API acceptance, live collection or backup/restore acceptance was run.
Those remain dependent-task work rather than assumed task 02 results.

Task 05 must establish Growatt resolved sample keys, timestamp interpretation,
units/signs, impossible values and optional-field quality rules. Task 02 retains
unresolved occurrences by attempt/ordinal with nullable keys/instants and separate
evidence associations; it supplies no resolution or reconciliation algorithm.
All dependent tasks must use typed transactions rather than ad hoc SQL.

The previously uncommitted task 01 implementation/plans were snapshotted separately
as baseline commit `8d82461`. Task 02 is on `task02-storage`. Separate baseline and
storage PRs keep their reviews distinct; storage's PR is based on the baseline branch.
No task 03 implementation is included. No execution-contract semantics were replaced.
