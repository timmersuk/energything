# 04 — Octopus discovery and bounded import/export collection

Status: planning draft. Depends on: 02, 03. Unlocks: 06.
Read: [execution contract](execution-contract.md), storage handoff and HTTP contract.

## Objective and boundary

Discover confirmed electricity import/export meters, fetch validated bounded
windows, and retain accepted observations with independent status and attempt audit.
This task provides an explicit one-shot collection path; durable recurring schedules
and refresh enqueue belong to 06. No gas, tariffs, monetary totals or CSV imports.

## Existing evidence and deliberate reuse

Read the sibling octopull files locally:

| Reference | Reusable knowledge | Required change |
| --- | --- | --- |
| internal/export/api.go | Account shape, nullable is_export, URL validation, disabled redirects, bounded windows, pagination/retry | Existing Select/Readings only supports export; add confirmed import deliberately; make requests and waits cancellable |
| internal/export/data.go | Exact json.Number/big.Rat interpretation, interval alignment/overlap checks | Do not reuse file archive format or a summary that treats conflicting data as acceptable partial totals |
| internal/export/export_test.go | Synthetic selection/window/completeness cases | Review fixtures individually; copy no household identifiers/private artifacts |

Production account URLs in that code use https://api.octopus.energy/v1/.
Treat this as inspected reference behavior; verify current API semantics during
implementation. Do not execute the old CLI or load its private configuration.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/octopus/client.go | Destination allowlist, auth, timeout, cancellable HTTP and bounded bodies |
| internal/octopus/discovery.go | Minimal account model and explicit import/export selection |
| internal/octopus/readings.go | Bounded half-open fetch, pagination and response assembly |
| internal/octopus/validate.go | Whole-window interval/decimal/conflict validation |
| internal/collection/octopus.go | One-shot attempt orchestration using storage contract |
| internal/web/status.go | Render stored import/export discovery/result state |
| internal/octopus/*_test.go | Synthetic HTTP/protocol cases |
| docs/data-semantics.md | Confirmed Octopus boundary/coverage semantics |

## Selection behavior

Confirmed false is import, confirmed true is export, absent/null direction is unknown.
Identity includes MPAN, serial and direction. Deduplicate identical discoveries;
conflicting descriptions of the same identity block selection. Explicit MPAN/serial
must match discovered confirmed identity; never trust a configured direction without
checking discovery. For each direction, auto-select only a unique eligible identity.
Zero or multiple candidates produce unavailable/selection-required status for that
stream. Unknown-direction points are not quietly selected. Import can operate while
export is absent/ambiguous and vice versa. Persist only necessary identity metadata.

## Implementation checkpoints

1. Model account fields minimally and implement pure selection tests before HTTP.
   Keep discovery and errors free of addresses/full account payload persistence.
2. Configure the client with a production destination policy independent of the
   injectable test transport. Every authenticated initial/page request must have
   HTTPS, the exact allowed host, no userinfo and an allowed path. Disable redirects;
   never attach credentials to a pagination URL before validating it. Verify page
   URLs still address the intended meter endpoint and bounded query, not just host.
3. Fetch one bounded window at a time, at most 14 absolute days. Application bounds
   are [from,to); adapt the provider's inclusive/overlap behavior explicitly. The
   reference subtracts one second from period_to; validate that behavior rather
   than widening into overlapping application windows. Neighbor boundary intervals
   wholly outside the requested unit may be excluded with recorded context; an
   interval straddling a boundary is a validation error, not silent truncation.
4. Bound page count, response bytes and record count. Establish conservative limits
   from 672 possible half-hours per 14-day window plus harmless duplicates/boundaries;
   test excessive/malicious responses. A missing results field differs from [].
   Detect cycles and repeated pages; order returned pages does not decide validity.
5. Assemble all pages before validation. Require exact nonnegative decimal values,
   30-minute duration and absolute half-hour alignment. Collapse identical readings;
   differing values/end times for the same start, nonaligned intervals, overlap,
   negative/nonfinite/unparseable values reject the entire window. Do not commit
   other readings from that response or replace previous accepted data.
6. A complete valid response with gaps may be accepted with missing coverage.
   Calculate expected elapsed slots using absolute London day bounds, not fixed 48.
   Preserve retained observations absent from a refresh; report latest-fetch coverage
   separately from retained coverage. Do not infer why slots are missing.
7. Execute through BeginAttempt/CommitAcceptedWindow/RecordRejectedOrFailedAttempt.
   Persist selected telemetry evidence including rejected content, sanitized reason,
   requested bounds, page/retry context and times. Later valid corrections update
   current accepted rows with prior evidence retained. No progress on rejection.
8. Add an explicit one-shot startup/discovery ingestion hook sufficient for local
   end-to-end verification, cancellable and nonblocking for browsing. Task 06 will
   replace/wire it into durable jobs; do not add independent long-lived timers here.
9. Privately verify actual import/export discovery and bounded ingestion only when
   runtime credentials are available. Keep evidence outside public source/CI.

## Verification matrix

| Synthetic scenario | Expected result |
| --- | --- |
| True/false/null direction plus duplicate discovery | Correct independent selection; unknown never selected |
| Two eligible imports, one export | Import needs explicit selection; export can collect |
| Wrong configured serial/MPAN | Actionable stream error; no request for guessed meter |
| Foreign host, redirect, changed meter path, cycle | Rejected before credential-bearing request |
| Empty results vs missing results | Empty coverage vs protocol failure |
| Second page fails after first page | No accepted rows/checkpoint; failed attempt and available evidence |
| Same reading twice, then conflicting values | Identical dedup accepted / whole window rejected |
| One negative, overlap or invalid duration | Whole window rejected; prior data unchanged |
| Boundary neighbor and adjacent windows | No double-count/dropped valid boundary interval |
| Changed valid reading on later refresh | Correction accepted; prior evidence retained |
| 429/Retry-After, 5xx, auth failure, cancellation | Bounded correct classification; no secret leak |
| Both UK DST days and partial current day | Correct expected elapsed slots and coverage |

HTTP retry budget is owned in one layer and documented; task 06 must not multiply
it with hidden inner retries. Waits respect cancellation and long Retry-After defers
work instead of blocking the service. Run package/integration tests and go test ./....

## Handoff and completion

Hand off discovery states, bounded fetch input/result types, destination and boundary
rules, retry ownership, synthetic proofs and private validation status. Do not claim
source live validation if credentials are unavailable. No pricing logic from octopull
is in scope even though its neighboring modules implement it.
