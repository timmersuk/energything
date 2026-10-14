# Milestone 1 shared execution contract

Status: planning draft, incorporating confirmed interview decisions on 14 October
2026. Read this file with the task being executed and the parent milestone plan.
This contract provides common behavior; it does not authorize implementing other tasks.

## Authority and change boundaries

1. Human decisions recorded below override older contradictory planning text.
2. This contract and the individual task specify milestone 1 implementation.
3. The overall goal supplies future context, not additional current scope.
4. Logthing supplies deployment/lifecycle conventions; octopull supplies reviewed
   protocol knowledge. Neither authorizes copying unrelated functionality or data.

If evidence invalidates a contract, report the specific contradiction and propose
an amendment before implementing different behavior. Do not paper over a gate by
inventing timestamps, accepting partial data or adding a replacement architecture.
Internal helper names and file splits may change within the named modules; public
routes, data semantics and cross-task behavior require coordinated doc updates.

## Confirmed decisions

- Single Go process/binary/container; embedded html/template, CSS and modest JS.
  No Node build, React, ORM, general source plugin framework or second primary store.
- Pure-Go SQLite is subject to the initial build/runtime gate. /data is a local
  filesystem bind mount; Linux amd64/arm64 and Docker Hub follow logthing.
- Keep all selected telemetry evidence and superseded versions initially. Repeated
  identical content can be deduplicated; retrieval events remain separately recorded.
  No aggregation, pruning or lossy compaction in milestone 1.
- Reject the entire bounded collection window when its readings are invalid or
  contradictory. Preserve accepted readings and completion progress. Retain suspect
  telemetry evidence and a timestamped, source-attributed error audit.
- Durable audit is available through authenticated REST endpoints and the web UI.
  Discord alerts are deferred; do not add webhook configuration or delivery workers.

## Superseding collection decision (14 October 2026)

The user subsequently rejected startup one-shot and backfill-date environment
controls. Task 06 must automatically bootstrap each selected configured identity,
collect recent data and resumably recover discoverably available history. No
operator initialization step or historical start date is required. Its revised
[task 06 brief](06-scheduling.md) owns the earliest-history protocol gate, exact
bounded search/stopping policy and migration. Empty windows are not proof that no
older readings exist. Existing partial data must not suppress historical work.

`ENERGYTHING_OCTOPUS_ONCE` and the BACKFILL_START variables exist only in the
intermediate implementation and are removed in task 06, including examples and
Compose passthrough. Exceptional historical repair becomes an explicit durable
CLI enqueue action rather than RANGE_REFRESH/REQUEST_ID startup environment values.
The older variable rows and scheduling paragraphs below describe the original
baseline; this decision and revised task 06 supersede them for implementation.
Do not restore those controls as a prerequisite or silently use a short lookback
as the complete available archive. Existing storage/HTTP safety contracts remain.

## Module ownership and coupling

| Module | Owns | Must not own |
| --- | --- | --- |
| cmd/server | configuration, wiring, startup, cancellation/shutdown | provider pagination or SQL |
| internal/storage | migrations, observations, evidence, jobs, audit and indexed queries | HTTP calls, templates |
| internal/octopus | account discovery, meter selection, bounded fetch and validation | scheduler timers, direct SQL |
| internal/growatt | read endpoint allowlist, discovery, units/time interpretation, bounded fetch | settings writes, direct SQL |
| internal/collection | job state, due work, retries, priorities, commits and attempt audit | HTML rendering, provider response field guessing |
| internal/web | authenticated pages/API, query validation, rendering and refresh enqueue | cloud calls, migrations or ad hoc SQL |

Use narrow concrete methods or consumer-owned interfaces, with context cancellation.
Do not invent a common provider interface unless the actual callers need it. Inject
HTTP and time/wait dependencies where they permit deterministic failure/retry tests.

## Configuration contract

The following names are the proposed implementation baseline. Defaults here are
engineering starting points, not claims about provider limits. Document any changed
baseline in the task before handoff to a dependent task.

