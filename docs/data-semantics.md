# Durable storage contract (task 02)

All absolute instants are signed integer UTC Unix microseconds. Go callers use
`Instant`/`At(time.Time)`; source clock text is independent. Windows are half-open.
Requested bounds are restricted to years 0001–9999 to prevent arithmetic overflow.
Electricity is nonnegative exact decimal TEXT (no exponent, float or rounding).
Octopus slots are aligned 30-minute intervals, bounded to 14 absolute days.

Synthetic examples established before the migration:

- Import identity `synthetic-mpan/synthetic-meter/import`, slot 00:00–00:30,
  `0.100`: replay keeps one current interval and one evidence blob, with separate
  retrieval/attempt links. A later accepted `0.120` replaces the current value;
  the earlier observation version and evidence remain.
- Two slots submitted with one invalid decimal or conflicting duplicate: the
  entire commit fails validation. The caller records a rejected attempt with
  suspect sanitized evidence; current observations and checkpoint do not change.
- Fully fetched empty Growatt history: terminal `empty`, fetched window recorded,
  zero measurement coverage, earlier local samples remain. Empty never means zero.
- Two `2026-10-25 01:30` samples without established key/time: each is retained
  under attempt plus observation ordinal, with null normalized time and key.
  A replay deduplicates evidence bytes but retains separate unresolved occurrences.
  Task 05 establishes resolution and reconciliation; task 02 guesses neither.
- Missing SOC is JSON null; explicit SOC `0` is a decimal string with its unit.
  An unestablished unit also remains JSON null.
  Unknown selected fields and original timestamp fields survive in telemetry
  evidence and original-time JSON. Unresolved device discovery evidence must be
  held by the future provider gate until a source identity is established.

## Logical schema

Source identity is unique by provider/stream/stable key. Octopus stable key must
encode MPAN and serial; Growatt must encode established plant/device identity.
Storage treats that key as opaque. Deselection never removes history. Necessary
discovery metadata is restricted to named meter/plant/device fields.

Jobs have a stable identity/kind/policy key, bounded current window, provider date,
optional range-request ID, due time, running attempt, retry count, pause flag and
nullable completed checkpoint. Different refresh/backfill policies have different
jobs. A range request ID binds an immutable ordered list of source/kind/bounds;
reuse with different work fails. No timer or provider request exists here.
Kinds are `history`/`latest`; policies are `routine`/`backfill`/`range`. Each bounded
historical window is immutable work; the later scheduler ensures the next unit
between fetches, rather than mutating a running window. Completed backfill/range
units are excluded from due work. Explicit acceptance replays are still supported.

Attempts have immutable source/job/window context, start/occurrence/finish,
terminal outcome, structured sanitized error and bounded retry/page details.
Evidence blobs are SHA-256 deduplicated canonical selected JSON. Each retrieval
links blob, source, attempt, retrieval time and completeness independently.
Current observations point to a retrieval; append-only versions retain all prior
values and retrievals. Unresolved Growatt rows use attempt/ordinal, never clock
text. Resolved keys are supplied only by the future protocol gate.

Window completions record latest accepted attempt, fetched state, expected/valid/
missing/quality slot counts and measurement-complete flag separately. Missing
slots do not delete old rows. Electricity expected slots are computed from elapsed
window bounds at attempt finish; selected samples do not imply full Growatt
measurement coverage. Completion checkpoint means fetched work, not full readings.

## Typed methods and guarantees

`Open(ctx, dir) (*Store, error)` opens, atomically migrates and reconciles old
running attempts; `Close()` closes last at shutdown. Database handles and SQL
remain private. Writes use a single connection and short transactions. Every
method takes context. Transactions are rolled back on cancellation or error.

- `UpsertIdentities(ctx, []Identity) ([]Identity, error)` retains historical sources.
- `EnsureJobs(ctx, []Job) ([]Job, error)` creates/coalesces immutable work;
  `EnsureRange(ctx, requestID, []Job)` atomically registers a configuration range.
- `DueJobs(ctx, now, limit)` returns eligible selected, unpaused, nonrunning work.
- `ResumeJob(ctx, jobID, nextDue)` explicitly unpauses unfinished work after a
  future configuration/discovery gate resolves an error; no checkpoint changes.
- `BeginAttempt(ctx, AttemptRequest) (int64, error)` atomically claims a job.
- `CommitAcceptedWindow(ctx, AcceptedWindow) error` validates the whole unit then
  atomically writes observations, versions, evidence/retrievals, window completion,
  checkpoint, terminal success/empty and next due. No absence deletes history.
- `RecordRejectedOrFailedAttempt(ctx, FailedAttempt) error` writes terminal audit,
  suspect/incomplete evidence and retry/pause state only; no accepted progress.
