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

### Growing telemetry day regression

Synthetic regression reproduced history discarded when the provider count increased between pages. Telemetry now follows monotonic increases within existing row/page budgets; decreases, repeated pages, short intermediate pages, overruns and invalid rows still fail. No measurement instants or stable repeated-hour identities are inferred, and no scheduler or quota changes were made. Storage-backed verification confirms accepted rows and per-page evidence survive reopen and remain intact after a later shrinking-count failure. Full Go suite and vet passed.

Retained preview verification: deployed task06a-growing-day with the original data mount, queued one current-day repair through the existing enqueue CLI, and confirmed a successful accepted history attempt. The authenticated samples endpoint now fills its maximum-sized first page with a next cursor. No records, retry ledger or evidence were cleared.

### Remaining paginated retrieval audit

Reviewed every provider retrieval entry point. Synthetic regressions first reproduced growing-count rejection in Octopus consumption, Growatt discovery and Growatt generation, then passed after accepting monotonic growth with final count agreement. Added shrinking-total tests and storage-backed Octopus collection-to-records/audit verification. Existing conflict, cycle, quota and destination tests pass. Octopus per-page evidence now includes typed counts and assembly position; malformed count payloads are excluded. Retained audit inspection found count-change failures confined to telemetry history, with none recorded for Octopus or generation. No other single-response endpoints have the cross-page count assumption.

Full Go suite and vet passed. Rebuilt and deployed task06a-growing-pagination with the original preview data mount, then verified health and authenticated status, audit, Octopus records and generation endpoints. No exceptional repair was required for these paths because no corresponding retained count-change failures were found.

### Canonical source-period schema (user correction)

Schema008 replaces attempt/ordinal sample identity with PRIMARY KEY(identity_id,provider_date,provider_time), where provider_time is the exact reported timestamp, syntax-validated without resolving UTC. Latest/history upsert that same row; corrections replace values. Removed sample_versions, interval_versions and aggregate_versions and their write paths. Missing/malformed periods and conflicting duplicate periods inside one accepted window reject rather than generating identities. Migration collapses repeated legacy periods to the latest accepted retrieval while preserving audit/evidence, schedules and quota reservations.

Storage tests cover unchanged refetch, correction overwrite, reopen, natural primary-key structure, absent version tables and migration from replicated legacy rows. Older migration tests reconstruct the actual old tables. Synthetic browser fixtures now use distinct reported periods rather than fabricated reading keys. Full Go suite and vet passed.

Review follow-up: identical overlapping provider rows now collapse before insertion; differing values for the same period still reject atomically. Both paths have regression tests. The spec review identified this edge case and it was fixed. The standards reviewer also identified it before its session ended; remaining standards checks were completed locally.

Preview upgrade verification: took a clean-stop full private data-directory backup and verified its database hash. Applied migration offline with no collectors; compared canonical contents against latest accepted source rows and compared every row of identities, jobs, schedules, attempts, evidence, retrievals, intervals, daily totals, quota reservations and completions before/after. All matched their expected contents, integrity and foreign-key checks passed, and no value-version tables remain. Synthetic Edge checks passed for date filters, reported readings, details and keyset paging.

Live canonical replay verification: after upgrade, queued one previously duplicated historical day through the existing enqueue CLI. The full provider-day fetch succeeded and its canonical row count remained equal to its distinct reported timestamp count before and after refetch. Recent-day and daily-generation refreshes also succeeded. Final conflict handling uses normal retry backoff and has storage-backed rejection/preservation coverage. Rebuilt/restarted the final preview; clean backup remains private. The remaining days of the requested 90-day detailed window continue through the retained automatic backfill; no completeness claim is made for unrequested days.

### Post-merge closeout and deployment initialization

Synced merged main and inspected retained preview state without starting/restarting collection. Checked reported-day coverage, canonical key uniqueness, generation progress and all retained rate-limit diagnostics/retry records. No private provider identifiers or readings are published here. Retained HTTP429 body/header evidence was not available; historical provider-limit records predate structured hint diagnostics, while later recorded budget deferrals are local spacing decisions.

Fresh initialization now executes an embedded final schema baseline, with historical versions marked satisfied rather than executing their superseded DDL. Existing versioned databases keep their upgrade path, and only post-baseline changes run on future first installs. Tests compare the full fresh and upgraded schema, poison historical scripts to prove they are skipped, verify reopen does not reinitialize, confirm later migrations execute, and verify atomic rollback on baseline failure. Full Go suite and vet passed. Standards and spec reviews reported no blockers. This prepares task07; release publishing and private host rollout remain outside this change.
