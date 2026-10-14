# 03 — Authenticated stored-record REST API and browsing

Status: planning draft. Depends on: 02. Unlocks: provider UI integration in 04/05.
Read: [execution contract](execution-contract.md).

## Objective and boundary

Browse synthetic import/export and Growatt history through bounded authenticated
REST queries and template pages. All reads are local. Refresh/jobs belong to 06;
attempt audit pages/endpoints belong to 06a. No cloud calls or charts here.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| internal/web/query.go | Parse dates/ranges, direction, quality and bounded cursor pagination |
| internal/web/records.go, status.go | Storage query calls and versioned JSON envelopes |
| internal/web/templates/status.html | Source cards and operational state |
| internal/web/templates/octopus.html, growatt.html | Tables, filters, pagination and quality states |
| internal/web/static/ | Minimal CSS/JS; optional local vendored library only if needed |
| internal/web/*_test.go | Auth, query bounds, serialization, escaping and zero-cloud behavior |
| docs/http-api.md | Implemented routes, parameter examples, response fields and errors |

## Implementation checkpoints

1. Implement GET /, /records/octopus, /records/growatt, /api/v1/status,
   /api/v1/octopus/intervals and /api/v1/growatt/samples. Preserve task 01 auth
   and public /healthz. Do not expose unimplemented audit/refresh endpoints as success.
2. Use one query parser for pages and API. Date-only UI ranges are inclusive
   calendar dates translated to half-open instants in configured Europe/London.
   API explicit instants require offsets. Reject missing one bound, reversed ranges,
   mixed formats and malformed cursors. Default view is the current local day;
   navigation changes stored range, not collection policy.
3. Bound each response to 100 rows by default and 500 maximum. Order normal records
   by instant and stable identity/key; cursors bind to filters. Parameter examples
   must state whether to is exclusive for instant queries. Return next_cursor only
   when another page exists. New arrivals cannot cause offset-based duplicate paging.
4. Return decimal kWh as strings and nullable values as JSON null. Include source,
   full provider identity, measured/retrieved times, unit and quality. Do not return account
   responses, credential values or raw evidence through normal record endpoints.
5. Status shows configuration/discovery state, selected full provider identities, running/
   backfill state, attempts/successes, newest measurement, retained count and coverage.
   A recent retrieval of old data does not advance measurement time. Show the fact
   of missing data without claiming outage/meter-lifetime causes.
6. Resolved Growatt rows support absolute-time query. Unresolved observations use
   a labelled separate view with provider-date/retrieval filters and original time
   text. State how many are excluded from normalized queries. Do not sort ambiguous
   local text as if it were absolute time or force it into the wrong day.
7. Render provider text through html/template. Exclude credentials; retain useful full provider identifiers
   in view models. Build empty/loading/partial/unconfigured/error states from stored
   state; a page render cannot enqueue a refresh.
8. Document exact API field names and query examples before provider tasks extend
   them. Keep HTTP behavior stable; no generic CRUD routes or browser framework.

## Verification matrix

| Check | Expected result |
| --- | --- |
| No/wrong auth on every route/asset | 401 without source data |
| Empty database | Useful empty cards/tables; no fabricated zero records |
| Exact kWh, SOC null and SOC zero fixtures | Correct string/null/zero output and units |
| March/October London days | Correct 46/50-slot bounds; repeated hour distinguishable |
| Malformed range/page size/cursor | 400 safe error; no unbounded query |
| More than one page, equal timestamps | Stable paging without duplicates/omissions |
| Provider text contains HTML/script | Escaped text, no execution |
| Provider transport stub errors if called | All browsing still succeeds; stub call count is zero |
| Old sample fetched now | Both times visible; no false freshness |
| Unresolved sample | Visible in quality view; excluded/count-labelled in absolute query |

Run handler tests with real storage fixtures and browser smoke checks against the
actual task build. Do not add a production demo-seeding endpoint. Temporary synthetic
seeding belongs to test support, cannot expose private fixtures, and is documented
for reproducible UI verification.

## Handoff

Record route/query/response contracts and actual browser verification. Later tasks
populate status/records through storage rather than patching handlers with provider
HTTP calls. Human visual judgment, if needed, must use this task's running build.
