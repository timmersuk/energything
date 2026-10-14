# Task 03 execution handoff

Implemented and verified with synthetic fixtures on 14 October 2026, on branch
`task03-browsing`, based on merged main `5f95c81`. The initial working tree was
clean; task 01/02 implementations and planning files are preserved. All edits
are in energything. No .env values were read or displayed. The approved pinned
Alpine image, Compose files and existing preview database are unchanged.

## Files changed

- `cmd/server/main.go`: wires the opened typed store into browsing handlers.
- `internal/web/query.go`: shared date/instant/quality/filter/limit parsing and
  next links preserving effective range and filters.
- `internal/web/records.go`, `status.go`: consumer-owned read-only storage seam,
  explicit versioned JSON/view models, masked identities, nullable UTC times,
  exact values, bounded record pages and sanitized operational summaries.
- `internal/web/router.go`: task 03 routes, unchanged auth/liveness protection,
  malformed query encoding rejection and safe storage errors.
- `internal/web/templates/status.html`, `octopus.html`, `growatt.html`: stored
  cards/tables, filters, cursor navigation, separate labelled quality view,
  empty/partial/configuration/error/running states and escaped provider text.
- `internal/web/static/style.css`: modest filter/table/navigation layout.
- `internal/web/router_test.go`: retains task 01 auth/empty/escaping assertions,
  using real opened storage; formerly absent routes now assert implemented reads.
- `internal/web/browsing_test.go`: real-storage task 03 acceptance and transport
  rejection stub, safe storage failures and read-only behavior assertions.
- `internal/storage/query.go`: typed `RecordQuery.Direction`, included in cursor
  binding and rejected for samples. Also fixes a status pointer-alias bug: the
  last accepted attempt no longer overwrites a later failure/running summary.
- `internal/testsupport/browsing.go`, `scripts/task03-fixtures/main.go`: synthetic
  typed seeding support, excluded from application imports. Creates fresh random
  directories under ignored `.cache`; accepts no existing database path.
- `scripts/task03-browser.js`: repeatable actual-build Playwright acceptance.
- `.gitignore`: excludes Playwright snapshots/runtime outputs.
- `docs/http-api.md`: exact routes, query examples, response fields and errors.
- `docs/data-semantics.md`, `README.md`: current contracts/scope and handoff links.
- This handoff.

## Contract for dependent tasks

See [HTTP API](../../http-api.md) for complete field names and examples. GET `/`
and the two `/records/*` pages, `/api/v1/status`, `/api/v1/octopus/intervals`,
`/api/v1/growatt/samples` and the embedded stylesheet require Basic Auth.
`/healthz` remains public. Audit/refresh endpoints return 404; no charts,
provider calls, scheduler, collection policy changes or tasks 04/05 were added.

Both pages and API accept paired inclusive calendar dates or paired explicitly
offset instants (exclusive `to`). Defaults are the current configured local day,
100 rows, maximum 500. Cursors bind range, identity, direction/quality/view,
provider date and page size. Tables order by measurement instant and stable local
identity/row ID; quality view orders by retrieval instant/row ID. Live keyset
paging has no snapshot promise; new arrivals behind the cursor require a fresh
query. HTTP rejects unknown/repeated parameters and unknown/cross-provider local
identity selectors. Credentials/raw stable identifiers/evidence are absent.

Decimal values are strings; missing values/units and unresolved measurement times
are null. Resolved Growatt queries label a count of all omitted unresolved rows
in the identity selection, without guessing their measurement date. Separate
quality view filters provider dates and/or retrieval ranges and retains original
clock text. Selection and protocol interpretation remain provider-task work.

Status distinguishes configuration validity, stored discovery, selection, retained
counts, retrieval versus measurement, latest versus accepted attempt, running/
paused/backfill jobs, checkpoint and fetched versus measurement coverage. The
server-rendered page has no asynchronous loading phase; a stored running attempt
is shown as running. All reads use typed storage; no handler SQL or writes.

## Checks actually run

| Check | Observed result |
| --- | --- |
| `go test ./internal/web -v` and targeted reruns | Passed after fixture/assertion corrections |
| `go test ./internal/web ./internal/storage` | Passed after status alias fix |
| `go test ./...` | Final complete native suite passed |
| `go vet ./...` | Passed |
| `go build ./cmd/server` | Passed |
| `gofmt -w` on changed Go source | Applied |
| `go run ./scripts/build.go -build-id task03-browsing` | Linux amd64 and arm64 CGO-disabled builds passed |
| `docker build -t energything:task03 .` | Passed using unchanged non-root pinned Alpine packaging |
| `go run ./scripts/task03-fixtures` | Fresh isolated empty/fixture SQLite databases created through typed methods |
| `playwright-cli -s=energything-task03 run-code --filename=scripts/task03-browser.js` | Passed in installed Edge against task03 image on 8081/8082 |
| `git diff --check` | Passed |
| Preview safe metadata inspection before/after | Running with original `.cache/preview-data` bind mount; never recreated or reconfigured |

