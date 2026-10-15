# Task06 verification — 15 October2026

Status: implementation under verification; **historical source-boundary gate open**.
Not a completed task06/archive claim. Task06a has not been started.

## Checks actually run

- Full `go test ./...` passed with loopback/subprocess permissions.
- `go vet ./...` passed.
- CGO-disabled Linux amd64/arm64 builds passed via scripts/build.go, task build
  `task06-scheduling`; enqueue Windows build passed.
- Real SQLite migration/reopen, restart after accepted empty history, older data
  beyond an empty gap, finite search stopping, routine priority/coalescing/due
  progression and independent identities passed synthetic tests.
- Separate-process repair enqueue preserved a running server attempt. Same operation
  ID/range coalesced; changed bounds rejected. Queue open performs no migration or
  startup recovery. Immediate SQLite transactions serialize writer claims.
- Refresh tests passed auth, method, missing/foreign CSRF token, same/foreign/null
  Origin, malformed/oversized/extra-field body and accepted enqueue paths.
- Existing HTTP fixture tests cover retry/deferred quota, partial-page rejection,
  correction/evidence preservation and cancellation. New clock tests retain all
  autumn candidates and identify spring gaps without fabricated IDs or instants.

Image/runtime/browser and preview verification are still pending in this revision.

## Contracts and unresolved gates

See [collection](../../collection.md) for exact schedules, retry budgets, independent
progress and repair syntax. Historical exploration is anchored to a120-calendar-day
engineering budget, with explicit `search_limit_boundary_unresolved` state. This is
not a verified supported source lifetime domain or an exhaustive archive. Empty
windows and aggregate zeros do not terminate search. Octopus omitted-date oldest
ordering is not yet proof of all-time semantics; Growatt needs a supported finite
calendar domain or authoritative retention/earliest metadata. These are concrete
remaining protocol gates, not operator requests to invent historical dates.

Current installation BST evidence supports UTC+1, but regional seasonal behavior
and repeated-hour sample identity remain unresolved. New acquisitions retain
regional and fixed-offset candidate interpretations, original clocks and evidence;
no old rows are rewritten. Existing post-transition automation remains unchanged.
Generation agreement with daily counter maxima and lifetime-counter difference
cannot select UTC/BST when shifted boundary hours have zero positive PV.

The pre-existing untracked generation comparison note and ignored private evidence
are preserved. No private identifiers, credentials, raw responses or screenshots
are included in public artifacts.
