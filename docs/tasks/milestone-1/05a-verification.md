# Task05a verification — retained daily Growatt generation

15 October 2026. Daily implementation verified; protocol qualifications below
remain open and must not be described as resolved retention/calendar guarantees.
No task06 implementation or automatic archive search was added.

## Files and contracts

- cmd/growatt-aggregate: explicit one-day confirmed-plant CLI; process token,
  existing private discovery report, stopped/private database, 2m context and
  bounded audit cleanup. No .env loading, ONCE/start-date settings or scheduler.
- internal/growatt/{aggregates,client}: only GET plant/energy added to the exact
  HTTPS host/path/form allowlist; header token, redirects disabled, 30s and4MiB
  per page, 100pages/10,000 raw rows. Stable count/day resolution/period/decimal
  validation, complete unit only. Exact decimal strings, no float conversion.
- internal/storage migration005: separate energy_aggregates current table and
  append-only aggregate_versions, plus durable aggregate_calls reservation ledger.
  Plant identity growatt/generation/plantID, independent of inverter replacement.
  Metric generation, day resolution, original provider calendar date, kWh,
  unverified calendar-zone provenance, qualified zero, retrieval/evidence links.
  No UTC measurement interval invented. Current replay idempotent; correction
  versions and every accepted retrieval remain. No absence deletes accepted rows.
- AcceptedWindow.Aggregates reuses the short atomic evidence/observation/version/
  completion/checkpoint/terminal-audit transaction. Invalid/conflicting duplicates
  reject; equal numeric duplicates collapse. Failed/rejected audits retain selected
  evidence and specific enumerated reasons with no accepted/checkpoint updates.
  Aggregate coverage is always qualified, never measurement-complete.
- internal/collection: CollectGrowattAggregateUnit, no timer/caller retry loop;
  context cancellation and detached bounded failure-audit cleanup. Storage failure
  is reported separately; a failed audit is never claimed durable.
- internal/web: authenticated GET-only local API/table, no-store, full plant IDs,
  exact values, kWh/calendar dates, qualification and expandable provenance;
  inclusive calendar-date/plant/metric/resolution filters, bounded filter-bound
  ascending period/identity keyset paging. UTC period boundaries null.
- Discovery deselection now targets telemetry only; independent generation survives.
  Status includes aggregate retained counts without aggregating energy feeds.
- Synthetic fixture/browser/preview helpers and relevant README, semantics, HTTP,
  protocol, operations, execution-contract and task06/06a handoff docs updated.

## Protocol and retry gates actually checked

Saved official public plant/energy page documents kWh generation, day/month/year,
<=7-day span,100rows/page, count/time_unit/energys and **10 calls/day**, alongside
one call/five minutes. The ten/day restriction was missing from the task summary;
the implementation conservatively reserves at most ten transmissions per rolling
24h and spaces them five minutes through durable local storage. Failed requests
consume reservations. Aggregate requests have one transmission/page, **no inline
HTTP retries**, unlike the unchanged detailed client. A page requiring a later
quota slot defers/rejects the complete unit; partial evidence is retained, no
checkpoint advances. Task06 must honor DeferredUntil and share the same ledger.
Other databases/software calls are not observable; manual probes were coordinated.

Private authorized calls made this session, spaced beyond five minutes:
1. January12–18: count7, seven ascending rows, both endpoints included, day unit,
   exact decimal strings, no reported zeros in that response.
2. January15 through the new CLI into a separate private database: accepted one
   older daily generation row, outside the detailed telemetry availability window.
3. January15 through the CLI into the cleanly stopped/backed-up preview: accepted
   one row; then preview recreated. No CLI opened the running preview database.

No credentials, identifiers, amounts or real payloads are in public fixtures or
this handoff. Ignored private evidence is under .cache/task05-aggregate-research
and .cache/task05a-private. The official page was already saved in the former;
a fresh browserless public-page fetch timed out, so the saved primary document
was used, rather than claiming a fresh public read succeeded.

## Executed checks