| Variable | Meaning and default |
| --- | --- |
| ENERGYTHING_HTTP_ADDR | Listen address; :8080 |
| ENERGYTHING_DATA_DIR | Directory holding energything.sqlite and companions; /data in image, ./energything-data locally |
| ENERGYTHING_USERNAME / ENERGYTHING_PASSWORD | Required, nonempty; no shipped defaults |
| ENERGYTHING_TIMEZONE | Europe/London; validate IANA location at startup |
| OCTOPUS_API_KEY / OCTOPUS_ACCOUNT_NUMBER | Both needed to configure Octopus; no automatic .env loading |
| OCTOPUS_IMPORT_MPAN / OCTOPUS_IMPORT_SERIAL | Explicit import selection when needed |
| OCTOPUS_EXPORT_MPAN / OCTOPUS_EXPORT_SERIAL | Explicit export selection when needed |
| GROWATT_API_TOKEN | Configures Growatt; no automatic .env loading |
| GROWATT_PLANT_ID / GROWATT_DEVICE_SERIAL | Explicit discovery selection when needed |
| OCTOPUS_BACKFILL_START / GROWATT_BACKFILL_START | Optional YYYY-MM-DD in configured calendar timezone; bounded start, never a future date |
| OCTOPUS_REFRESH_INTERVAL | 1h; positive duration |
| GROWATT_LATEST_INTERVAL | 5m; positive duration |
| GROWATT_HISTORY_INTERVAL | 1h; positive duration |
| OCTOPUS_RANGE_REFRESH_START / OCTOPUS_RANGE_REFRESH_END | Optional paired YYYY-MM-DD bounds; inclusive calendar dates |
| GROWATT_RANGE_REFRESH_START / GROWATT_RANGE_REFRESH_END | Optional paired YYYY-MM-DD provider-date bounds; inclusive dates |
| ENERGYTHING_RANGE_REFRESH_ID | Required nonempty request ID when either range refresh is configured; persist ID/ranges and enqueue once; new ID deliberately repeats a range |
| ENERGYTHING_SHUTDOWN_TIMEOUT | 10s; positive duration |
| ENERGYTHING_IMAGE / ENERGYTHING_HTTP_PORT / ENERGYTHING_DATA_PATH | Compose substitution values, separate from process configuration |

Invalid global auth/address/timezone/data configuration is fatal before serving.
A missing/partially configured provider disables that provider with actionable,
sanitary status; the other provider and browsing remain usable. Invalid provider
selection/dates/intervals similarly block its jobs, not the entire service.
Secrets are never echoed in parse errors. No secret values go into SQLite.
No sources configured is a supported empty UI; no collection work is enqueued.

## Observation identity and values

- Octopus logical identity includes provider, MPAN, serial and confirmed direction.
  A replacement meter is a different identity; deselection never deletes its history.
  The interval key is identity plus absolute start; end/duration is separately validated.
- An electricity value is retained as an exact decimal representation, not SQLite
  REAL or a float64 round trip. JSON APIs return decimal kWh as strings. Zero is valid.
- UTC instants use one consistent storage representation and sort chronologically.
  Preserve source timestamp fields; retrieval and measurement times are distinct.
- Growatt logical identity includes plant/device identifiers established in discovery.
  Sample key rules are finalized at the protocol gate, not guessed from local text.
  An unresolved sample may have no normalized instant; evidence remains queryable.
- Normalize only units/signs supported by protocol evidence. Nullable SOC/power/counter
  values carry unit and quality metadata. A missing field is never a zero.
- Evidence is selected telemetry response data sufficient to reproduce interpretation,
  including unknown telemetry fields. Remove credentials, headers and discovery
  account/address material before persistence. It is not a full HTTP transcript.

## Window and commit contract

A window is a bounded provider fetch unit: at most 14 absolute days for Octopus;
one provider-calendar day for Growatt history; one response for Growatt latest.
All pages belonging to the unit must be fetched and validated before acceptance.
A larger backfill is a sequence of independently committed windows.

1. Register an attempt with UTC start, source/stream/device reference, job/window
   identity and requested bounds. Do not hold a DB transaction while fetching.
