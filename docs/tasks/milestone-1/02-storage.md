# 02 — Durable observations, progress, evidence and audit

Status: planning draft. Depends on: 01. Unlocks: 03, 04, 05 and 06a storage contract.
Read: [execution contract](execution-contract.md) and [milestone](../../plans/milestone-1-service-and-collection.md).

## Objective and boundary

Establish durable storage semantics using synthetic records. Accepted observations,
evidence, checkpoints and attempt audit must remain coherent across replay,
rejection and restart. This task contains no cloud requests, timers or web pages.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/storage/migrations/ | Ordered versioned SQL migrations |
| internal/storage/schema.go | Migration bookkeeping and transactional startup upgrade |
| internal/storage/records.go | Typed identities, intervals, samples, evidence and quality |
| internal/storage/commit.go | Accepted-window transaction and rejected/failed attempt recording |
| internal/storage/jobs.go | Durable due work/checkpoints and restart reconciliation |
| internal/storage/query.go | Indexed records/status/audit queries and pagination |
| internal/storage/*_test.go | Real temporary SQLite integration tests |
| docs/data-semantics.md | Logical keys, correction rules, nullable time and evidence links |

File splits may vary; all SQL and migrations remain in storage. Consumers receive
plain record/query/result types, not *sql.DB, rows or a generic SQL escape hatch.

## Logical schema contract

Define migrations and Go types for the following logical entities. Exact SQL
column names may vary; keys, nullability and relationships may not silently vary.

| Entity | Required state and invariant |
| --- | --- |
| Source identity | Provider, stream/direction, stable provider identity, selection and necessary discovery metadata; unique logical provider identity |
| Electricity observation | Identity, start/end, exact decimal kWh, retrieval time and evidence/attempt link; current accepted row unique by identity/start |
| Growatt observation | Device, resolved key if established, original time fields, nullable normalized instant, quality, nullable selected values/units, retrieval/evidence links |
| Evidence | Sanitized selected telemetry content, content digest, source/attempt association; identical content can share storage without losing retrieval links |
| Window completion | Identity/job kind/bounds, latest accepted attempt and coverage; not merely newest record time |
| Job | Identity/kind, bounded requested work, next due, completion checkpoint and retry state |
| Attempt/audit | Shared-contract timestamps, outcome, sanitized error details, requested window, page/retry context and evidence references |

Use integer absolute time at a documented precision for sortable instants; original
provider text is separate. Use TEXT for exact decimal kWh, validated before commit.
Nullable Growatt values must round-trip as null. Use foreign keys for identity,
attempt and evidence associations. Retain historical identities after deselection.

## Implementation checkpoints

1. Write synthetic examples and the logical schema in docs/data-semantics.md before
   migrations. Cover a correction, identical replay, rejected window, empty history,
   repeated local time and unresolved identity/time. These examples define storage
   behavior without requiring unknown Growatt protocol decisions.
2. Implement migration version checks. Empty database upgrades to current; reopen
   is idempotent. Migration failure rolls back its version/data changes and aborts
   startup. A newer unsupported schema fails clearly rather than downgrading.
3. Provide methods equivalent to UpsertIdentities, BeginAttempt,
   CommitAcceptedWindow, RecordRejectedOrFailedAttempt, RecoverInterruptedAttempts,
   QueryIntervals, QuerySamples, QueryAudit and QueryStatus. Document their typed
   inputs/outputs and transaction guarantees. Names are illustrative, not a framework.
4. Accepted commit is all-or-nothing: observation upserts, evidence/retrieval links,
   window coverage, checkpoint, terminal success/empty audit and next due state.
   A changed valid value updates the current view and keeps prior evidence. A
   repeated identical payload changes retrieval/attempt metadata, not row count.
5. Rejected-window recording writes suspect evidence and terminal audit separately
   from accepted data. It never changes accepted readings, accepted coverage or
   completion checkpoint. Attempt/retry scheduling can advance. Do not store rejected
   readings in the accepted table with a flag that a future query can accidentally ignore.
6. Incomplete transport attempts can retain received evidence tagged incomplete;
   never mark their page/window complete. Storage failure must leave no false success.
7. Model unresolved Growatt evidence separately enough that two observations with
   identical local clock text cannot overwrite one another. Task 05 supplies the
   resolved-key algorithm; task 02 must not invent one or enforce NOT NULL UTC.
8. Implement bounded indexed queries and aggregate status/count queries. Build query
   predicates from validated fields; parameterize values. Stable cursor pagination
   follows the shared contract. Status distinguishes attempted/fetched coverage
   from accepted measurement coverage and last retrieval from measurement time.
9. Keep writes serialized and transactions short. Cloud fetches occur elsewhere.
   Test actual driver connection PRAGMAs, lock/busy behavior and cancelled calls.

## Concrete synthetic verification

| Input/action | Required result |
| --- | --- |
| Commit two intervals and progress, reopen | Same records, decimal strings and checkpoint |
| Replay identical accepted window | No duplicate observations/evidence bytes; new attempt remains traceable |
| Later valid 0.100 → 0.120 reading | Current 0.120, prior evidence retained |
| Window with one invalid/conflicting reading | No accepted updates anywhere in window; rejection audit/evidence retained |
| Failure halfway through accepted transaction | No new records, completion or success audit |
| Missing slot in accepted response | Missing coverage; retained prior observations never deleted |
| Empty expired Growatt day | Empty/unavailable attempt; existing local history preserved |
| Null SOC vs SOC=0 | Distinct after reopen and API-facing query serialization |
| Two autumn local times with unresolved identity | Two retained evidence associations; no guessed UTC or overwrite |
| Old process has running attempt | Restart marks interrupted and leaves unfinished work eligible |
| Audit filtered by provider/outcome/time | Correct counts and stable paging, no raw secrets |
| Newer schema / locked database / disk write failure | Actionable error without false completion or silent pruning |

Use real temporary SQLite databases, not mock SQL. Inject transaction failures at
meaningful boundaries. Run package tests plus go test ./...; test migrations and
query plans against realistic synthetic row counts to verify identity/time indexes.
No benchmark suite or speculative aggregation layer is required.

## Handoff

Publish typed method signatures and schema/data semantics for later tasks, with
example requests/results. Explicitly identify the unresolved Growatt key/time gate.
No dependent task may bypass transactions or write ad hoc SQL to add its own state.