- Targeted Go provider/storage/collection/web suites: passed. Includes exact tiny
  decimals, zero/empty/null/missing/placeholder/negative/numeric wrong types, wrong
  unit/date, equal/conflicting duplicates, changed counts, cycles, early stop,
  complete synthetic paging, cancellation, redirects and one-transmission failures.
- Real SQLite: replay/correction/reopen, conflicting-unit rejection, empty preserving
  history, injected transaction rollback, durable quota/reopen, migration001 to005,
  migration failure rollback/newer-schema rejection; passed.
- Collection integration: acceptance, validation rejection with selected evidence
  and specific audit reason, authentication failure, quota deferral preserving
  prior acceptance, wrong-plant rejection and one-transmission transport failures;
  passed. No live failure was manufactured against Growatt.
- Handler/storage integration: auth, methods, no-store, filter-bound multi-plant
  paging with equal dates, limits, malformed/unknown/duplicate filters, exact values,
  zero qualification, full identity and Octopus browsing; passed.
- Full `go test ./...`, `go vet ./...`, native server/CLI builds and Linux amd64/arm64
  builds: passed. Existing localhost server tests fail under restricted networking;
  rerunning with authorized localhost/cache access passed. No source workaround.
- Linux amd64 and arm64 actual-image SQLite runtime/migration tests: passed.
  Initial arm64 invocation used the amd64-only tag and could not start; the proper
  arm64 image was built from the unchanged pinned multi-platform base, then passed.
- Playwright CLI using installed Edge, actual task image and separate synthetic
  database at localhost8087: passed exact/full IDs, compact rows, paging, plant
  filter, zero qualification, expandable provenance, sample navigation, retained
  Octopus browsing and zero outbound cloud browser requests. Chrome was unavailable;
  Edge succeeded. Synthetic screenshot is ignored .cache/task05a-browser.png.
- Private preview browser: passed task build, older aggregate exact owner-visible
  value/plant, null UTC bounds, provenance expansion and both existing record pages.
  Credential-bearing generated code/output was ignored and removed afterward.

## Preview

energything-preview; Compose project energything; exact docker-compose.yml plus
ignored .cache/preview-compose.override.json; original .cache/preview-data mount.
Clean stop, entire directory backup in ignored .cache/task05a-preview-backup-*,
then migration/one-day ingestion while stopped and force recreation. Health,
actual image ID, task build, mount and every preexisting identity retained count
were verified; all true. Octopus startup one-shot remains false. Approved pinned
Alpine base unchanged. No synthetic data was put in preview.

## Unresolved gates and handoff

Provider calendar timezone/DST boundaries are not established by this endpoint;
retain date-only provenance and null UTC boundaries. Live missing/null/placeholder
and generated-zero availability semantics were not observed; synthetic behavior
is fail-closed, zero explicitly has unknown availability meaning. Oldest aggregate
history/retention and annual completeness remain unknown. No zero/empty result
establishes a boundary. Daily real multi-page output remains unverified; whole
pagination is synthetic tested and quota deferral never accepts partial results.
The exact seven-day rejection edge was not probed; production uses start=end.
Month/year formats and overlap remain unverified and are not ingested. Richer
battery/grid/load/SOC aggregates are not established. Task06 owns independent
bootstrap, availability investigation/stopping policy, scheduling and retries;
task06a owns raw retained evidence inspection. No task was merged or started.

## Pre-push review

The implement skill's code-review Standards and Spec axes reviewed main...HEAD.
Standards: zero actionable findings. Spec: one P2 selector-binding edge found and
fixed before push. ValidateAggregateJob now checks plant, identity, kind, calendar
date and bookkeeping bounds against persisted work before quota reservation/HTTP.
Mismatched selectors record configuration/aggregate_request with no accepted
progress, even if the hypothetical provider response is empty. Wrong-plant and
wrong-date empty-response regression tests assert zero transmissions and no
checkpoint/window changes. Spec re-review confirmed the fix; zero findings remain.

After the review fix, full tests/vet/native and both Linux builds passed again;
both architecture SQLite runtime/migration gates passed on rebuilt final images.
Preview was cleanly backed up/recreated again with the saved Compose pair (no
additional provider call), retained counts and private browser checks passed.
