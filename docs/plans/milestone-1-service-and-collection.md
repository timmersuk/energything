# Milestone 1: running service and durable collection

Created 13 October 2026. Detailed implementation plan; no service is built yet.
Parent goal: [Energything stages](goal.md).
Execution briefs: [task map and shared contract](../tasks/milestone-1/README.md).
Updated 14 October 2026 with whole-window rejection and durable REST/UI audit decisions.

## Outcome and scope

Run one Docker container on the OMV host, open an authenticated UI and see
Octopus electricity import/export records and Growatt cloud telemetry arriving
and being retained locally. Restarting or replacing the container preserves
history and collection progress. A source outage is visible without preventing
browsing or collection from the other source.

Include electricity import/export discovery and collection, Growatt plant/device
discovery and read-only latest/history collection, durable storage, scheduled
refresh, bounded backfill, a simple status/table UI, authentication, Docker
deployment, durable collection audit through REST/UI, synthetic tests and CI/release builds.

Defer chart parity, gas, tariff/rate collection, money calculations, seasonal
aggregation, optimiser/scheduling suggestions, inverter writes and local Growatt
connectivity and Discord alert posting. Store useful inputs without implementing later features. Importing
historical investigation files is optional follow-up work; bootstrap from cloud
APIs first.

## Storage recommendation: SQLite first

