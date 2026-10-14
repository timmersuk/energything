# Task 04 execution handoff

Implemented on 14 October 2026 on `task04-octopus`, based on merged main `79ffae9`.
Initial Git working tree was clean. All edits are in energything. Task 01–03 code,
HTTP/storage contracts and approved planning files are preserved. The pinned
Alpine base is unchanged. No recurring scheduler, refresh/audit route, Growatt,
gas, tariff, monetary total, CSV import or later task implementation was added.

## Files and contracts

- `internal/octopus/{client,discovery,readings,validate}.go` and tests: minimal
  ephemeral account discovery, independent selection, authenticated destination
  policy, bounded cancellable fetch/retry and exact whole-window validation.
- `internal/collection/octopus.go` and tests: explicit one-shot orchestration,
  independent streams, typed attempt/commit/failure writes and bounded cancellation
  audit cleanup. No cloud work occurs in a storage transaction or handler.
- `internal/storage/discovery.go`, tests, `migrations/002_discovery.sql`,
  `schema.go`, migration regression test: narrow durable per-stream discovery,
  atomic selection replacement and version 1 upgrade with existing data retained.
- `internal/config`, `cmd/server/main.go`, `.env.example`, Compose: opt-in
  `ENERGYTHING_OCTOPUS_ONCE` (default false), worker lifecycle and stream isolation.
- `internal/web/status.go`, status template and failure stub: masked stored
  discovery states, retaining existing status/coverage/record contracts.
- `scripts/task04-fixtures`, `task04-browser.js`, `task04-private.ps1`: repeatable
  isolated synthetic/browser and private runtime verification paths.
- README, data semantics, HTTP API and this handoff: exact behavior and limits.

Detailed fetch/selection/retry/evidence contracts are in [data semantics](../../data-semantics.md#octopus-protocol-and-one-shot-handoff-task-04).
Every fetch is `[from,to)`, <=14 absolute days; all pages precede validation and
commit. Rejection/failure leaves accepted rows, latest accepted coverage and
checkpoint unchanged. Later corrections retain earlier evidence/versions; absence
does not delete retained rows. Coverage is latest response coverage, not retained
count. No accepted progress is claimed when a storage transaction fails.

Reviewed the three specified sibling export files locally; did not execute its
CLI, read sibling configuration or copy private artifacts/identifiers. Official
[Octopus endpoint documentation](https://docs.octopus.energy/rest/guides/endpoints/)
confirms account direction shape, electricity/export consumption endpoint,
UTC/overlap query behavior and half-hour measurement shape. The conservative
`to-1s` adapter is backed by official overlap examples and adjacent synthetic
windows; real consumption boundary behavior remains part of the gated live fetch.

## Checks actually run

- Pure selection test first failed to compile before implementation, then passed.
- `go test ./...`: complete native suite passed, including provider/collection,
  real temporary SQLite integration, existing web/config/lifecycle tests.
- `go vet ./...`: passed. Linux amd64/arm64 CGO-disabled builds passed via
  `go run ./scripts/build.go -build-id task04-octopus`.
- `docker build -t energything:task04 .`: actual pinned Alpine task image passed.
- Exact preview Compose combination `config --quiet`: passed without printing
  resolved configuration or reading .env values.
- New synthetic fixtures generated through the production typed provider/collection
  path, never against the preview database.
- Actual task-build Edge acceptance passed using `scripts/task04-browser.js`:
  independent selected streams; ambiguous import with selected/collected export;
  masked discovery; exact visible/JSON decimals; 46-slot missing coverage; filters;
  empty state; absent later routes; zero browsing outbound requests.
  Agent visually inspected synthetic status screenshot. Favicon 404 is the only
  observed console error; no provider/script error occurred.

Synthetic matrix covers true/false/null, dedup/conflict/mismatch/unpaired selection;
foreign host/scheme/userinfo/port/path/query/redirect; count/missing/empty/truncated
results; repeated pages/cycles and byte/page/record/window caps; out-of-order pages;
exact exponent/long-decimal/zero values; identical/conflicting readings; negative,
invalid duration/alignment/overlap/straddle; adjacent neighbors/windows; correction,
gaps and retained rows; failed second page with incomplete evidence; 429/5xx/408,
auth pause, Retry-After seconds/date/deferral, network and request/body/wait
cancellation; 46/50-slot London days and partial elapsed day; typed begin/commit/
failure-audit errors with no false completion. Migration tests cover version 1
upgrade, atomic selection rollback, stale deselection and preserved identities.

Initial sandbox loopback lifecycle tests failed and passed when rerun with allowed
loopback/cache access. Migration tests initially assumed schema version 1 and were
updated for version 2. A collection request-count assertion was corrected from
an incorrect expected value to the independently enumerated 20 requests. A
Compose text-edit interpolation mistake was corrected before `config --quiet`.
These initial failures are not verification passes.

## Browser/private evidence and remaining gates

Preview safe metadata confirmed `energything-preview`, original
`D:\projects\energything\.cache\preview-data` mount and its existing image.
Preview was never recreated, restarted, migrated or reconfigured. No plain
Compose startup was used. Task browser containers use new
`.cache/task04-browser-2240583994/{selected,ambiguous,empty}` mounts on loopback
8083/8084/8085. Synthetic screenshots are `.cache/task04-{status,records}-*.png`;
browser output stays under ignored `.playwright-cli`.

Process Octopus credentials were unavailable, but the existing energything preview
runtime had key/account credentials. Only presence booleans were printed; no .env
values were read/displayed and no secret/account payload was logged. After synthetic
verification, the task image ran on loopback 8086 with a fresh private mount using
only energything runtime credentials. Actual HTTPS authenticated account discovery
succeeded and confirmed import/export candidates. Both directions had multiple
eligible meters and lacked explicit runtime MPAN/serial pairs. Both stored
`selection_required`; no guessed meter request occurred. **Live bounded consumption
ingestion is unverified: explicit confirmed selection is the unavailable gate.**
The container was stopped. Private SQLite/evidence and masked status remain under
an ignored `.cache/task04-private-*` directory; no private data is tracked/public.

Human/mobile/other-engine visual review, real ARM64 hardware and OMV remain
unverified. ARM64 here is cross-build only. No preview database upgrade, recurring
job execution, full backfill progression or release/deployment was exercised.
The explicit one-shot collects only one bounded unit per selected direction.

## Required review

Pending final two-axis review against fixed main `79ffae9`; findings and resulting
checks will be recorded here before opening the separate PR. No merge is authorized.
