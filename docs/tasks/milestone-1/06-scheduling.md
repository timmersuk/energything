# 06 — Durable scheduling, bounded backfill and recovery

Status: planning draft. Depends on: 04, 05. Unlocks: 06a, 07.
Read: [execution contract](execution-contract.md) and both provider handoffs.

## Objective and boundary

Replace provider one-shot wiring with durable independent jobs. Automatic refresh,
backfill and authenticated refresh-now must survive restart without archive-wide
refetch. Collection remains inside this process; no cron/worker container or
complex management UI. Attempt audit is emitted here; browsing it belongs to 06a.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/collection/scheduler.go | Due-work arbitration, concurrency/non-overlap, cancellation |
| internal/collection/windows.go | Half-open Octopus and provider-day Growatt plans |
| internal/collection/retry.go | Explicit retry budget, cancellable waits and next due |
| internal/collection/worker.go | Attempt lifecycle, fetch/validation/commit/failure orchestration |
| internal/storage/jobs.go | Persisted job mutations/restart reconciliation, no timers |
| internal/web/refresh.go | CSRF-protected enqueue handler and response |
| cmd/server/main.go | Wiring and shutdown ordering |
| docs/collection.md | Schedules, checkpoints, explicit range replay and source errors |

## Job kinds and defaults

| Kind | Initial requested work | Routine behavior |
| --- | --- | --- |
| Octopus import/export backfill | Previous 30 London calendar days plus current partial day, or configured older start | At most 14 absolute days per unit; resumable |
| Octopus routine refresh | Selected direction's recent 7 calendar days through elapsed current intervals | Every 1h |
| Growatt history backfill | Previous 7 provider-calendar days plus current partial day within availability, or configured start | One day per unit; resumable |
| Growatt latest | One latest response | Every 5m |
| Growatt history refresh | Current day and previous 2 days | Every 1h, one day per unit |

Initial bounds are persisted when work is created. Do not move unfinished backfill
bounds forward each time the process restarts. Use separate checkpoints for routine
refresh and historical backfill. A fully fetched empty historical day can be recorded
and advanced; known validation/transport failure cannot be advanced as complete.
Do not endlessly retry unavailable old history or interpret it as zero energy.

## Implementation checkpoints

1. Startup opens/migrates storage, reconciles interrupted attempts, serves local UI,
   and performs discovery asynchronously. Enqueue only selected configured identities.
   Selection/configuration failure remains source/stream-specific.
2. Represent durable job kind/identity/range/window, next due, retry state and last
   completed unit. Reuse pending/running work instead of making duplicate jobs on
   every startup. Derive next unfinished unit from completion state, not max timestamp.
3. Use one fetch lane per provider initially, interleaving latest/routine/backfill
   at window boundaries. Distinct providers may fetch simultaneously. No execution
   overlap for the same job. A long backfill yields after every bounded unit so
   routine work can run. Do not hold storage locks during network calls or sleeps.
4. Fetch/validate via provider modules. One accepted transaction advances its unit
   and success audit. Failure/rejection records audit/evidence and retry due state
   without changing accepted observations/completion checkpoint. A later window
   cannot be used to hide a failed earlier checkpoint.
5. Consolidate retry ownership: transient HTTP retries have a bounded per-window
   budget and every scheduler reattempt gets a new ID. Follow provider handoff limits;
   document exact limits so nested layers cannot multiply retries accidentally.
   Start with exponential bounded backoff/jitter; supported long Retry-After sets
   a later due time instead of sleeping indefinitely. Permanent auth/config/selection
   errors stop rapid retries and remain visible until config/discovery recovery.
6. Expired history availability is not a transient outage. Mark empty/unavailable
   fetched coverage and move to next bounded historical unit. Recent overlap may
   revisit it according to routine policy; never refetch the archive on every tick.
7. Expose config-based explicit historical replay through documented start/end
   values (OCTOPUS_RANGE_REFRESH_START/END or GROWATT_RANGE_REFRESH_START/END)
   with ENERGYTHING_RANGE_REFRESH_ID. Persist request ID and bounds; enqueue once
   across restarts. Reusing an ID with different bounds is configuration error;
   a new ID intentionally repeats a range. A historical replay creates bounded
   work distinct from routine refresh. A changed identity creates new jobs and keeps
   old history; extending a start date enqueues only the newly requested old range.
8. Implement POST /api/v1/refresh with body selecting a configured source/stream.
   It enqueues routine work only, coalesces pending/running equivalents and returns
   202 with job IDs. Validate Basic Auth, token and origin before any mutation.
   Enforce method/body limits; a foreign/absent CSRF token must not enqueue work.
9. On shutdown stop enqueue, cancel fetches/waits, finish or roll back active storage
   transactions, mark cancellation when possible, join workers and close DB last.
   On crash, startup marks orphan attempts interrupted and requeues the same unit.
10. Write schedule/window/retry docs and update status counts/due/running/backfill
    fields using stored state. The UI refreshes status locally; GET cannot trigger clouds.

## Deterministic verification matrix

| Scenario | Required result |
| --- | --- |
| Advance fake clock through defaults | Expected jobs become due once; no actual hour-long waits |
| Trigger twice while job runs | One execution; refresh request returns existing job ID |
| Large backfill plus due latest/routine | Routine executes between bounded units; backfill remains resumable |
| Provider A blocks/fails, B healthy | B collects and UI responds |
| Crash after commit / before commit | Completed unit skipped / same unit retried on restart |
| Rejected interval/day | No accepted/checkpoint updates; durable rejected audit and retry state |
| Empty old history | Recorded availability, no fabricated zeros or endless archive retry |
| Retry-After longer than inline budget | Future due time, no blocked provider lane during wait |
| Cancel HTTP/backoff | Prompt shutdown without partial progress |
| Extend historical start / change meter | New bounded work only; prior history retained |
| Malformed refresh, wrong method/token/origin | Safe 400/405/403; no job enqueued |
| Repeated legitimate refresh requests | Bounded coalesced queue, no goroutine explosion |

Run clock/HTTP tests, real storage restart integration and go test ./.... Perform
browser smoke through an injected provider outage and observe automatic behavior
in a synthetic container. Hand off actual budgets, job states, config replay names
and audit fields; task 06a consumes them without adding its own scheduler.

