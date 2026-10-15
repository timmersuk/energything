# Energything

Task 01 supplies an authenticated empty application. Task 02 adds durable SQLite
observations, evidence, jobs and audit through typed storage methods. Task 03 adds
authenticated stored-record tables and REST queries. Task 04 adds confirmed
Octopus discovery and an opt-in bounded one-shot ingestion
worker. Recurring scheduling and Growatt collection are not implemented.

Requires Go 1.26.4 (module compatibility 1.26.0). Dependencies are pinned in
go.mod/go.sum: modernc.org/sqlite v1.60.1 and modernc.org/libc v1.77.1.
No Node, frontend build, cgo or database container is required.

Build and test:

```text
go test ./...
go build ./cmd/server
make test
make build
```

In PowerShell, set ENERGYTHING_USERNAME and ENERGYTHING_PASSWORD to your chosen
nonempty credentials, then run `go run ./cmd/server`. The application reads only
the process environment; it does not load any .env file. Local defaults are
:8080, ./energything-data, Europe/London and a 10s shutdown timeout.
See [.env.example](.env.example) for every supported environment name and
the [configuration contract](docs/tasks/milestone-1/execution-contract.md#configuration-contract)
for their meaning. Provider settings are parsed independently; invalid or partial
settings produce an authenticated error card without blocking the other source.
Valid credentials produce a configured card. With ENERGYTHING_OCTOPUS_ONCE=true,
startup discovers Octopus once and fetches one bounded window independently for
each selected direction. Browsing never calls providers. The option defaults false.

GET /healthz returns public fixed process liveness. GET / and
GET /static/style.css, /records/octopus, /records/growatt, /api/v1/status,
/api/v1/octopus/intervals and /api/v1/growatt/samples require Basic Auth.
See [HTTP API](docs/http-api.md) for bounded queries and exact JSON fields.
Other paths return 404; wrong methods on implemented routes return 405.
Refresh and audit routes are not implemented.
Use trusted LAN access or an HTTPS reverse proxy for Basic Auth.

Docker:

```text
make docker-build
docker compose up -d
```

`make build` and `make build-go-local` create both
bin/linux/{amd64,arm64}/energything with CGO_ENABLED=0. Build IDs use
`make build CI_BUILD_ID=<public-id>`. `make docker-build` builds these binaries
before building energything:local. Plain `docker build -t energything:local .`
requires `make build` first. On hosts without Make use
`go run ./scripts/build.go`, then the same Docker build command.
Compose uses the built image and requires ENERGYTHING_USERNAME/PASSWORD.
Compose may read its own project .env; use .env.example as a placeholder guide.
ENERGYTHING_IMAGE, ENERGYTHING_HTTP_PORT and ENERGYTHING_DATA_PATH select the
image, host port and bind mount. Never use sibling .env files.

The image is Alpine 3.23, includes CA roots and UK timezone support,
runs as UID 10001/GID 100, exposes only 8080 and stores energything.sqlite under
/data. See [operations](docs/operations.md) for mount permissions and
[task 01 verification](docs/tasks/milestone-1/01-verification.md) for executed
checks and remaining gates. The SQLite runtime gate passed on both architectures.
Storage opens with transactional versioned migrations and interrupted-attempt
recovery. See [data semantics](docs/data-semantics.md) and
[task 02 verification](docs/tasks/milestone-1/02-verification.md) for storage
contracts, synthetic checks and the unresolved Growatt key/time gate.
See [task 03 verification](docs/tasks/milestone-1/03-verification.md) for
isolated synthetic browser acceptance and remaining gates.

Task 04 one-shot verification: set `ENERGYTHING_OCTOPUS_ONCE=true` in the
energything process environment (or Compose substitution), with Octopus key/account.
The worker starts alongside HTTP serving; shutdown cancels fetches and waits for
bounded audit cleanup before closing storage. It never repeats on a timer.
Default bounds are previous local midnight through the latest elapsed half hour.
With `OCTOPUS_BACKFILL_START`, only the first at-most-14-absolute-day unit is fetched;
a later scheduler owns progression. Range-refresh configuration is not executed here.
Restarting with the explicit option repeats this window and can accept corrections.
Both directions need confirmed discovery; ambiguous/mismatched/unpaired selection
is isolated to its stream. Configure paired MPAN/serial to resolve ambiguity.
Discovery states appear in status without exposing raw identifiers.
See [task 04 verification](docs/tasks/milestone-1/04-verification.md) for synthetic,
actual-build browser and private live gates and the single HTTP retry policy.

## Local preview standard

After task verification, rebuild and recreate the existing local preview on its
original mount so localhost:8080 serves the current implementation. Follow the
[standard preview update procedure](docs/operations.md#standard-local-preview-update),
including the saved Compose override, safe metadata checks and build verification.

## One-off meter discovery report

Use the read-only CLI when discovery finds several confirmed meters and you want
to choose without adding a selection screen or REST endpoint:

```powershell
# Requires OCTOPUS_API_KEY and OCTOPUS_ACCOUNT_NUMBER in the process environment.
go run ./cmd/octopus-discover
# Optional: sample fewer days and choose a NEW private output file.
go run ./cmd/octopus-discover --days 7 --timeout 2m --out .cache/octopus-discovery/my-report.md
```

No .env file is loaded. Default output is a new private Markdown file under ignored
`.cache/octopus-discovery`; stdout contains only its path and completion status.
The report lists confirmed import/export MPAN/serial choices, valid versus expected
slots, newest/oldest validated interval times and sanitized sample failures. It
contains private identifiers, so keep it outside tracked/public files.

Sampling uses the existing safe client sequentially over the last 14 absolute days
(default), ending at the latest elapsed half-hour. Limits: 1–14 days, at most 20
candidates, total deadline 2m (maximum configurable 10m), existing page/body/retry
budgets. Unknown direction and conflicting discovery are never sampled.
Authentication/rate-limit failures stop further sampling to avoid repeated calls.

Candidates are ranked by newest validated interval end, then observed slot count.
Zero-energy readings count as reporting. If exactly one of several fully sampled
candidates has readings ending in the last 48 hours, the report provides a tentative
steer and candidate environment settings for your confirmation. Several recently
reporting candidates, failed/incomplete sampling or no recent observations leave
the selection unresolved. Sampling cannot prove a meter is current or infer why
readings are absent. A sole confirmed identity needs no explicit selector.

The tool does not write selections, .env, SQLite or raw telemetry. After confirming
choices, configure the energything runtime privately and enable its explicit
one-shot ingestion to populate readings. This report itself does not ingest.

Task 05 adds bounded read-only Growatt discovery/latest/provider-day collection
through `cmd/growatt-verify`, with no automatic scheduler. The [protocol gate](docs/growatt-protocol.md)
records verified endpoints and explicit unresolved unit/time/identity states.
Use a private report and separate verification database as described in
[operations](docs/operations.md). Authenticated stored browsing includes full
plant/device identifiers, exact values and qualified telemetry. Task 06 owns
recurring work and automatic available-history recovery.

## Retained daily generation (task05a)

On 15 October 2026 the user agreed to a separate aggregate-ingestion task before
scheduling. [Task05a](docs/tasks/milestone-1/05a-growatt-aggregates.md) defines the
historical generation baseline, separate storage and read API/table contract.
Daily plant generation is retained separately through an explicit bounded CLI,
versioned SQLite aggregate storage, authenticated `/api/v1/growatt/aggregates` and
compact `/records/growatt/aggregates` browsing. See [protocol](docs/growatt-protocol.md),
[operations](docs/operations.md#explicit-bounded-daily-aggregate-collection-task05a)
and [verification](docs/tasks/milestone-1/05a-verification.md). Exact kWh, provider
calendar dates, unverified timezone/zero availability qualifications and corrections
are retained. One day per invocation; documented five-minute and 10/day limits.
No automatic aggregate scheduling, archive scan or refresh endpoint is added.
