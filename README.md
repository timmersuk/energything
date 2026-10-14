# Energything

Task 01 supplies an authenticated empty application and SQLite lifecycle.
Provider requests, observations, scheduling and APIs are not implemented.

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
Valid credentials produce a configured card, but no network requests.

GET /healthz returns public fixed process liveness. GET / and
GET /static/style.css require Basic Auth. Other paths return 404; wrong methods
on implemented routes return 405. No API, refresh, records or audit route exists yet.
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
checks and remaining gates. Task 02 must wait for the SQLite runtime gate.