- `RecoverInterruptedAttempts(ctx, now) (int64, error)` marks running attempts
  interrupted and makes the same unfinished jobs due, preserving checkpoints.
- `QueryIntervals`, `QuerySamples`, `QueryAudit` return bounded typed pages;
  `QueryAudit` also returns matching total/failed/rejected counts and evidence links.
  `QueryAttempt(ctx, id)` retrieves one sanitized audit with evidence references
  or `ErrNotFound`; no database or SQL handle is exposed.
  `QueryStatus` returns aggregate counts, last retrieval/measurement, attempt state,
  jobs and explicit fetched/measurement window coverage.

Example: ensure an Octopus `history` job with start/end in microseconds; begin it
with `Started=At(now)`; accept `Intervals=[{Start,End,KWh:"0.100",Evidence:0}]`
and `Evidence=[{Content: selectedJSON,Retrieved:At(now),Complete:true}]`.
An evidence index is local to this commit, not a SQL ID. Query results expose
decimal strings, nullable sample instants/values and opaque next cursors.
Even empty accepted responses require complete selected evidence. Successful
attempts may carry bounded retry details and a timestamp-quality notice while
retaining their success outcome. Evidence canonicalization/digesting happens
before the write transaction; cloud work never happens inside it.

Query limits default to 100 and cap at 500. Ascending records use instant/identity
or instant/row-ID; descending audit uses occurrence/ID. Opaque versioned cursors
bind all filters and page size; malformed or mismatched cursors fail validation.
Sample absolute ranges omit null instants and report their count; quality queries
use provider date or retrieval range. Source IDs in results are local IDs; audit
display identity contains useful full provider identifiers. Opaque stable keys
are not display labels. Selected evidence inspection belongs to authenticated task
06a detail routes, not public artifacts or routine status responses.
Status includes latest attempt and latest accepted attempt separately. It returns
at most 500 recent jobs/windows per identity plus total counts; accepted coverage
is always coverage of the latest fetched response, not old retained rows.

Task 03 adds `RecordQuery.Direction` for typed Octopus `import`/`export` filtering
(empty selects both). It is included in cursor binding; `QuerySamples` rejects a
nonempty direction. HTTP contracts are in [HTTP API](http-api.md). Task 03 also
fixes independent last-attempt/last-accepted status values: reading a last accepted
attempt must not overwrite a later failed/running attempt summary.

Evidence inputs must already be selected telemetry, not HTTP transcripts or
discovery responses. Storage additionally rejects credential/header/address keys
recursively; unknown telemetry fields are retained. Arbitrary raw error strings
are not accepted: public messages derive from enumerated error classes/codes.
Retry codes are enumerated and carry no URL/body. This avoids attempting to redact
unknown secrets from free text. Selection, unit and timestamp protocol validation
belong to tasks 04/05; no consumer may bypass these transactions with ad hoc SQL.

## Octopus protocol and one-shot handoff (task 04)