Handler assertions exercise no/wrong auth on every implemented protected route
and asset; GET-only behavior and absent future endpoints; useful empty pages;
exact `0.100000000000000001`/`0.000`, SOC null versus `"0"` and null units;
London 46/50-slot bounds; malformed/mixed/missing ranges, limits, encoding,
duplicate/unknown parameters and cursors; stable interval and quality pagination;
new earlier arrivals between pages without duplicate/omitted preexisting records;
cursor filter mismatch; escaped script text; zero calls to a rejecting default
HTTP transport; recent retrieval of older measurements; omitted unresolved counts;
failed versus accepted attempt distinction; sanitized 503 on real closed storage
and arbitrary error stub; unchanged durable attempt counts after browsing.

## Actual browser verification

The existing preview on localhost:8080 was left intact. Safe Docker inspection
identified its original `/data` bind source `D:\projects\energything\.cache\preview-data`
and its Compose project files `docker-compose.yml` plus ignored
`.cache/preview-compose.override.json`. No plain Compose command was used.

Two independent task image containers use newly created mounts under
`.cache/task03-browser-2318655554/{fixtures,empty}` on loopback ports 8081 and
8082. Credentials and provider configuration in those containers are synthetic.
Fixtures intentionally use synthetic future October observations/retrievals to
exercise the autumn DST gate before the actual date. They are not real history.
No private fixture or production seed endpoint exists.

The fixture container has a partial synthetic Octopus configuration error and
synthetic valid Growatt configuration. The empty container has both unconfigured.
Fixtures include both directions, 46/50-slot days, a deselected historical source
with 47 missing slots, paused failed work, and repeated unresolved local clock text.
Storage startup reconciles the seed’s abandoned running attempt as interrupted;
pre-restart running behavior is verified by handlers instead of falsely claiming
it survives restart. No collection runs.

Browser script result:

```json
{"passed":true,"authenticatedRoutes":7,"calendarSlots":[46,50],"paginatedRows":100,"pages":34,"unresolvedExcluded":4,"outboundRequests":0}
```

Actual navigation and filter submission exercised table and next-page behavior,
quality/provider-date view, visible null/zero and escaped script text with no
script element/execution. Browser-context HTTP requests exercised auth challenges
for all seven protected paths, API types, bounds, all 34 pages, equal timestamps,
retrieval filters, safe errors and absent future routes. Empty cards and both
tables were visited in the empty container. API and visible status keep old
measurement time separate from retrieval time. Agent inspected the actual
Growatt screenshot for table/filter layout and escaping.

Screenshots are local ignored verification artifacts:
`.cache/task03-{status,octopus,growatt,empty}.png`; browser snapshots/logs are
ignored under `.playwright-cli`. These contain synthetic values only.

Reproduce from repository root (requires Go, Docker and installed Edge/Playwright CLI):

```powershell
go run ./scripts/task03-fixtures # prints a NEW absolute root; use it below
go run ./scripts/build.go -build-id task03-browsing
docker build -t energything:task03 .
# Replace <root> with the printed path. Never substitute the preview data path.
docker run -d --name energything-task03-fixtures -p 127.0.0.1:8081:8080 --mount 'type=bind,source=<root>\fixtures,target=/data' -e ENERGYTHING_USERNAME=synthetic -e ENERGYTHING_PASSWORD=synthetic -e OCTOPUS_API_KEY=synthetic -e GROWATT_API_TOKEN=synthetic energything:task03
docker run -d --name energything-task03-empty -p 127.0.0.1:8082:8080 --mount 'type=bind,source=<root>\empty,target=/data' -e ENERGYTHING_USERNAME=synthetic -e ENERGYTHING_PASSWORD=synthetic energything:task03
playwright-cli -s=energything-task03 open --browser=msedge
playwright-cli -s=energything-task03 run-code --filename=scripts/task03-browser.js
```

Initial Chrome launch failed because Chrome is absent; Edge succeeded. Initial
browser assertions used an incorrect sanitized-message expectation and a select
label locator; both were corrected, followed by the successful complete run.
One synthetic seed initially deselected a source before attempting work; typed
storage rejected it, and the seed was corrected to deselect after recording work.
A test initially reused a decoded struct’s absent next-cursor field; resetting
it before decoding the terminal page fixed the assertion. None of these failed
runs is counted as a verification pass. Docker and Go cache/loopback checks used
approved elevated access after initial sandbox Docker denial.

## Remaining gates

Human visual review, mobile browser review, other browser engines, actual ARM64
runtime/hardware and OMV deployment remain unverified by task 03. ARM64 here is
a successful cross-build only. No real provider credentials/interpretation,
collection, preview database upgrade, refresh/audit API, backup/restore or release
publication was exercised. The existing preview still serves its earlier build;
the verified task build is separate. Later tasks populate the established typed
store rather than adding provider HTTP calls to browsing handlers.
