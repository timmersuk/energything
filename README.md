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
