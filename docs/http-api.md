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
