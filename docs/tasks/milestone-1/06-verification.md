# Task06 verification — 15 October2026

Status: implementation verified for review; whole-history correction below supersedes the original limited-search policy.
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

- Final amd64 and emulated arm64 images passed non-root UID, CA/timezone files,
  real SQLite schema initialization, authenticated status, repair CLI execution,
  clean SIGTERM and persisted database restart via scripts/task06-runtime.ps1.
- Actual-image Edge checks passed recent-refresh enqueue, CSRF rejection, durable
  schedule status, exact retained Octopus/generation browsing during provider
  failure, mobile status and zero external browser requests. Both provider hosts
  were mapped to container loopback, so synthetic credentials reached no real cloud.
  Fixture selection was restored with a helper **inside Linux**, after the injected
  discovery failure, to model retained selected contexts during an outage. This
  fixture validates local queue/UI behavior, not successful live cloud collection.
- Preview cleanly stopped, full original directory backed up, migrated and recreated
  from the exact saved Compose pair/project/mount. Actual image ID, task build,
  health and nondecreasing counts for every retained identity passed. Automatic
  collection intentionally starts with existing selections and the retained quota
  ledger. The pinned Alpine base is unchanged. Private browser build/status and
  retained Octopus/generation browsing passed; generated credential-bearing code
  and output were removed.

Restricted-loopback attempts and an early stale migration fixture failure were
corrected before the final passing checks. Docker internal-network publishing did
not expose the synthetic port, so the fixture uses explicit loopback host mappings.
A host-Windows helper could not safely share live Linux WAL visibility; that failed
synthetic fixture was replaced with a fresh isolated database and all live helper
mutations moved inside Linux. Original preview data was never opened by that host
helper. Use docker exec energything-enqueue for a container server's repair.
Synthetic screenshots and all private backups/evidence remain ignored under.cache.

## Contracts and unresolved gates

See [collection](../../collection.md) for exact schedules, retry budgets, independent
progress and repair syntax. Historical acquisition now continues into older calendar
years without the rejected120-day stop. Annual targets are durable planning units;
empty/zero responses do not terminate scanning. Unknown Growatt source-lifetime
metadata does not block backwards progress or become a false archive-complete
claim. Octopus retains its verified explicit-domain oldest probe and weekly recheck.

Current installation BST evidence supports UTC+1, but regional seasonal behavior
and repeated-hour sample identity remain unresolved. New acquisitions retain
regional and fixed-offset candidate interpretations, original clocks and evidence;
no old rows are rewritten. Existing post-transition automation remains unchanged.
Generation agreement with daily counter maxima and lifetime-counter difference
cannot select UTC/BST when shifted boundary hours have zero positive PV.

The pre-existing untracked generation comparison note and ignored private evidence
are preserved. No private identifiers, credentials, raw responses or screenshots
are included in public artifacts.

## Review

Standards: no hard documented-standard violations. One heuristic duplication of
unit limits was addressed with shared collection.UnitSpec for scheduler/repair.
Spec: generation/device-selection coupling and permanent discovery retries were
fixed. Tests now cover ambiguous/unsupported/failing device discovery beside a
confirmed plant, serial-only selection across multiple plants, permanent pause,
long discovery deferral and older newly discovered boundary after a limited scan.
No task06a implementation or changes to the existing autumn automation were made.

Transient telemetry discovery recovery was also reviewed and fixed: independent
retry due state lets generation continue, then bootstraps telemetry without a
server restart. Its fake-clock regression and long-deferred-discovery reopen test
passed. Final focused spec re-review reported no additional actionable findings.

Final private preview metadata also confirmed automatic plans are present, generation
has independent aggregate plans, both Octopus oldest boundaries were observed by
the running server, and source attempts remain visible. Public verification output
contained booleans only. Final preview browser recheck passed after the last image
replacement. No exhaustive live archive completion is claimed.

## Whole-history correction verification

The user rejected the120-day stop. New tests passed for continuous acquisition
through a full empty/zero year into older positive generation, year-boundary
rejection preservation, restart without replay/skipping, automatic continuation,
and representable-date underflow protection. A real schema006-to007 migration test
passed: a formerly stopped plan resumes at the identical frontier, preserving
accepted aggregate/version/checkpoint and durable quota reservations.

The remaining whole-suite, image, migration/preview and browser checks for this
correction are pending; older executed results above describe the prior revision.