Use `database/sql` with `modernc.org/sqlite`, subject to a small build/runtime
check on the deployment architecture. Pin a compatible maintained version when
implementing; old project toolchain versions do not establish compatibility.
The driver supports [pure-Go builds without cgo](https://pkg.go.dev/modernc.org/sqlite).
It fits a self-contained Go binary without a database container.

| Need | JSONL files | SQLite |
| --- | --- | --- |
| Append/manual inspection | Straightforward | Needs database inspection/export |
| Late readings/corrections | Needs versioning/replacement rules and replay | Unique keys and transactional updates |
| Progress saved with a batch | Separate writes need reconciliation | Data and progress commit together |
| Date queries/later aggregation | Scans or a maintained index | Indexed queries and SQL grouping |
| Runtime footprint | Local files, no database process | Local files, no database process |

SQLite introduces migrations and backup discipline, but avoids writing our own
correction, indexing and checkpoint machinery. This is the recommended trade for
historical records that can change. JSONL can be a future export format;
maintaining two primary stores is outside this milestone.

Put `energything.sqlite` and companion files under `/data`. Recommend WAL mode,
foreign keys, a bounded busy timeout and serialized writes with short transactions.
Validate per-connection configuration with the driver. Fetch cloud pages outside
transactions. Use an OMV-local filesystem bind mount, not SMB/NFS: WAL requires
suitable storage on the same host. See [SQLite's WAL constraints](https://www.sqlite.org/wal.html).

For milestone 1, document a clean stop before backing up the whole data directory
and verify restore. Copying only the main database during live writes is inadequate.
A later online backup can use a supported [SQLite backup mechanism](https://www.sqlite.org/backup.html).

If the build check fails, revisit driver/toolchain choice before collection work.
JSONL would require an explicit correction/index/checkpoint plan, rather than a
silent fallback.

## Application shape

Follow logthing's single-process/container deployment pattern. Start with Go's
html/template, plain CSS and modest JavaScript; use HTMX only where it simplifies
fragment updates. Embed templates and static assets directly with go:embed, with
no frontend compilation step. Avoid copying syslog-specific behaviour or unrelated
functionality:

```text
cmd/server/                 lifecycle and configuration
internal/octopus/           discovery and meter-reading HTTP module
internal/growatt/           cloud discovery and telemetry HTTP module
internal/collection/        scheduling, windows, retries and status
internal/storage/           SQLite schema, migrations, commits and queries
internal/web/               authenticated handlers and template rendering
internal/web/templates/     HTML pages and reusable fragments
internal/web/static/        embedded CSS and JavaScript
docs/plans/                 public plans
docs/                       operation and data semantics
```

Keep SQL and cloud pagination out of handlers. Accept context cancellation and
inject HTTP/clock dependencies where needed for meaningful tests. Small interfaces
should hide bounded-window fetching, batch/progress commits and time-range queries.
Do not introduce a universal source plugin system, ORM or hypothetical replaceable
storage layer. Tests should exercise module interfaces rather than mirror internals.

Keep data queries separate from template rendering. If later features become
difficult to implement or maintain, switch to React/Vite with logthing's embedded
frontend model as described in [the overall goal](goal.md#ui-fallback-for-later-milestones).
That decision adds a frontend build step but preserves collection and storage.

## Data and collection contracts

### Identity and records

Store privately discovered source identities and selected meters/devices in the
database. Meter serial and import/export direction are part of identity. Never
merge different meters or infer export provenance from an old CSV. Auto-select
only one eligible identity per stream; require explicit selection if ambiguous.
Missing direction flags mean unknown. Initially support one selected meter per
direction and one Growatt device; unsupported families show actionable status.

Recommended schema, refined with synthetic examples before coding:

- Source identities: provider, stable source identity, stream/direction and only
  necessary discovery metadata. No credentials or full account/address responses.
- Accepted electricity intervals: identity, start/end instants, decimal kWh and
  retrieval/provenance. Rejected observations remain in evidence/audit, separate
  from accepted history. Key by identity/start; separately validate
  end/duration. Preserve decimal precision for future financial calculations.
- Growatt samples: device, source timestamp/key, nullable established normalized instant,
  timestamp quality, selected fields with units/quality and retrieval time.
  Missing values stay null rather than zero.
- Payload evidence: selected telemetry payloads associated with records/completed
  pages, sufficient for field/timestamp reprocessing. Exclude credentials,
  headers and full discovery responses. Initially retain all selected telemetry
  payload evidence, including superseded
  versions, without duplicating identical payloads on repeated collection.
  Measure storage growth before introducing retention limits or lossy compaction;
  normalized history remains durable.
- Collection state: job/window/checkpoint, attempt/success times, newest measurement,
  next due time, sanitized failure classification and coverage. Index identity/time;
  version schema migrations.
- Attempt audit: timestamp, source/stream, requested bounds, outcome, sanitized
  error class/reason, retry/page context and evidence references. Retain initially
  without pruning; expose bounded authenticated REST/UI queries and repeat counts.

Transactional upserts support corrections and provenance. Repeating a window/page
must not duplicate data. A later validated window may correct an earlier valid
observation while retaining prior evidence. Invalid or contradictory readings
reject the entire bounded window; preserve all accepted data and completion
checkpoints. Identical readings may be deduplicated, but arrival order never
resolves conflicting values. All pages must arrive and the assembled data must
pass validation before observations and progress commit atomically. Failure or
rejection persists suspect evidence and attempt audit separately, without marking
completion or erasing old data. Retry scheduling may change; completion may not.

### Octopus

Selectively reuse reviewed octopull logic for destination validation, pagination,
retry, export provenance, disjoint windows and completeness. Extend it deliberately
to confirmed import meters. Omit legacy private defaults, fixtures and scripts.

Keep serial/direction explicit. Start with consumption requests bounded to 14 days,
reject foreign/cyclic pagination and credential-carrying redirects. Validate aligned,
non-overlapping intervals. Invalid/negative/conflicting readings reject the whole
window, with evidence and the reason visible in audit; preserve previously accepted
data, genuine zeros and missing expected-slot distinctions.

Use UTC request bounds and half-open application windows. Europe/London days can
have 46 or 50 half-hour intervals; never assume 48 for completeness. Older gaps
may reflect meter lifetime, latency or missing records; do not invent a cause or
refetch all history indefinitely.

### Growatt

Start with token-authenticated cloud collection for SPH/MIX. Discover plants/devices,
validate device type, and include only discovery/latest/history reads. Some reads
use POST: read-only depends on endpoint behaviour, not HTTP method. No settings
write endpoints are included.

Collect SOC/BMS SOC, solar power, household load, grid import/export power, battery
charge/discharge power and energy counters where present. Establish units/signs
before normalization. Retain unknown fields in selected payload evidence without
inventing values. Power samples are not billing intervals; daily counters can reset.

Fetch day-sized paginated history with bounded backoff; start with sequential pages.
Add bounded concurrency only after paging semantics are established and tested. Retention is
limited and can change. Empty history is empty/unavailable coverage, never zero
energy. Keep locally retained observations when cloud history expires.

Timestamp interpretation is an implementation gate: readable local time, timezone
metadata and embedded calendar/epochs may disagree. Confirm semantics before
assigning UTC. Keep original fields; ambiguous samples remain browsable with a
quality flag and are excluded from cross-source aggregation. Preserve repeated
local times at the autumn clock change using provider identity where available;
never silently deduplicate by local clock text alone. If identity/time cannot be
resolved, keep payload evidence for reprocessing instead of pretending it has a
unique UTC instant.

### Scheduling defaults to validate

These are starting defaults, not guaranteed provider limits:

| Work | Behaviour |
| --- | --- |
| Startup | Open/migrate storage, serve UI, discover sources and enqueue asynchronous bounded backfill |
| Octopus initial history | Recent data first, then automatic resumable available history (task 06 boundary gate) |
| Growatt initial history | Recent data first, then automatic verified available history (task 06 boundary gate) |
| Octopus refresh | Hourly; revisit recent 7 days for late/corrected readings |
| Growatt latest | Every 5 minutes |
| Growatt history refresh | Hourly; current and previous 2 days to repair missed latest polls |
| Older backfill | Small resumable windows, yielding to routine refresh; provider boundary/search progress persisted |

No overlapping executions of the same job. Sources run independently, HTTP calls
have timeouts and retries use bounded backoff/jitter and supported Retry-After.
Persist due work/checkpoints so restart resumes without refetching the archive.
Wider historical correction uses an explicit durable repair CLI action; recent
overlap does not guarantee detecting all old changes. On 14 October 2026 the user
rejected one-shot/backfill-start environment controls: normal startup must collect
and bootstrap history automatically from credentials and confirmed selections.
The revised task 06 owns verified historical boundaries, finite search budgets,
resumption and removal of temporary/obsolete controls. No operator date or
initialization toggle is required; authenticated refresh-now remains, with complex
job/selection UI deferred.

## Minimal UI and HTTP behaviour

Serve template-rendered pages, HTML fragments, static assets and versioned HTTP
endpoints from the same binary. Protect them with
configured credentials and require a nonempty password; ship no defaults.
An unauthenticated liveness endpoint reveals no source details. Basic Auth requires
trusted LAN access or HTTPS through the host's reverse proxy.

Each source card shows full provider identity, configuration/discovery state,
running/backfill state, last attempt/success, newest measurement, record
count/coverage and sanitized errors. Retrieval and measurement times are distinct:
polling old cloud data must not make it look newly measured.

Import, export and Growatt tables have date/range selection, bounded pagination,
units and quality indicators. Read local storage; UI refresh does not call clouds.
Latest known SOC/power can appear as a summary. No charts or savings required.
Empty/loading/partial states explain ongoing collection. Manual refresh returns
job status without blocking through a backfill; prevent cross-site refresh requests.

The authenticated audit page and REST API expose durable attempt history, including
timestamp, source/window, sanitized failure reason and repeated-failure counts.
Rejected windows do not alter accepted record tables. Audit remains inspectable
after restart and source recovery. Discord alerts are deferred.

## Configuration and deployment

Use environment configuration and a placeholder-only `.env.example`. Proposed
names: `ENERGYTHING_HTTP_ADDR`, `ENERGYTHING_DATA_DIR`, `ENERGYTHING_USERNAME`,
`ENERGYTHING_PASSWORD`, `OCTOPUS_API_KEY`, `OCTOPUS_ACCOUNT_NUMBER`,
`GROWATT_API_TOKEN`. Also document explicit meter/device selection, timezone,
backfill start and refresh intervals. Finalize names in the scaffold.

Require a configured source for collection; UI and a healthy source can run while
another is unconfigured/unavailable. Invalid global configuration or an unusable
database fails startup clearly. No secret values, authenticated URLs, request
headers or private response bodies in logs.

Build directly with Go, embedding templates, CSS and JavaScript without a frontend build.
Vendor and pin browser libraries locally rather than relying on a runtime CDN. Provide local/container build commands, a non-root Docker image, one HTTP port and a writable `/data` bind mount. Document
OMV UID/permissions, one application instance per database, restart policy and
graceful shutdown. Include UK timezone data and TLS trust roots. Runtime needs no
Node process, database server or separate cron/worker container.

CI builds Go with embedded UI assets, runs synthetic tests, builds images and scans
for secrets. No frontend package installation or compilation is required initially.
PRs build without publishing; versioned release images use Docker Hub following logthing, with narrow workflow
permissions and no publication from PRs. Follow logthing's Linux amd64/arm64
release pattern where practical; validate the actual OMV target. Document pinned
image updates through Compose. Host rollout is explicit: public workflows do not
receive household runtime credentials or automatic access to private data.

## Implementation sequence

Each step leaves a reviewable running result. The following are milestone slices;
the [nine execution briefs](../tasks/milestone-1/README.md) refine them, separating
stored browsing, REST/UI audit and private host acceptance. Execute from those
briefs and their shared contract rather than this sequence alone.

1. **Scaffold/build baseline.** Create the Go module, templates/CSS/JavaScript UI,
   auth/liveness/shutdown, Docker/Compose and build docs. Add Git/Docker exclusions
   for secrets/data/exports/caches/private files. Validate compatible SQLite/Linux
   builds. Result: runnable empty application.
2. **Store and browsing.** Implement migrations, atomic batch/progress writes,
   indexed queries and synthetic tables. Check corrections, replay, nulls and
   persistence across reopen. Result: synthetic history survives restart.
3. **Octopus end-to-end.** Add configured discovery/selection, import/export
   windows, validated pagination, completeness and UI status. Result: mocked tests
   and private read-only validation show both streams stored and browsable.
4. **Growatt end-to-end.** Add discovery/latest/history, unit/timestamp semantics,
   payload evidence and UI. Result: latest and historical samples retained;
   uncertain timestamps visibly flagged rather than guessed.
5. **Continuous collection/recovery and audit.** Add schedules, backfill checkpoints, overlap
   refresh, independent failures, serialized writes and manual refresh. Result:
   restart/outage recovery without database repair or full archive refetch. Add
   bounded authenticated REST/UI audit queries, failure reasons and repeat counts.
6. **Release/deployment readiness.** Complete CI/images, storage permissions,
   backup/restore docs and privacy review; privately smoke-test on OMV. Result:
   both sources collect in Docker; history survives replacement and restore.

## Verification and completion

- Synthetic HTTP tests: selection ambiguity/direction, pagination, foreign hosts,
  retries/partial failures and unavailable Growatt history.
- SQLite integration tests: atomic checkpoints, replay, corrections, preserving
  earlier valid data on failure, migrations and indexed date queries.
- Time/quality tests: both UK DST changes, inconsistent Growatt timestamp fields,
  zero/null distinctions and partial current periods.
- Worker tests: cancellation, resumed backfill, non-overlap and independent failures.
- Audit tests: timestamp/source/reason, rejection preservation, repeated-failure
  filters/counts, restart retention and secret redaction through REST/UI.
- Browser smoke checks: auth, status, pagination/date filters, empty/partial/error
  states and browsing through a provider outage.
- Container checks: non-root storage permissions, replacement persistence,
  timezone/TLS support and backup/restore.
- Private live check: actual discovery/ingestion from both providers, without
  inverter writes. Evidence stays local; no real fixtures, credentials,
  household screenshots/results in public source or CI.

Complete when OMV's authenticated UI shows retained records from both providers,
automatic refresh continues, durable REST/UI audit is inspectable,
recovery/persistence checks pass and operation is documented. A synthetic demo/local build alone does not complete the milestone.
No application tests are needed for this planning-only change.

## Bounded decisions during implementation

Inherit logthing Linux amd64/arm64 and Docker Hub conventions; verify actual OMV
runtime/local storage during deployment, pin driver/toolchain in step 1,
validate Growatt timestamp/units in step 4 and tune schedules from private source
behaviour. These checks do not expand scope into gas/pricing, chart parity,
control investigation or bulk migration of private research.

## Historical baseline amendment (15 October 2026)

The user agreed to retain Growatt's older provider generation aggregates alongside
detailed locally archived telemetry, to make historical annual-pattern inputs
available without waiting for a year of new collection. [Task05a](../tasks/milestone-1/05a-growatt-aggregates.md)
now owns a separate exact period/metric table, bounded aggregate ingestion and
simple authenticated browsing before task06 schedules it. This is collection of
provider aggregates, not the deferred calculation of seasonal analytics/charts or
integration of instantaneous power. Existing detailed/Octopus data remains separate;
overlapping resolutions are not added together. Implementation has not started.
