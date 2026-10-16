# Stored browsing HTTP API (v1)

Task 03 implements local reads only. No request calls a provider or enqueues work.
All implemented routes accept GET only (405 with `Allow: GET` otherwise).

| Route | Result | Authentication |
| --- | --- | --- |
| `/healthz` | Fixed `ok` process liveness | Public |
| `/` | Stored source/configuration/job/coverage status | Basic Auth |
| `/records/octopus` | Import/export interval table | Basic Auth |
| `/records/growatt` | Resolved or quality/retrieval sample table | Basic Auth |
| `/api/v1/status` | Status JSON | Basic Auth |
| `/api/v1/octopus/intervals` | Interval JSON | Basic Auth |
| `/api/v1/growatt/samples` | Sample JSON | Basic Auth |
| `/static/style.css` | Embedded stylesheet | Basic Auth |

Missing/wrong credentials return 401 with a Basic challenge and no source data.
Unknown routes, including audit and refresh routes, return 404. Responses use
`Cache-Control: no-store`, nosniff and a same-origin content security policy.

## Record query parameters

Pages and APIs use the same parser. Unknown or repeated parameters are errors.
Empty optional form fields are ignored; an explicitly empty `limit` is invalid.

| Parameter | Meaning |
| --- | --- |
| `from`, `to` | Paired inclusive `YYYY-MM-DD` calendar dates, or paired offset RFC3339 instants with **exclusive `to`**. Mixed formats, missing bounds, invalid/reversed ranges fail. Precision is at most microseconds; UTC bounds stay within years 0001–9999. |
| `limit` | Default 100; integer 1–500 |
| `cursor` | Opaque storage keyset cursor, bound to all filters and limit |
| `identity` | Optional positive local source ID; must exist for this provider. History includes deselected sources. |
| `direction` | Octopus only: `import` or `export`; empty selects both |
| `view` | Growatt only: `resolved` (default) or `quality` |
| `quality` | Growatt quality view only: exact stored quality text, at most 256 bytes |
| `provider_date` | Growatt quality view only: `YYYY-MM-DD`, matches original provider date |

No range defaults to the current day in the configured calendar timezone
(Europe/London by default). A quality query with `provider_date` and no range
selects that date across all retrieval times. Explicit quality-view ranges apply
to **retrieval time**, including when a provider date is also supplied.
Quality view includes resolved and unresolved samples; `quality=unresolved` is
an exact quality label filter, not an inferred test of null measurement time.

Calendar dates become half-open UTC instants using calendar arithmetic. Thus
29 March 2026 covers 23 hours / 46 half-hour slots and 25 October covers 25 hours /
50 slots in London. Resolved UTC instants distinguish the repeated autumn hour.
Unresolved original clock text is never interpreted or sorted as absolute time.

Examples (the same parameters work on the corresponding HTML route):

```text
/api/v1/octopus/intervals?from=2026-03-29&to=2026-03-29&direction=import
/api/v1/octopus/intervals?from=2026-10-25T00%3A00%3A00%2B01%3A00&to=2026-10-26T00%3A00%3A00Z&limit=50
/api/v1/growatt/samples?from=2026-10-25&to=2026-10-25
/api/v1/growatt/samples?view=quality&provider_date=2026-10-25
/api/v1/growatt/samples?view=quality&from=2026-10-26&to=2026-10-26&quality=unresolved
```

Encode `+` as `%2B` in query offsets. Normal records ascend by measurement instant
and identity (Octopus) or local row ID (Growatt). Quality records ascend by retrieval
instant and row ID. `next_cursor` exists only if a further page exists; repeat the
same filters and limit with it. HTML next links preserve the explicit effective
range, including when the first request defaulted to today. No offset pagination.
This is a live keyset view, not a historical snapshot: new records behind the cursor
are seen on a fresh query; new records ahead can appear on later pages. Corrections
to Growatt measurement times may change ordering. Cursors are validated, not signed
authorization tokens; source IDs are local selectors, not access-control boundaries.

## Record envelopes and fields

Both record APIs return `version: "v1"`, `timezone`, `view`, `range_basis`
(`measurement` or `retrieval`), normalized `from`/`to` UTC strings (null when a
provider-date query has no retrieval range), `limit`, `records` (always an array),
optional `next_cursor`, and `omitted_unresolved`.

Octopus record fields: `source: "octopus"`, `identity_id` (local integer),
`identity` (direction, full MPAN and meter serial), `mpan`, `serial`, `direction`, `measured_at` (interval start UTC),
`end` (exclusive UTC), `retrieved_at` (UTC), `kwh` (exact decimal string),
`unit: "kWh"`, `quality: "accepted"`. Accepted means validated retained data,
not complete coverage. No aggregate conversion or floating-point round trip.

