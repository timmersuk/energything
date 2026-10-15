# 05a — Retained Growatt generation aggregates

Status: daily implementation verified for review (15 October 2026); open protocol qualifications in [verification](05a-verification.md).
Depends on: 02, 03, 05. Unlocks: 06, 06a.
Read: [execution contract](execution-contract.md), [task05 handoff](05-verification.md),
[aggregate investigation](../../growatt-aggregate-investigation.md), storage/HTTP
contracts and operations backup/preview procedure.

## Objective and user decision

Provide a historical baseline without waiting for a new year of local detailed
telemetry. The user agreed to a separate aggregate-ingestion task before scheduling,
with its own storage and API contract. Retain provider-reported generation totals
independently of instantaneous Growatt observations and Octopus meter intervals.

The detailed MIX endpoint's observed rolling approximately 95-day window does not
limit all Growatt data. One older winter daily generation aggregate has already
been privately verified through GET /v1/plant/energy. This proves older availability,
not a complete annual archive or the earliest accessible aggregate date.

Daily generation is the initial deliverable. Validate and add monthly/yearly feeds
only if useful and their request/period semantics are established. Daily records
alone can support later annual-pattern charts. Charts, derived seasonal analytics,
tariffs, financial calculations and battery/grid/load aggregate invention are not
part of this task.

## Protocol gate

1. Verify the documented plant/energy read contract, permission, required date/unit
   arguments, response/error/pagination shapes and five-minute request frequency.
   Add only the verified read endpoint to the authenticated destination allowlist;
   retain exact HTTPS host checks, disabled redirects, cancellation/body/row budgets
   and explicit retry ownership. Do not invoke settings/control or website sessions.
2. Establish exact generation metric, kWh unit, calendar-date/period boundaries and
   timezone provenance. Do not label generation as grid export, household consumption
   or battery energy. Do not interpret a calendar date as a measured UTC instant.
3. Test the documented <=7-day date-span behavior, inclusion boundaries, ordering,
   complete pagination and stable counts. Specify exact engineering request budgets.
   Prove zero-versus-absent/null/error behavior; a generated zero row must not become
   proof of source-lifetime coverage or an earliest-history boundary.
4. Verify availability on bounded older requests and record what remains unknown.
   Do not assume a daily/monthly/yearly retention guarantee from endpoint existence,
   a current lifetime counter, one success or empty responses. Month/year date formats
   and overlap semantics need their own gate before those resolutions are ingested.
5. Keep private values/identifiers/payloads ignored; public fixtures/examples are
   synthetic. Authenticated application views retain useful full plant identifiers.

## Storage contract amendment

Introduce a separate SQLite table, provisionally `energy_aggregates`, through a
versioned migration. Finalize the exact typed contract in data-semantics before
implementation; do not write observations through raw SQL outside storage.

- Plant-level source identity: provider + confirmed plant + metric. Generation for
  a multi-device plant is not copied once per device or attributed to one inverter.
  Device replacement must not duplicate plant totals; deselection preserves history.
- Logical record identity: source identity + metric + resolution + provider calendar
  period. Initial supported resolution is day. Retain original date and explicit
  calendar-zone provenance/quality; nullable UTC boundaries if not established.
- Exact decimal generation value, verified unit, retrieval time, attempt/retrieval/
  evidence association and qualification. No float round trips, inferred intervals
  or power-to-energy integration.
- Current accepted values plus retained superseded versions/evidence. Replay is
  idempotent for established period keys; valid corrections retain previous versions.
  Identical duplicates collapse, conflicting duplicates reject the complete unit.
- Separate aggregate transport/fetched coverage from measurement/availability claims.
  Empty or missing periods never erase accepted history or create zero production.
- Typed attempt/accept/failure methods preserve existing short-transaction guarantees:
  complete bounded-unit validation before atomic commit; failed/rejected evidence
  and specific safe diagnostics without accepted/checkpoint changes. Extend existing
  job/window/audit types explicitly where necessary; do not disguise aggregate work
  as instantaneous latest or Octopus half-hour consumption.

Aggregates and detailed observations may overlap in calendar time without being
added together. Future annual generation charts should use a consistent metric/feed;
monthly/yearly totals must not be added to constituent daily totals or expanded
into invented daily readings. This task retains the inputs; later chart logic owns
selection of compatible series and explicit partial-period display.

## Bounded collection and browsing

Implement one bounded aggregate unit through provider/collection/storage seams,
with synthetic HTTP/storage fixtures first. An explicit private verification CLI
may exercise selected bounded dates; no new ONCE/backfill-start variables, timers,
automatic archive scans, refresh endpoints or cross-process worker are added.

Proposed read contracts to finalize before coding:

- GET /api/v1/growatt/aggregates: authenticated, bounded plant/metric/calendar-date
  query, exact decimal strings, unit/resolution/period/source/quality/provenance and
  stable filter-bound keyset paging. No cloud call from GET.
- GET /records/growatt/aggregates: compact local generation table, explicit calendar
  dates and kWh, full plant identity, missing/partial/qualified states. Keep raw
  evidence behind optional detail; use normal scrolling and clear navigation from
  Growatt samples. No new chart dashboard or custom scroll controls.

Preserve all existing Octopus, sample, status, auth/no-store and data contracts.
Back up the cleanly stopped preview before migrations; retain its exact saved
Compose pair/project/original data mount. Browser fixtures use a separate database.
Verify actual task image/UI and rebuild/recreate preview after verification.

## Verification and handoff

Cover synthetic optional/zero/invalid decimal/unit/date cases, complete paging,
changed counts/cycles/early stop, duplicate conflict, whole-unit rejection, replay/
correction/reopen, empty preserving history, plant-versus-device identity, aggregate
and detailed overlap without addition, and existing-provider browsing during failure.
Verify auth/method/query/paging/privacy and useful owner diagnostics. Run full Go
tests, vet, required builds, real SQLite integration and actual-image browser checks;
perform bounded authorized private older-date verification where available.

Publish 05a-verification.md with exact protocol/storage/query/retry contracts,
private/browser outcomes, unavailable gates and files/checks actually run. Update
README/data-semantics/http-api/operations/execution-contract and task06 handoff.
Task06 then owns independent routine aggregate refresh, resumable available-history
bootstrap, explicit earliest-boundary policy/provenance and retry scheduling. Keep
aggregate historical progress independent of detailed samples and Octopus streams;
existing detailed history must not suppress missing aggregate bootstrap.
