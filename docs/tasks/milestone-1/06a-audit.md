# 06a — Durable collection audit REST API and web UI

Status: planning draft. Depends on: 02, 03, 06. Unlocks: 07.
Added by explicit human decision on 14 October 2026. Discord is deferred.
Read: [execution contract](execution-contract.md).

## Objective and boundary

Make repeated source/protocol/validation failures inspectable from retained attempt
history. Audit persists independently of accepted readings and survives restart.
This task exposes the audit produced by storage/collection; it does not implement
notification posting, alerts, incident debounce or a generic logging platform.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/web/audit.go | Authenticated list/detail handlers, bounded filters and JSON |
| internal/web/templates/audit.html | Filterable attempt history, counts and sanitized details |
| internal/web/templates/status.html | Audit navigation/error links from source cards |
| internal/storage/query.go | Indexed audit filters/counts if not already implemented in 02 |
| internal/web/audit_test.go | Privacy, paging, filters, detail and restart-backed checks |
| docs/http-api.md, docs/collection.md | Audit examples, meaning and diagnostic workflow |

## Required event content

Use the shared attempt schema: stable ID, UTC occurrence/start/finish, provider,
stream and masked identity, job kind, requested window/day, terminal outcome,
sanitized error class/code/message, page/retry details, next retry when relevant,
evidence references and accepted row count. Include cancelled/interrupted states
and empty availability; distinguish them from rejected invalid data and network
failure. A fetched old sample never gets a new measurement timestamp just because
its attempt is recent.

An error audit must state what failed without needing raw payload access. Examples:
"interval duration is not 30 minutes", "conflicting values for the same start",
"pagination stopped before reported count". Counts, interval bounds and internal
references are sufficient. Avoid full meter serials, addresses, authenticated URLs,
request headers, exception stacks and private payload text in error messages.

## Implementation checkpoints

1. Check existing attempt producers against the shared schema. Fill missing fields
   through those modules, not through invented UI-only events. Terminal rejection
   audit must already persist even though accepted observations/checkpoint did not.
2. Add GET /api/v1/audit with source, stream, outcome/error class and from/to filters;
   use validated stable newest-first keyset paging, default 100/max 500.
   Return filtered failure/rejection counts over the requested period in addition
   to page rows. Counts must reflect all matches, not just the visible page.
3. Add GET /api/v1/audit/{id} for sanitized attempt detail and evidence references.
   This is not a raw payload download endpoint. Return 404 for absent IDs, including
   IDs from stale cursors, without revealing household identity.
4. Add GET /audit rendering from local query results with the same filter parser.
   Provide range/source/outcome controls, result counts, distinct outcome labels,
   timestamps, masked source identity, requested bounds and concise failure reason.
   Detail expansion can show retry/page/evidence references using escaped text.
5. Link source-card errors to the filtered audit. Display whether failures have
   repeated in the selected period; use factual counts/times rather than inventing
   an alert threshold or diagnosis. Historical errors remain visible after recovery.
6. Show attempt timestamps in configured calendar timezone with absolute/offset
   context in API, and label UTC fields clearly. Repeated autumn clocks must not
   collapse events or produce unstable paging.
7. Document an investigation sequence: inspect repeated class/window, check last
   successful measurement/coverage, inspect bounded retry state; retain evidence
   for private follow-up. Do not recommend blindly erasing DB or forcing full backfill.

## Verification matrix

| Scenario | Expected result |
| --- | --- |
| Invalid reading rejects whole window | Older accepted table unchanged; rejected event visible with source/time/reason |
| Three later failures then success | All attempts retained, filtered failure count=3, recovery success visible |
| Restart with old active attempt | Interrupted event visible; no fake success |
| Multiple pages with equal event times | Stable ID tie-break; no duplicate/missing events |
| Filter provider/outcome/day | Correct rows and counts independent of page size |
| Secret-bearing fake provider error | Secrets absent from API, rendered HTML and routine logs |
| Unknown ID/malformed filter/wrong auth | 404/400/401 safe response |
| Provider outage while browsing audit | No cloud request; local history remains responsive |

Run handler tests backed by real temporary storage, plus browser smoke of rejected,
repeated, empty and recovered attempts. Retain audit/evidence without pruning in
this milestone. No Discord webhook, test-post button or hidden outbound call.

## Handoff

Document exact response field names and evidence-reference behavior. Provide
synthetic query examples and verification results. Task 07/08 must include this
feature in release and private host acceptance; audit is now a completion criterion.