Growatt record fields: `id` (local row ID), `source: "growatt"`, `identity_id`,
full provider `identity`, `measured_at` (UTC string or null), `retrieved_at` (UTC),
`plant_id`, `serial`, `provider_date`, `original_time` (escaped provider field/text map), `quality`,
`values` (field-name map). Each value has `value` (decimal string or null),
`unit` (string or null) and `quality`. A decimal `"0"` is not null. Resolved keys,
opaque storage keys, evidence and account responses are omitted from normal records.

Resolved Growatt results exclude null-time samples. `omitted_unresolved` counts
all unresolved observations in the identity selection, independent of absolute
range, since no measurement day can be assigned to them. It is zero in quality
view and for Octopus. JSON escapes HTML-sensitive characters; pages use
`html/template` and never mark provider text as trusted HTML.

## Status envelope

`/api/v1/status` accepts no parameters. Fields: `version: "v1"`, `build_id`
(public build identifier), `timezone`, `providers` (Octopus and Growatt).
Each provider has `source`, `configuration` (`disabled`, `valid`, `error`),
sanitized `message`, `discovery` (stored identities available or none), `sources`.
Configuration validity does not prove provider authentication or discovery.

Each source has `identity_id`, `source`, `stream`, `identity`, `selected`,
`retained_count`, `unresolved_count`, nullable `last_retrieved_at` and
`newest_measured_at`, nullable `last_attempt` and `last_accepted_attempt`,
`job_count`, `window_count`, `jobs`, `coverage`. Newest Octopus measurement is
the newest interval end; Growatt uses newest established instant. A recent fetch
of old data does not advance measurement time. Retrieval includes retained
evidence of failed attempts, not only successful readings.

Attempt summaries contain `started_at`, nullable `finished_at`, `outcome` and
optional sanitized `error_class`/`message`; they expose no audit detail endpoint
or evidence. Jobs contain `kind`, `policy`, `running`, `paused`, `retry_count`,
`next_due`, nullable `checkpoint`. Coverage contains `from`, exclusive `to`,
`fetched`, `expected`, `valid`, `missing`, `qualified`, `measurement_complete`.
Job/coverage arrays cap at 500 per identity; full counts are separate. Empty
coverage means none stored; incomplete measurements do not establish an outage.
Startup storage recovery can mark abandoned running work interrupted; GET does
not change it. The static server-rendered UI has no asynchronous loading phase;
stored running work is shown explicitly instead.

## Errors

Errors are bounded JSON: `{"error":{"code":"invalid_query","message":"Invalid range, filter, limit or cursor"}}`.
400 uses `invalid_query` (status/encoding errors may say `Invalid query parameters`);
404 `not_found`; 405 `method_not_allowed`; 503 `storage_unavailable` with
`Stored data temporarily unavailable`; template failure is 500 `internal` with
`Page unavailable`. Authentication uses the task 01 safe authentication response.
No SQL, raw error text, credential value or provider response is returned.

## Task 04 discovery extension

The existing GET routes and record/status fields remain unchanged. Provider status
additionally has optional `streams`, containing `{stream,state,message,occurred_at}` for
stored Octopus discovery. Streams are independently `discovering`, `selected`,
`unavailable`, `selection_required`, `conflict` or `failed`; occurrence is UTC.
The message supplies static selection/recovery guidance. No account payload, raw identity or provider error is included. Configuration
validity and stored discovery remain distinct, including after credentials are
removed. These states and existing retained records are rendered on `/`.

An incomplete explicit selector with valid Octopus credentials is now a per-stream
selection gate, rather than disabling both directions during configuration parsing.
Global provider date/credential errors retain their existing configuration behavior.
The explicit startup worker is independent of GET requests; browsing never performs
cloud requests or changes jobs. No refresh/audit endpoint or scheduler was added.

## User-approved Octopus identity display

The user requested full MPANs for comparison with the Octopus website. Authenticated
Octopus records and status now include explicit `mpan` and `serial` fields and a
readable identity label such as `Import · MPAN 1234567890123 · meter SYNTH-METER`.
Identifiers are not masked in these views. Missing legacy discovery metadata is
labelled unavailable rather than inventing an MPAN or parsing an opaque stable key.
Local identity IDs and cursor/filter contracts remain unchanged. These routes
remain authenticated and no-store; keys, account/address payloads, raw stable keys
and telemetry evidence are omitted. Growatt identifiers follow the same owner-visible policy described below.

## Authenticated identity and diagnostics cleanup

