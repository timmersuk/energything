# 05 — Growatt protocol gate and read-only latest/history collection

Status: planning draft. Depends on: 02, 03. Unlocks: 06 after protocol gate.
Read: [execution contract](execution-contract.md), storage handoff and HTTP contract.

## Objective and boundary

Retain SPH/MIX latest and day-history telemetry with established units and honest
timestamp identity/quality. Discovery/latest/history are the only allowed endpoint
behaviors. No inverter settings reads for control investigation, writes, local
logger adapter, energy integration from power or historical-file migration.

## Existing evidence and known pitfalls

Read only generic protocol logic in sibling octopull scripts/pull-growatt-soc.ps1
and scripts/backfill-growatt-soc.ps1. They show token-header authenticated POST
reads on openapi.growatt.com, MIX latest/history paths, day requests, perpage/page,
reported count and timezone_id. They also load private local files/defaults and
key samples by local time. Those are not production behaviors to copy. Do not run
them, use their defaults or deduplicate by local text. Never invoke the neighboring
control-inspection script as part of this task.

The reference establishes latest/history examples, not a complete verified cloud
contract. Discovery endpoint shapes, supported family codes, field units and time
semantics are implementation gates. Do not fill these gaps with a guessed URL,
assumed unit, sign or a fixed offset from one observed day.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/growatt/client.go | Exact read allowlist, token header, safe HTTP/body/retry behavior |
| internal/growatt/discovery.go | Minimal plant/device metadata and supported-family selection |
| internal/growatt/history.go, latest.go | Provider-date paging and latest response acquisition |
| internal/growatt/normalize.go | Verified field units/signs, nullable values and time quality |
| internal/collection/growatt.go | Attempt orchestration and storage commit/failure audit |
| docs/growatt-protocol.md | Generic verified endpoint/field/time contract with synthetic examples |
| internal/growatt/*_test.go | Synthetic discovery, page, normalization and DST cases |

## Gate A — establish protocol before normalizing

1. Review authoritative provider material and generic private investigation findings
   locally. Use private read-only verification if needed and available; record only
   generic conclusions and synthetic examples publicly. No real payloads or identifiers.
2. Enumerate an explicit method/path allowlist for plant discovery, device discovery,
   supported device details needed for telemetry, MIX/SPH latest and history. For
   each, document form fields, response envelope, success/error code and pagination.
   Unsupported endpoints cannot be called through a generic request helper from
   external configuration. Validate host/destination before adding the token;
   disable credential-carrying redirects.
3. Define family support from evidence, not substring guesses. Unique eligible
   device may be auto-selected; multiple candidates require explicit plant/device.
   Missing/unsupported device has actionable state without blocking Octopus.
4. Create a field matrix covering SOC/BMS SOC, solar, load, grid-to-house/grid-export,
   battery charge/discharge and available energy counters. Record provider field,
   unit, scaling, sign, normalized meaning, optionality and invalidity criteria.
   Field absence stays null. Unknown selected telemetry fields remain in evidence.
   Do not equate SOC and BMS SOC or invent unavailable measurements.
5. Define timestamp precedence and stable observation identity using readable time,
   timezone metadata, embedded calendar/epochs and provider IDs. Distinguish resolved,
   ambiguous, contradictory and missing time. Retain originals. Repeated autumn
   local clocks need identity/offset evidence; a timezone name alone does not pick
   which occurrence. Do not use the server/host timezone to guess.
6. Document optional-quality versus hard-rejection rules. Unresolved time can be
   retained as qualified evidence/sample with null instant; known contradictory
   readings or malformed required structure reject the unit. State exactly which
   cases block normalization and which block acceptance.

Gate output is docs/growatt-protocol.md plus synthetic fixtures and a reviewable
mapping. If evidence cannot resolve a field, preserve it without normalization.
If identity/time remains unresolved, qualify it; do not block all useful evidence
retention or quietly assign UTC. Material contradictions require a plan amendment.

## Gate B — implement bounded acquisition and acceptance

1. Implement context-aware client and discovery from Gate A, with no settings paths.
   Check provider success code even for HTTP 200. Never log provider error bodies.
2. Fetch latest as one unit and history as one provider-local day. Use sequential
   pages initially. Establish first-page count; detect empty pages before completion,
   cycles/duplicates and count mismatch. Bound pages/bytes/rows from verified cadence
   and limits and test exceeding them. Today's moving page set may be inconsistent:
   reject/retry that unit rather than declaring it complete from partial pages.
3. Preserve a day with two equal local-time strings if provider identity distinguishes
   them. Identical same-key content is replay; differing content for the same
   established identity within a fetch is a conflict. Unknown keys require evidence
   associations, not overwriting a time-keyed map. Page position alone is not a
   stable cross-refresh sample identity.
4. Apply Gate A normalization to all records before committing. A known invalid/
   contradictory reading rejects the entire day/latest unit. Preserve old accepted
   records and checkpoint; persist suspect evidence and reason in attempt audit.
5. Empty/unavailable history is not zero production or an instruction to delete.
   Label provider retention limits as observed availability, not a guaranteed
   fixed number of days. Retain local samples after cloud expiry.
6. Integrate one-shot collection through storage/collection contracts, then display
   stored latest/qualified history through task 03 UI/API. Keep measurement and
   retrieval distinct; energy counters are labelled counters, not billing intervals.

## Verification matrix

| Scenario | Expected result |
| --- | --- |
| Two devices / unsupported family / absent family | Explicit selection or actionable unsupported status |
| Disallowed endpoint, redirect or foreign destination | No authenticated request sent |
| HTTP 200 with provider error | Classified failed attempt, no accepted change |
| Missing SOC vs SOC=0; SOC/BMS SOC differ | Null/zero distinction; both fields preserved |
| Verified unit/sign cases and counter reset | Correct normalized values; no invented interval energy |
| Local time vs embedded epoch disagree | Gate-defined quality/rejection, never guessed UTC |
| Spring missing hour / autumn repeated clock | Proper resolution when evidenced; unresolved evidence retained otherwise |
| Page fails, stops early, repeats or changes count | Whole unit rejected/failed, checkpoint unchanged |
| One invalid known field in otherwise valid day | Whole unit rejected; prior records and audit preserved |
| History expires or is empty after prior ingestion | Local history preserved, empty coverage visible |
| Latest and history represent same established sample | Idempotent accepted view; retrieval/evidence provenance intact |

Use injected HTTP and synthetic payloads with explained field meanings. Run unit,
SQLite integration and end-to-end stored UI checks. Private validation verifies
actual discovery and ingestion without invoking settings endpoints.

## Handoff

Publish generic verified protocol/normalization rules, result types, retry ownership,
unknown fields/time states and private validation status. Task 06 schedules these
bounded operations; it must not guess normalization or add independent endpoints.
