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

The code-review skill ran independent Standards and Spec agents against fixed
main `79ffae9`. Standards found one issue: response-body timeouts bypassed retries;
no material judgement-call smells. Spec found two issues: null count accepted as
zero and repeated-page hashing depending on whitespace/object order. All three
were corrected. Regression tests cover timeout then success/exhaustion, canonical
repeated pages with different formatting/row order, and null count through typed
collection with no checkpoint. Static selection/recovery guidance was also added
to the stored discovery UI. Final checks and actual-build acceptance were rerun.
No merge is authorized.

## Authorized preview update follow-up

The user subsequently requested restarting the local Docker preview and making
that the standard future workflow. The verified final task 04 image was tagged
`energything:local`; the exact project/file combination was validated quietly.
The preview was stopped cleanly, its whole original data directory backed up to
a new ignored `.cache/preview-backups/task04-*` directory, then recreated with
both saved Compose files, its original name/port/mount and one-shot collection
disabled. This supersedes the earlier statement that the preview was untouched:
the earlier task verification preserved it; this follow-up explicitly upgrades it.
README and operations now require the current task preview update and verification
as part of future handoffs, with separate synthetic databases and safe mount checks.

Post-recreation checks passed: running state, original bind mount, public health,
authenticated status build ID `task04-octopus` and one-shot disabled. No live
collection was triggered by the preview upgrade.

## Read-only discovery CLI follow-up

The user requested a small one-off CLI instead of extending the app/REST selection
flow. `cmd/octopus-discover` reuses the existing account discovery, independent
selection and validated bounded fetch client. It reports candidate IDs privately,
recent slot coverage, oldest/newest observations, page/retry counts and enumerated
failures; it never reads/writes the app store or applies configuration.

Default sampling covers 14 absolute days through the latest elapsed half-hour,
with 20-candidate and 2m overall budgets. Unknown/conflicting directions are skipped;
auth/rate-limit/cancellation stops subsequent sampling. The tentative steer requires
exactly one recently reporting candidate (last 48h) among successfully sampled
candidates. Multiple active candidates and incomplete/failed samples remain unresolved.
Zero readings count as telemetry. No inference of meter lifetime is made.

Synthetic CLI tests cover independent import/export, unknown/conflicting discovery,
zero readings, recent versus empty/older candidates, multiple recent candidates,
network/validation/auth/rate-limit failures, cancellation, candidate budget,
sanitary errors, private report contents and refusal to overwrite an existing file.
CLI reports contain only necessary identity and summary data, no account/address
payload, API key or individual energy readings. Default reports are ignored.

Executed CLI follow-up checks: targeted CLI tests, full `go test ./...` and
`go vet ./...` passed. Independent Spec/Standards reviews found no additional
findings. Actual authenticated candidate sampling completed with the existing
energything runtime credentials, creating a private ignored report. No identifiers,
account payloads or sample values are included in this public handoff. Application
selection and database state were unchanged. Server/web code and build are unchanged,
so no preview recreation was needed for this standalone CLI addition.

## Automatic collection plan revision

On 14 October 2026 the user rejected the temporary one-shot/backfill-start operating
workflow. Task 06, the shared execution contract and parent scheduling plan were
revised: startup automatically collects and bootstraps each selected identity,
reconciles existing partial/empty fetched history and resumes durable backfill.
Task 06 removes ONCE/BACKFILL controls and environment-triggered repair ranges;
exceptional repair uses a durable CLI enqueue action. A required provider gate
establishes earliest accessible history or a finite supported search/stopping rule;
gaps do not prove history ends, and no arbitrary short range is called the full
archive. No task 06 runtime code was added. Current task 04 behavior is unchanged.

## Authorized ingestion after private selection

The user applied the privately reported meter settings and requested ingestion.
The preview was recreated with one-shot enabled using the exact saved Compose
project/files and original mount. Both selected directions completed authenticated
bounded ingestion successfully. Selected-stream records were confirmed through
bounded authenticated API and HTML table reads. One-shot was then disabled and the
preview recreated; accepted data remained queryable on the original mount.
This supersedes the earlier missing-selection/live-ingestion gate for the executed
recent window only; it does not establish full historical or measurement coverage.
No credential values, raw identifiers or telemetry values are included here.
An initial verification query used PowerShell-coerced locale timestamps and was
correctly rejected; preserving RFC3339 strings produced successful verification.