The owner requested useful full identities throughout the authenticated application,
not just Octopus records. Growatt status/sample views now expose `plant_id` and
`serial` with readable labels when established metadata exists; missing metadata
is explicitly unavailable. The typed attempt audit likewise identifies actual
meter/device metadata. Opaque stable keys and credentials are not display fields.
Attempt summaries add `error_code` and `page` where available. New Octopus failures
retain specific enumerated validation/count/structure reasons rather than only a
broad failure class. Existing historical errors retain their recorded descriptions.
Task 06a will add authenticated selected-evidence inspection/download with typed
attempt/retrieval association checks, bounds and escaping. No audit/evidence route
is implemented early by this cleanup; current unknown routes still return 404.

## Growatt task 05 fields

Growatt identifiers remain full in authenticated records/status (`plant_id`,
`serial`, readable identity). Existing routes/query contracts are unchanged.
Known values remain exact decimal strings with nullable unit and explicit quality.
Task 05 emits `unresolved_time`, `missing_time`, or `contradictory_time`; use the
quality/provider-date view to browse these rows, whose measured_at is null.
No host-zone UTC or stable sample key is invented. Energy fields are counters.

Status attempt JSON adds `diagnostic` with optional field/value/expected/observed/
provider_code. These are structured validated context, never provider error text.
The status HTML displays available diagnostics. Durable audit/evidence routes and
refresh remain unimplemented until their assigned later tasks.

Discovery stream status additionally carries optional `failure` with class/code/
message/page/diagnostic; HTTP status is `diagnostic.http_status`. Safe failure
context remains available when discovery fails before identity selection.

Growatt HTML uses compact reading columns and an expandable Details view for all
exact values and original timestamp evidence. Visual ellipsis does not change API
precision. The table uses normal browser scrolling; query and record JSON contracts are unchanged.

## Daily Growatt aggregate queries (task05a)

GET `/api/v1/growatt/aggregates` and `/records/growatt/aggregates` require Basic Auth,
GET only, no-store. They read SQLite only. Optional filters: `plant_id` (full plant
identifier), `metric=generation`, `resolution=day`, `from` and `to` inclusive ISO
calendar dates. No conversion through the configured display timezone. `limit`
default100/max500 and opaque filter/page-size-bound `cursor`; order period then
local identity ascending. Duplicate/unknown fields, wrong resolution/metric,
malformed dates/cursors and changed cursor filters return the usual 400 envelope.
Empty matches return 200 with records[], never invented zero values.

JSON has version=v1, records and optional next_cursor. Each row contains source
(growatt), identity_id, full plant_id, metric, resolution, period, value (exact
nonnegative decimal string), unit=kWh, calendar=provider_date_zone_unverified,
quality (provider_reported or reported_zero_availability_unknown), period_start
and period_end (null; UTC boundaries unestablished), retrieved_at (UTC RFC3339),
and retrieval_id. Retrieval links retained selected evidence internally; task06a
owns authenticated raw-evidence inspection. Records are current accepted versions;
superseded values/evidence remain stored. Dates with no rows have unknown availability.

The compact local table offers plant/date filters, stable next-page links and
optional provenance expansion. It displays calendar dates, exact kWh and full
plant identity without a raw JSON wall, charts or custom scrolling controls.

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

### POST /api/v1/refresh

Basic Auth is required. GET `/api/v1/status` returns `X-CSRF-Token` and `csrf_token`;
the authenticated status page embeds the same process-secret HMAC token. Send it
as `X-CSRF-Token`. Missing/foreign token is403. When Origin is supplied it must
match scheme/host/port; null/foreign/multiple origins are403. Restart invalidates
old tokens; reload status. No query parameters are accepted.

Body is at most1024 bytes of one JSON object, with no unknown fields:
`{"source":"octopus","stream":"import"}` or
`{"source":"growatt","stream":"generation"}`. Optional stream selects all selected
configured streams for that source. Allowed streams are import/export and
telemetry/generation. Ranges belong to the enqueue CLI. Unknown selectors, malformed
JSON, additional JSON values and oversized bodies are400; wrong methods405.
Success202 returns `{"job_ids":[123]}`. Pending/running equivalents coalesce;
requests perform no cloud call. An unavailable selection/store returns503 without
inventing successful enqueue. GET status includes independent `schedules` with
readable owner identity, kind/policy, queued/running/paused/retry/limit state,
frontier, search_limit, next_due and current_job. Each Octopus source may include
historical_boundary state, oldest, observed, next_probe_due and provenance.
These fields describe fetched progress or observed availability, not fabricated
measurement completeness. Stored discovery failures retain class/code/context and
supported deferred-until, including across restart.

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

## Collection audit (task06a)