Verified on 14 October 2026 against the official [Octopus REST endpoint guide](https://docs.octopus.energy/rest/guides/endpoints/).
The account electricity points include nullable `is_export`: only explicit false
confirms import and true confirms export. Account/address/gas/tariff material is
never persisted. Account responses remain ephemeral; full point descriptions are
canonicalized in memory solely to reject contradictory repeated descriptions.
Identical identities are deduplicated. Automatic selection requires exactly one
confirmed eligible meter per direction. Explicit selection must match both MPAN
and serial from discovery. Unpaired, mismatched or ambiguous selections affect
only that stream; conflicts block that stream. Unknown direction is never selected.

`RecordOctopusDiscovery` atomically stores two enumerated stream states and necessary
identity metadata, deselecting stale Octopus identities without deleting history.
`QueryDiscovery` returns stream/state/occurrence only. Migration 002 adds this narrow
state table to the existing versioned schema. Fetch attempts retain the task 02
BeginAttempt/CommitAcceptedWindow/RecordRejectedOrFailedAttempt guarantees.
No raw SQL handle is exposed. Storage failures return sanitary storage errors;
failed failure-audit writes leave no claimed durable terminal result.

`octopus.Client.Fetch(ctx, confirmedMeter, from, to)` returns a whole-window result
and an enumerated error. Callers may commit intervals only on nil error. All pages
are assembled before validation. Selected evidence and retries remain available
on failure/rejection. Evidence retains unknown telemetry fields and original
source timestamps, excluding transcript/credential/account/address-shaped fields
recursively, pagination URLs and nontelemetry envelopes. Malformed/truncated JSON
has no reproducible selected telemetry; previously obtained page evidence remains.
Boundary exclusion/duplicate counts and requested bounds are recorded in evidence.

Consumption for import and export uses the same electricity consumption endpoint.
The official guide documents UTC query bounds, overlapping boundary behavior,
half-hour intervals and export energy reported as consumption. Application windows
are `[from,to)`, at most 14 absolute days. Query `period_to=to-1s` avoids the end
neighbor; wholly outside adjacent aligned neighbors may be excluded and recorded.
A straddling interval, nonaligned timestamp, invalid duration, overlap, negative/
unparseable/nonfinite decimal or conflicting duplicate rejects the entire unit.
Repeated equal numeric values collapse even with different decimal spelling.
JSON numeric exponents expand exactly to storage decimal TEXT, limited to 128
characters; no float or rounding. Future/unelapsed in-window readings reject.
Missing readings are accepted as missing coverage; zero remains valid energy.

Every authenticated request requires HTTPS, exact `api.octopus.energy` host
(without port/userinfo/fragment/escaped path), the exact intended account or meter
endpoint, and the original bounded query. Only a single bounded `page` parameter
may change. Redirects are disabled. Injectable transports cannot alter this policy.
Timeout: 30s per request, response limit: 4 MiB/page, consumption page size: 100,
maximum: 16 pages and 2,048 assembled records including duplicates/neighbors.
These narrowed engineering limits accommodate the 672 possible half-hours plus
conservative duplication; they are not asserted provider limits. Required stable
count, results array and next field distinguish empty from incomplete; count
mismatch, canonical URL cycles, canonical repeated nonempty result sets and excess budgets fail.

HTTP retries are owned solely by the Octopus client: at most three requests/page
for network errors, 408/429/5xx. Ordinary waits are 1s then 2s plus 0–250ms jitter,
cancellable; Retry-After seconds/HTTP-date override them. Beyond 30s returns a
deferred instant without waiting. JSON/validation/auth/destination failures have
no inner retries. Retry details retain page/request/status/code/planned delay,
never URL/body/key. An exhausted final request records its planned delay but does
not wait. Collection writes deferred next_due (otherwise now+1m on fetch failure)
and pauses authentication/configuration failures; no scheduler executes reattempts.
Task 06 must not multiply this inner retry budget.

Task06 supersedes the task04 startup one-shot controls with automatic collection.
ONCE, BACKFILL_START and startup range settings are ignored and removed from the
supported configuration. Use [the durable enqueue CLI](collection.md#refresh-and-exceptional-repair)
for explicit repair, and credentials/selections alone for ordinary collection.

## User-approved Octopus identity visibility

The user explicitly rejected hiding MPANs because it prevented comparison with the
Octopus website. Authenticated Octopus records/status expose full MPAN and serial
from necessary discovery metadata. QueryStatus returns that metadata internally
(without serializing it wholesale); web responses explicitly select Octopus fields.
Readable direction/MPAN/serial labels replace local masked placeholders. No stable
key, credentials, account/address payload or evidence is exposed. No schema change
or cloud request is needed; legacy missing metadata remains explicitly unavailable.

Owner-visible diagnostics use full necessary provider identifiers. Selected telemetry
is retained for authenticated audit inspection in task 06a, not prohibited from the
owner's application. Public-repository/session privacy applies to where real evidence
is published, not to hiding useful operational data behind opaque local IDs.
New Octopus validation failures have enumerated decimal/timestamp/duration/alignment/
boundary/future/conflict reasons; count and missing-results reasons are distinct.
Storage derives human-readable messages from these safe codes; historical class-only
messages are not retroactively fabricated. Broad errors cannot satisfy task 06a's
specific-reason/context requirement for new attempts.

## Growatt bounded collection (task 05)

See [the protocol gate](growatt-protocol.md) for the exact allowlist, field matrix,
time qualifications and acquisition budgets. The explicit `growatt-verify` CLI
performs discovery and optionally latest plus one provider-day collection. It
requires an opted-in private report path; no Growatt ONCE or backfill-start control
is introduced. Automatic collection and available-history recovery remain task 06.

`RecordGrowattDiscovery` atomically updates the telemetry discovery state and
necessary identities, deselecting stale Growatt identities without deleting data.
Migration 003 adds Growatt discovery and an additive typed attempt diagnostic JSON
column. Existing Octopus discovery, attempts, observations and checkpoints survive.
`CollectGrowattUnit` uses BeginAttempt/CommitAcceptedWindow/RecordRejectedOrFailedAttempt.
All pages and known fields validate before acceptance; failed audits use a separate
five-second cancellation-independent cleanup context. A failed audit write never
claims durable completion. Authentication/configuration pauses; other errors set
a future due instant (client deferral or one minute). No worker executes it yet.

Diagnostic is typed: optional enumerated telemetry `field`, exact numeric `value`,
integer `expected`, `observed`, `provider_code`. Storage rejects arbitrary field
names and nondecimal values; no unchecked provider error text is accepted. Page,
requested job/day/bounds and evidence references remain separate. Task 06a can query
this retained context without reconstructing it from raw exception strings.

Growatt `Expected=0` means no established expected cadence, not zero production.
`Valid` counts accepted occurrences and `Qualified` counts qualified occurrences.
Storage allows these observed counts without manufacturing expected coverage;
when Expected is explicitly positive the existing arithmetic validation remains.
MeasurementComplete stays false for this adapter. Empty acceptance preserves older
rows and records observed unavailability. Unresolved occurrences retain attempt/
ordinal and null key/instant; latest/history replay cannot claim idempotent sample
identity until independent provider evidence establishes it. Evidence is digest
reused but every retrieval and unresolved occurrence remains attributable.

Review follow-up adds migration 004 for safe discovery failure class/code/page and
Diagnostic. QueryDiscovery/status expose this stored context even when no identity
could be selected. Diagnostic includes `http_status`; nonnegative count diagnostics
can represent rejected overruns beyond the acceptance budget. Such counts never
increase the permitted fetch/commit budget.

## Task05a daily aggregate contract (15 October 2026)

Protocol gate: the saved official plant/energy page additionally limits this feed
 to **10 calls/day**, alongside one call per five minutes. A fresh authorized
January 12–18 query returned seven daily rows, including both endpoints, outside
 the observed detailed-telemetry window. Evidence is private under .cache.
The feed reports plant generation in kWh, not inverter/grid/load energy.
Provider calendar date is retained verbatim. No timezone is supplied by this
endpoint: calendar provenance is `provider_date_zone_unverified`, with no UTC
measurement boundaries. A zero is `reported_zero_availability_unknown`; it is
never evidence of operation, coverage or an oldest available date. Absent rows
remain absent; null/placeholder/invalid energy rejects the whole unit.

The acquisition unit is **one through seven inclusive calendar dates**, perpage=100. A
complete stable count is required, maximum 100 rows/page, 100 pages/10,000 rows;
all rows must fall within the requested inclusive date range. Identical numeric duplicates collapse;
conflicting duplicates reject. Seven-date inclusivity was privately verified;
month/year are deferred. Complete pagination is validated
synthetically; large real pagination is not established for daily units.
Aggregate requests have **one HTTP transmission per page**, no inline retries.
A durable local ledger enforces five-minute spacing and ten calls per rolling 24h; a page requiring a later slot defers the whole unit. Errors defer to a later explicit
invocation/task06; no caller retry loop. Task06 must use this ledger across restarts and coordinate calls from other clients. The explicit CLI permits one bounded
unit per invocation; operator must account for calls by other clients.

Storage identity is provider growatt, stream generation, stable plant ID alone,
metadata PlantID only. Job kind `aggregate`, existing backfill policy and UTC date
bookkeeping bounds (not measurement instants). Separate energy_aggregates current
rows and append-only aggregate_versions key identity/metric/resolution/date.
Exact nonnegative decimal TEXT, kWh, day, provider date, calendar provenance,
quality, retrieval/evidence and attempt associations are retained. Corrections
update current and preserve versions. Replays preserve one current record and
record distinct retrievals. Acceptance atomically writes aggregate rows, evidence,
versions, fetched completion, checkpoint and terminal audit; failure changes only
audit/retry state. Fetched completion never asserts measurement completeness.

Query: GET /api/v1/growatt/aggregates and /records/growatt/aggregates, authenticated,
no-store, local only. Optional plant_id, metric=generation, resolution=day,
from/to inclusive YYYY-MM-DD, limit default100/max500, cursor. Ascending date then
identity; cursor binds all filters/page size. Unknown/duplicate arguments reject.
Expose full plant ID, exact decimal string, kWh, period, calendar provenance,
quality, retrieved UTC and retrieval ID. Never combine aggregate and sample feeds.

`ValidateAggregateJob` binds supplied plant/identity/kind/date/bounds to persisted
work before aggregate quota reservation or transmission, including empty response
units. A mismatch records `configuration/aggregate_request`, never fetched progress.

Seven-date amendment: normal aggregate acquisition units span up to seven inclusive
provider dates, while record identity remains daily. ProviderDate is the range's
first date, End is the exclusive UTC bookkeeping midnight after its final date.
All dates validate/commit atomically, absent dates remain absent, and existing
one-day units/rows remain valid without a migration. No additional calls per day
or inline retry ownership changes.

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

Task06a adds authenticated read-only audit browsing and selected evidence retrieval.
Attempts, accepted observations, retrievals and deduplicated evidence remain distinct;
a retrieval is authorized through its owning attempt and source identity. New typed
diagnostics may retain normalized interval endpoints and provider calendar period;
old missing context is not reconstructed. No migration or pruning accompanies these
read APIs. Local aggregate quota deferrals use class rate_limit/code local_budget;
provider deferrals retain limited. Both retain failed terminal outcome with an
explicit deferral label, rather than implying empty or successful collection.
