# 06 — Durable scheduling, bounded backfill and recovery

Status: planning draft. Depends on: 04, 05. Unlocks: 06a, 07.
Read: [execution contract](execution-contract.md) and both provider handoffs.

## Objective and boundary

Replace provider one-shot wiring with durable independent jobs. Automatic refresh,
backfill and authenticated refresh-now must survive restart without archive-wide
refetch. Collection remains inside this process; no cron/worker container or
complex management UI. Attempt audit is emitted here; browsing it belongs to 06a.

## Revised user decision — automatic collection, no startup/backfill switches

On 14 October 2026 the user rejected requiring BACKFILL environment variables or
`ENERGYTHING_OCTOPUS_ONCE` for normal operation. This revision supersedes the older
30-day/7-day configurable-start defaults and environment-based historical replay.
Credentials and any necessary confirmed meter/device selection are sufficient.
The server starts collection automatically; operators do not run an initialization
step, toggle a one-shot flag or choose a historical start date.

Task 04's one-shot path is temporary verification wiring, not the final operating
model. Task 06 must remove it and its configuration/Compose/example/documentation
entries, along with both BACKFILL_START variables. Do not keep obsolete variables
as hidden gates or let their stale values disable ordinary collection. Document
migration clearly without printing values. Existing accepted data and typed
attempt/window guarantees remain intact. This planning revision does not implement
scheduling in task 04 or authorize starting task 05/06 now.

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
| Octopus import/export backfill | Recent data first, then all discoverably available history; no configured start | At most 14 absolute days per unit; resumable |
| Octopus routine refresh | Selected direction's recent 7 calendar days through elapsed current intervals | Every 1h |
| Growatt history backfill | Recent data first, then verified available provider history; no configured start | One day per unit; resumable |
| Growatt latest | One latest response | Every 5m |
| Growatt history refresh | Current day and previous 2 days | Every 1h, one day per unit |

For every newly selected identity, enqueue recent collection immediately and
initialize a durable historical plan automatically. This is per-identity bootstrap
state, not a global `database row count == 0` check: an empty accepted response,
partly populated database or one populated stream must not suppress other work.
Adopt task 04's accepted windows instead of restarting their history; retain
unfinished/failed windows and interrupted attempts for normal retry/recovery.

Persist historical target/frontier, requested bounded units and completed windows.
Restart resumes these records, including accepted empty windows; neither a maximum
measurement timestamp nor the earliest retained row establishes fetched coverage.
Routine refresh and historical progress stay separate. Existing data without a
bootstrap record must be reconciled into a plan, not treated as history complete.

## Discovering the historical boundary — required protocol gate

The goal is all history accessible for each confirmed selected identity, with no
operator-supplied start date. Task 06 must verify a provider-supported way to find
the earliest accessible reading or an authoritative supported retention boundary.
Do not assume account move-in, tariff dates, the oldest retained observation or an
empty response establishes the earliest available meter history. Do not hardcode
30 days, a year or another arbitrary lookback as “all available history.”

Investigate a minimal earliest-reading metadata/list probe if the provider supports
it, preserving exact authenticated destination rules, cancellation, body/request
limits and retry ownership. A probe may locate a bound; observation ingestion still
uses the established <=14-day Octopus windows/provider-day Growatt units and full
validation. No unbounded observation download or archive scan inside discovery.
Verify ordering, gaps, pagination, identity and availability semantics against
primary sources and synthetic fixtures before using a probe's answer as a boundary.

Where no authoritative earliest probe exists, establish and document a finite,
provider-supported search domain and bounded resumable search strategy. Persist
search progress, yield to routine work and expose its stopping reason. Empty
windows can be recorded and skipped within that domain but cannot by themselves
prove there are no older readings; failures/rejections never prove absence.
Never stop merely after an arbitrary number of consecutive empty windows or assume
availability is monotonic across time. No binary search based on that assumption.

The gate must result in an executable automatic policy, exact budgets and proof of
its stopping rule before declaring task 06 complete. If evidence cannot establish
a safe boundary, record the specific unresolved source gate while recent collection
continues; do not ask the operator to invent a date or silently claim historical
completion. A bounded search limit is visible as a limit, not a source lifetime.
Historical completion is scoped to the selected identity, verified boundary and
observation time; retain its provenance and allow recovery if provider availability
expands. Historical/deselected meters are not automatically merged or fetched.

## Implementation checkpoints

1. Startup opens/migrates storage, reconciles interrupted attempts, serves local UI,
   and performs discovery asynchronously. Enqueue only selected configured identities.
   Selection/configuration failure remains source/stream-specific. No one-shot flag
   or backfill-date setting is required; bootstrap new identities automatically.
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
7. Normal backfill is automatic and needs no replay configuration. For exceptional
   historical repair, provide an explicit CLI enqueue command with confirmed local
   identity, paired bounded dates and an operation ID. Reuse typed durable work and
   immutable range-ID binding; no direct observation SQL, competing worker process
   or server restart is needed. Verify safe cross-process queue access with the
   running server; the CLI must not run startup interrupted-attempt recovery or
   reconcile another live process's attempts. The server worker performs the fetch.
   New IDs intentionally
   repeat ranges; reuse with different bounds is rejected. Remove the old
   RANGE_REFRESH_START/END and ENERGYTHING_RANGE_REFRESH_ID startup controls from
   config/Compose/examples in favor of this explicit action. No selection-management
   page/endpoint or complex job UI is required. A changed identity gets new automatic
   bootstrap work while keeping old history.
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
| First startup with credentials/selections only | Recent collection and historical bootstrap start asynchronously; no ONCE/BACKFILL vars |
| New empty identity beside populated stream | Independent bootstrap, even when the database already contains other data |
| Accepted empty response / partial existing history | Persisted progress reconciled; no bootstrap loop or false historical completion |
| Gap followed by older readings | Empty windows do not prematurely terminate history discovery |
| Earliest-boundary probe / unavailable probe | Verified bound and provenance / explicit unresolved gate, recent collection continues |
| Bounded historical search / supported retention | Persisted search yields to routine work; finite tested stopping rule, no invented oldest date |
| Restart during boundary search / backfill | Same frontier and unfinished unit resumed; no archive-wide rediscovery/refetch |
| Obsolete ONCE/BACKFILL values present | No hidden collection gate; migration behavior is documented and tested |
| Explicit repair CLI / changed identity | Durable coalesced bounded work only; prior history retained |
| Malformed refresh, wrong method/token/origin | Safe 400/405/403; no job enqueued |
| Repeated legitimate refresh requests | Bounded coalesced queue, no goroutine explosion |

Run clock/HTTP tests, real storage restart integration and go test ./.... Perform
browser smoke through an injected provider outage and observe automatic behavior
in a synthetic container. Hand off actual budgets, job states, automatic bootstrap/boundary policy and repair CLI
and audit fields; task 06a consumes them without adding its own scheduler.

## Required Growatt time-resolution gate (user decision, 15 October 2026)

Before task06 is declared complete, investigate the task05 timestamp uncertainty
using authoritative provider semantics and bounded private read-only comparisons:
readable time versus calendar epoch/components/timezone, utcTime/createTime,
requested display timezone, latest versus history, and daylight-saving transitions.
Establish which fields identify measurement time versus retrieval/server serialization,
with a tested precedence/contradiction policy and stable identity where evidenced.
Do not simply carry the uncertainty forward without investigation. If evidence
still cannot establish UTC or repeated-hour identity, document the exact blocker
and a concrete follow-up; keep qualified evidence and recent collection available,
without guessing offsets, silently rewriting existing rows or declaring resolution.