`GET /audit`, `/audit/{id}`, `/api/v1/audit`, `/api/v1/audit/{id}` and
`/api/v1/audit/{id}/evidence/{retrieval_id}` require Basic Auth and use no-store,
nosniff and the existing CSP. They read the already-open local store only; no
provider call, enqueue, recovery or migration occurs while browsing. Wrong methods
return 405, invalid filters 400, absent IDs or mismatched attempt/retrieval 404,
missing/wrong auth 401. Storage failures return a sanitized 503.

List parameters (HTML and API share the parser): `source` (octopus/growatt),
`stream` (import/export/device/generation), positive existing local `identity`,
`outcome` (running/success/empty/rejected/failed/cancelled/interrupted), `error_class`
(configuration/selection/authentication/rate_limit/network/timeout/pagination/protocol/
validation/timestamp_quality/storage), `from`, `to`, `limit`, `cursor`.
Unknown/repeated parameters fail. Empty optional form fields select all; empty
limit fails. Default limit 100, maximum 500. Unlike record queries, omitted range
selects all retained audit. Paired calendar dates are inclusive in the configured
zone; paired offset RFC3339 instants use exclusive `to`, with the same precision
and date-domain checks as record queries. Ranges apply to **attempt occurrence**,
never provider sample clocks or requested-window overlap.

Ordering is `(occurred_at DESC, id DESC)` with an opaque cursor bound to every
filter and limit. Keep filters unchanged for later pages. This is a live keyset
view; newly finished attempts can appear on a fresh query. Equal times retain the
stable ID tie-break, including repeated autumn hours.

List/detail envelope: `version`, `timezone`, `range_basis: attempt_occurrence`,
nullable UTC `from`/`to`, `limit`, `records`, `counts: {total, failed, rejected}`,
optional `next_cursor`. Counts cover **all filtered matches**, independent of
page/cursor. Detail uses the same envelope with one record, null range and zero
list counts/limit; no query parameters are accepted.

Each record has `id`, `job_id`, `identity_id`, `source`, `stream`, full `identity`
and dedicated `provider_identity` (mpan/serial/plant_id/device_type when retained),
`kind`, `requested_from`, exclusive `requested_to`, optional `provider_date`,
`started_at`, `occurred_at`, nullable `finished_at`, `occurred_local` (configured
zone with numeric offset), `outcome`, `outcome_label`, optional `error_class`,
`error_code`, `message`, `diagnostic`, nullable `page`, `retry_count`, nullable
`next_retry_at`, `retries`, `accepted_rows`, `evidence`, `detail_url`.
UTC instant strings end in Z. Aggregate requested bounds are bookkeeping, not
measurement instants. `diagnostic` may contain HTTP/provider codes, known field,
safe decimal value, expected/observed counts, provider period and normalized
`interval_start_utc_microseconds`/`interval_end_utc_microseconds`. Inline retries
retain `Page`, `Retry`, `HTTPStatus`, `Code`, `WaitMicroseconds`. Missing historical
context is left absent; no context is invented for older attempts.

Evidence references contain `retrieval_id`, `evidence_id`, `digest`,
`retrieved_at`, `complete`, `url`. Each URL checks both attempt and retrieval plus
identity ownership in a narrow joined query, even when content is deduplicated.
The response is retained selected telemetry JSON, bounded to 4 MiB before encoding,
with unknown telemetry and original timestamps preserved. `?download=1` selects
an attachment filename; no other evidence parameters are accepted. Unsafe older
content fails closed with 404. No headers, keys, authenticated URLs, account/address
material or exception/stack dumps are exposed. The UI fetches evidence only on an
explicit inspect click and inserts pretty JSON as text, never HTML.

`rate_limit` retains terminal `failed` and an explicit deferral label.
`local_budget` means the local aggregate reservation budget prevented this page's
request; `limited` means provider rate limiting. Counts therefore include deferred
failed attempts. Empty is accepted fetched unavailability; rejection/failure does
not advance completion. Interrupted/cancelled is never success.

Synthetic examples:

```text
/api/v1/audit?source=octopus&stream=import&from=2026-10-25&to=2026-10-25&limit=2
/audit?source=growatt&stream=generation&error_class=rate_limit
/api/v1/audit/123
/api/v1/audit/123/evidence/456?download=1
```

Status sources add `audit_counts` across all retained attempts and filtered links,
so repeated failures remain accessible after recovery. Aggregate responses add
`collection_progress` for matching plants, independent of row date filters:
`identity_id`, `identity`, plan `state`, `frontier`, `search_limit`, `next_due`,
`provenance`, `unfinished_jobs` (the exact automatic plan current job when unfinished: requested bounds, due, paused/running/retry count),
and nullable `last_attempt`. The generation page displays these stored values.
A frontier describes backwards fetched progress, not physical coverage; legacy
January rows can coexist with an unfinished gap before August. Inspect individual
attempt windows to distinguish not fetched yet, fetched empty and rejected data.