2. Fetch bounded pages. Missing required structure, pagination cycles, untrusted
   destinations, truncation and count mismatch are failures; HTTP 200 alone is not success.
3. Validate the assembled window independent of page/order. Identical observations
   may be deduplicated; differing values/end times for one logical interval and
   overlaps/invalid values reject the unit. Evidence, not arrival order, determines validity.
4. On acceptance, one short transaction writes observations/evidence, coverage,
   completion checkpoint, attempt success and due state. A later accepted window
   can update changed valid readings; preserve earlier evidence. Absence does not delete.
5. On failure/rejection, a separate short transaction records failure audit and
   suspect evidence without changing accepted readings, coverage success or completion
   checkpoint. Retry scheduling/attempt counters may change. If that transaction
   fails, do not claim a durable audit event; report storage failure and retain no
   false completion state. Disk-full cannot be solved by silently deleting history.

Unknown optional Growatt fields or unresolved time interpretation are quality
states, not automatically malformed readings. Preserve them under the explicit
protocol gate rules. Known impossible values, contradictions in required fields
and structurally incomplete pages reject the unit. Task 05 must enumerate which
fields are optional, invalid or unresolved before building its normalizer.

Empty fully fetched history may complete an attempt as empty/unavailable; it does
not establish zero energy, erase older observations or establish full coverage.
For Growatt, transport/page completeness and measurement coverage are separate.
For Octopus, report expected elapsed half-hour slots, valid/missing/quality slots;
a day with missing readings can be fetched successfully without becoming complete
measurement coverage. Current/future slots are not expected before they have elapsed.

## Durable attempt audit

Keep all audit entries initially. Separate the internal attempt identifier from
masked display identity. One attempt has a start and one terminal result, including
interrupted after restart when the previous process did not finish.

Required fields: event/attempt ID; UTC occurrence/start/finish times as applicable;
provider and stream; local identity reference; job kind; window bounds or requested
provider date; outcome; sanitized error class/code and message; page/index where
relevant; retry count/next retry when applicable; evidence references; accepted
row count on success. Mask provider identifiers in API/UI output by default.

Terminal outcomes: success, empty, rejected, failed, cancelled, interrupted.
Error classes distinguish configuration/selection, authentication, rate_limit,
network/timeout, pagination/protocol, validation, timestamp_quality and storage.
Timestamp-quality evidence may accompany accepted qualified samples; it must not
be described as a failed fetch if the fetch itself succeeded.

Low-level HTTP retries are bounded details within an attempt, recording status/code
and wait duration without raw URLs/bodies/secrets. A scheduler reattempt has a new
attempt ID. A configured range-refresh ID cannot be reused for different bounds;
report a configuration error rather than silently changing persisted work.
Audit failures are visible even when last successful collection is old.
Queries support source, outcome/error class and time range, newest first with a
stable ID tie-breaker. Provide counts of failed/rejected attempts over a requested
period so repeated incidents can be identified without loading every row.
No raw telemetry or stack traces in public liveness or routine logs.

## HTTP and browser contract

| Method and path | Purpose | Authentication |
| --- | --- | --- |
| GET /healthz | Process liveness only; no source/database details | Public |
| GET / | Status page | Required |
| GET /records/octopus | Stored import/export table | Required |
| GET /records/growatt | Stored resolved/unresolved observations | Required |
| GET /audit | Collection attempt/error history | Required |
| GET /api/v1/status | Stored source/job/coverage status | Required |
| GET /api/v1/octopus/intervals | Stored electricity query | Required |
| GET /api/v1/growatt/samples | Stored Growatt query | Required |
| GET /api/v1/audit | Stored attempt audit query and counts | Required |
| GET /api/v1/audit/{id} | Sanitized attempt detail/evidence references | Required |
| POST /api/v1/refresh | Enqueue configured-source routine refresh | Required plus CSRF protection |

Protect static assets/fragments with the same credentials. No GET performs a cloud
request or changes job state. Refresh returns 202 with job IDs promptly; duplicate
pending/running work coalesces. Unknown sources, invalid selectors/ranges/cursors
return 400, unknown record/event IDs 404, wrong methods 405 and failed CSRF checks 403. Return bounded JSON
error envelopes {"error":{"code":"...","message":"..."}}, never raw provider errors.

