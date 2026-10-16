# Task06a verification

Implemented 16 October 2026 on feat/task06a-audit, based on latest origin/main
14107f3. No applicable AGENTS.md was present in the repository or its ancestors.

- Storage-backed handler checks: auth/methods/no-store, list/detail, bounded and
  malformed filters, full-match counts across pages, equal-time ID ordering,
  repeated failures then recovery, accepted empty, rejection reason, full synthetic
  identity, retained unknown evidence fields, escaped JSON/HTML, download headers,
  cross-attempt retrieval 404, missing IDs and restart interruption.
- Full `go test ./...` passed with local networking available. Initial sandbox
  run could not deliver loopback requests in the existing server shutdown tests;
  the rerun passed unchanged. `go vet ./...` passed.
- No schema changes; audit browsing uses already-open typed local read queries.
  Existing history-start policy, scheduler, quota reservations and qualified clocks
  are unchanged. Public fixtures contain only synthetic identities/telemetry.

Browser, final review and preview update results are recorded below after execution.

Final review (code-review skill, separate Standards and Spec reviewers): initial
Standards result had no hard violations and one low-priority broad-status-query
concern. Initial Spec result identified generic Octopus pagination/structure codes
and missing count context. These were addressed with enumerated producer reasons,
expected/observed diagnostic persistence through collector/API tests, and a narrow
transactional generation progress projection reading the exact current job rather
than capped/unrelated status history. Existing historical audit rows are unchanged.

Browser checks used Playwright CLI with installed Edge and a newly generated
synthetic fixture served on 127.0.0.1:8086 without collectors. Passed combined
source/stream/outcome/error-class/date filters, counts independent of one-row pages,
next-page uniqueness, rejected evidence expansion as text (script did not execute),
JSON attachment and mismatched retrieval denial, success/recovery, empty, interrupted,
cancelled and empty filtered results. The synthetic January/August fixture displayed
January rows alongside the August frontier, 2022 configured boundary, stored quota
deadline and filtered quota navigation. Visual screenshot inspected; only the
expected unimplemented favicon 404 appeared in console.

Final full Go suite and vet passed after review fixes. Storage tests also cover
unsafe/corrupted/oversize historical evidence failing closed and deduplicated content
not permitting cross-attempt retrieval access.

Follow-up Standards and Spec reviews both confirmed the findings resolved, with
no remaining blockers. Targeted storage/web/Octopus tests passed again.

Preview update executed with build ID `task06a-audit`, CGO-disabled Linux amd64 and
arm64 server/enqueue binaries, rebuilt energything:local, and the exact saved
Compose project/override. Recreated energything-preview on the original
D:\projects\energything\.cache\preview-data:/data bind mount. No schema upgrade,
new data directory, provider repair, private configuration edit or ledger reset.
Health returned ok; authenticated status reported task06a-audit. Authenticated
audit list/HTML and daily-generation progress HTML passed. Retained row count
increased during normal collection; fetched-window and schedule counts did not
fall. Private count-only summaries are ignored under .cache; private identifiers,
amounts, evidence, credentials and runtime tokens were not published.

Follow-up: the Growatt samples web page defaulted to absolute measurement time,
which intentionally excluded retained unresolved-clock samples. A storage-backed
regression reproduced the symptom (`quality_selected=false empty=true`) before
the fix. The HTML route now defaults an omitted view to quality/retrieval, with
an explicit range-basis explanation. Explicit absolute view and the v1 API default
remain unchanged. No generation quota, observations or clock normalization changed.
Full Go suite and vet passed after the page-default follow-up. Edge browser checks
confirmed default quality selection, retained sample visibility, quality paging and
explicit absolute-time selection. Preview rebuilt as task06a-samples-default on its
original data mount; authenticated default page showed retained samples and health
passed. The generation cap was not changed.

Quota follow-up: the user explicitly requested a boolean flag with the ten/day
cap disabled by default. Implemented GROWATT_AGGREGATE_DAILY_CAP_ENABLED=false;
true restores the existing cap. No numeric-limit setting or bounded-experiment
runner was retained. Synthetic tests cover eleven reservations with the cap off,
five-minute spacing, restart retention, cap restoration and provider deadlines
remaining unchanged when only local-budget deferrals are reconsidered.
Full Go suite and vet passed for the boolean flag. Preview rebuilt with the cap
disabled by default; retained data mount and five-minute reservation ledger kept.

429 follow-up: added bounded selected-body retry-hint parsing shared by Octopus
and Growatt, preserving Retry-After seconds/date and taking the latest valid hint.
Growatt 10012 now respects longer hints while keeping its five-minute fallback.
Synthetic coverage includes conflicting body/header hints, malformed/oversize/read
failures, unsafe message/key/header exclusion, no aggregate inline retries,
collector-to-audit/API/UI persistence and restart durability. No deliberate live
rate-limit probe was sent, and the daily-cap flag remains false by default.
Full Go suite and vet passed for retry hints. Additional parser tests passed for
invalid offsets and rounding beyond the supported timestamp domain. Preview
updated to task06a-retry-hints on the original mount; authenticated audit/status
and default-off daily-cap policy verified after recreation.

Reading-date UI follow-up: the screenshot selected a wide retrieval range, whose
ascending first page showed historical samples. Normal samples browsing now uses
inclusive Growatt-reported reading days and newest reported clock first, with
quality/source internals under advanced diagnostics and no duplicate single-date
input. API default and explicit retrieval/absolute diagnostic modes stay compatible.
Storage-backed tests cover reading/retrieval dates differing, broad-range first
page, malformed clock fallback, equal-clock stable paging, cursor/filter binding
and no invented UTC instant.
Full Go suite, vet and Edge checks passed for reading-day browsing. The synthetic
browser verified hidden quality internals, removal of the duplicate date box,
broad-range newest-first display, reading versus retrieval date separation and
paging mode retention. Preview task06a-reading-days now shows today's readings
first for the screenshot's wide range on retained data. The initial implementation
scanned historical payload pages; using the existing quality/date index and reading
only relevant day clock keys reduced that local check from about12.7s to1.35s.
No migration or retained data rewrite was required.

### Samples page simplification

Removed the timestamp qualification on every row, diagnostic view selector, quality/source controls and raw timestamp evidence from the normal samples page. Retained reported reading-day filtering, newest-first paging, exact reading details and the note: Times shown as reported by Growatt. Diagnostic data remains available through the API and collection audit. This supersedes the earlier advanced-controls UI description.

Validation: full Go suite and vet passed; synthetic Edge verification passed for date filtering, filtered paging, exact details and absence of diagnostic clutter. Rebuilt preview as task06a-simple-samples with its existing data mount preserved and verified the authenticated page.