Query baseline: from/to are ISO dates for UI (inclusive dates mapped to half-open
absolute range) or explicitly offset RFC3339 instants for API; do not accept mixed
formats in one range. Default page size 100, maximum 500. Use opaque validated
keyset cursors, deterministic ascending records by instant/key, descending audit
by occurrence/id. Bind cursor to query filters; malformed/mismatched cursors are 400.
Unresolved Growatt time cannot be forced into an absolute range: offer a separate
quality view filtered by provider date when known, otherwise retrieval range.
Expose how many unresolved observations are omitted from an absolute-time query.

Use text/template escaping behavior through html/template, not trusted HTML from
providers. Basic Auth has no default credentials. For mutation use a process-secret
HMAC CSRF token embedded in authenticated forms, validate same-origin when supplied,
and require the token for API callers too. Reject missing/wrong tokens before enqueue.
Restart invalidates old tokens; reloading the page supplies a fresh one.

## Initial safety budgets

These are explicit engineering defaults to prevent unbounded execution, not
asserted provider limits. A provider gate may justify a documented amendment.

- HTTP request timeout: 30s; maximum response body: 4 MiB per page. Oversize or
  truncated JSON is a failed unit, never a partial accepted response.
- Maximum 100 pages and 10,000 assembled observations per window. Stop with a
  protocol/budget error on excess; do not truncate and advance completion.
- At most 3 HTTP attempts per page for network errors, HTTP 408/429 and 5xx;
  do not retry invalid JSON, known validation failure, 401/403 or selection errors
  inside that attempt. Ordinary retry delays start at 1s then 2s, with bounded jitter.
- A supported Retry-After beyond 30s defers the job to that instant rather than
  holding its lane. Parse HTTP seconds/date formats and test cancellation.
- Scheduler transient/validation reattempts start after 1 minute and double to
  a maximum 1h delay, with bounded jitter. Keep attempt history. Authentication/
  configuration errors pause that job until configuration/restart or a later
  explicit rediscovery resolves them; avoid repeated credential storms.
- One fetch lane per provider, one serialized write lane, page sizes of 100 for
  browsing with a 500 cap. Growatt starts with sequential history pages.
- Keep one pending routine refresh per job, coalescing equivalent triggers;
  historical ranges are persisted bounded units, not thousands of spawned goroutines.

Task 04/05 can narrow these budgets based on verified protocol behavior; task 06
must use the published single retry policy rather than layering new hidden attempts.

## Job state and recovery

Persist job identity, selected source identity, kind, bounded window, requested
policy/range, next_due, attempt counters and completed checkpoint. Runtime running
state is reconciled on startup: old running attempts become interrupted and the
same unfinished window becomes eligible. Do not advance to the next window.

No two executions of the same job overlap. Independent sources may fetch concurrently;
SQLite writes are serialized. Start one fetch lane per provider; schedule bounded
windows so routine work takes precedence between historical windows. Historical
backfill can make progress when routine work is idle; no unbounded goroutine per day.
Start with sequential Growatt history pages; concurrency is optional only after
count/order behavior is established and tested. Storage/query failure is distinct
from provider failure and may prevent durable progress for all jobs.

Recent refresh and backfill need distinct durable checkpoints. Completed backfill
never reruns all history on restart. Changing selected identity creates new work;
changing start to an older date enqueues only the newly requested old range. An
explicit historical-range configuration action is separate from routine refresh-now.
Retries use cancellable bounded waits, respect supported Retry-After, and yield to
other due work. Never mark a partially fetched or rejected window complete.

## Evidence and review completion

Each task handoff records files changed, checks actually run and results, gates
still open, and deviations from this contract. Use synthetic fixtures only. If a
required tool/runtime/live source is unavailable, state the exact unverified gate;
do not label the task or milestone complete on assumed success.
Later milestone features, deployment, publishing and private provider access are
not implied by authoring this plan. This session changes documentation only.
