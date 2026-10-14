# Task 05 verification and handoff

Implemented on 14 October 2026 on `task05-growatt`, based on clean main at
36545d6 (tasks 01–04 merged). No pre-existing uncommitted work was present.
All implementation edits are in energything. Only generic logic in the two
specified sibling scripts was read; no sibling script/configuration/control
endpoint was executed. Task-created public documentation reference copies briefly
landed in the sibling cache through an omitted working-directory argument and
were moved into energything; no existing sibling files were changed.

## Files and contracts

- `internal/growatt/{client,discovery,history,normalize}.go` and tests: four-endpoint
  exact read allowlist, family-5 SPH/MIX discovery, bounded sequential page assembly,
  exact scalar values, optional fields, selected evidence and explicit time/unit quality.
- `internal/collection/growatt.go` and tests: typed discovery and one bounded unit,
  BeginAttempt, accepted transaction, failure/rejection/cancellation audit and deferred
  due/pause state. No timer or scheduler. Existing Octopus behavior is preserved.
- `cmd/growatt-verify`: explicit cancellable CLI with private candidate report,
  latest plus optional one-day verification and separate private database. No
  Growatt ONCE flag or required backfill-start environment variable.
- `internal/storage`: migrations 003 (Growatt discovery/attempt diagnostics), 004
  (safe discovery failure context); typed Diagnostic/DiscoveryFailure, queries and
  atomic selection. Migration tests updated for real prior-schema upgrades. Observed
  Growatt valid/qualified counts no longer require an invented expected cadence;
  Octopus and explicitly established expected-count arithmetic remain validated.
- `internal/web/status.go`, status/Growatt templates: full identifiers, useful typed
  discovery/attempt diagnostics and clear power/counter/unresolved-unit labels.
  Existing HTTP routes and stored query contracts are preserved.
- `scripts/task05-fixtures`, browser/preview scripts: separate synthetic database,
  actual-image browser verification, exact saved preview setup and private browser
  runner that suppresses/removes credential-bearing generated code/output.
- README, data-semantics, HTTP API, operations, execution-contract and
  `docs/growatt-protocol.md`: protocol gate, configuration, storage and later-task handoff.

Gate A was documented before normalization against Growatt_API's current official
ShineServer pages, linked in the protocol document. Device type exactly 5 establishes
SPH/MIX, with unique selection or private CLI ambiguity report. MIX basic-details
exposes settings and is deliberately excluded. The only cloud paths are plant/list,
device/list, mix_last_data and mix_data. No settings/control reads/writes occurred.

Only SOC `%` and total PV input `W` units are established. BMS SOC remains distinct;
other named power/counter values retain exact decimals and unresolved units/signs.
Unknown telemetry and original clock/calendar/epoch fields remain selected evidence.
Provider calendar serialization does not independently prove measurement UTC or
stable observation identity. All current adapter samples have nullable instant/key
and explicit unresolved/missing/contradictory quality. Repeated autumn clocks remain
separate occurrences; latest/history replay cannot claim stable-key idempotency.

Gate B: exact HTTPS host/method/path/form validation before token, redirects disabled,
30s requests, 4 MiB/page, 100/page, 100 pages and 10,000 assembled-row budgets. History
is one provider day with stable reported count, complete assembly and whole-unit
validation before acceptance. Empty does not erase data or imply zero production.
Client alone owns at most three HTTP attempts/page, ordinary 1s/2s plus <=250ms jitter;
long Retry-After defers instead of sleeping, provider 10012 defers five minutes.
Authentication/configuration pauses; ordinary unit failure gets one-minute next_due.
No component executes scheduled reattempts. Useful structured field/value/count/HTTP/
provider-code context is safe and owner-visible. Failed failure-audit writes explicitly
claim no durable terminal completion.

## Checks actually run

| Check | Result |
| --- | --- |
| Synthetic injected HTTP tests | Passed: discovery ambiguity/unsupported family, destination rejection/redirect refusal, HTTP200 provider error, optional/null/zero/distinct SOC/BMS, exact exponent decimals, known invalid values, unknown evidence/privacy, unresolved spring/autumn clocks, page count change/early stop/repeated page/missing structure/body and row budgets, retries/deferred waits/cancellation |
| Real SQLite collection integration | Passed: accepted/rejected/empty/reopen, old samples/checkpoint preserved, suspect evidence and terminal audits, oversized-page diagnostic >10,000 durably retained, discovery HTTP404 context stored without response text |
| `go test ./...` | Passed after final review fixes |
| `go vet ./...` | Passed after final review fixes |
| `go run ./scripts/build.go -build-id task05-growatt` | Linux amd64/arm64 CGO-disabled builds passed |
| `go build -o bin/growatt-verify.exe ./cmd/growatt-verify` | Passed |
| Docker task images amd64/arm64 | Built with unchanged approved pinned Alpine base |
| Storage and collection probe executables in actual non-root Alpine images | Passed amd64 and emulated arm64; includes real migrations, transaction rollback, SQLite busy timeout/write denial/SQLITE_FULL and provider collection integration |
| Synthetic Edge actual-image browser | Passed: build task05-growatt, full identifiers, exact values, null time/unit/zero contracts, useful HTTP404 discovery diagnostics, failed attempts preserving prior acceptance, Octopus browsing and no outbound browser cloud requests |
| Private read-only verification | Passed discovery/latest/full provider-day fetch, validation and accepted audit/evidence in separate ignored private database; final schema reopen/qualified occurrence checks passed |
| Exact preview Compose `config --quiet` | Passed using both saved files, project energything |
| Clean preview backup/recreate | Passed; original preview-data mount, retained counts, health200, task05-growatt build and disabled Octopus one-shot confirmed |
| Private actual-preview Edge check | Passed build/changed Growatt UI/Octopus browsing; real credentials/identifiers were not printed |
| `git diff --check` | Passed after trimming accidental template trailing blank lines |

Private runtime credential presence was checked without reading/displaying .env
values. Discovery found a unique eligible device; real identities/payloads and
accepted private database remain ignored, not copied into fixtures or repository.
A full latest/day acquisition initially used an earlier CLI binary and failed its
storage coverage commits. Its failure audits were retained. An immediate repeat
hit provider frequency limitation and was not counted as successful. After deferral,
the corrected CLI accepted latest and all day pages; final stored checks verified
both accepted attempts and evidence associations. No archive/retention-boundary or
current-moving-day completeness claim is made from that bounded historical day.

The first browser attempt used unavailable Chrome; rerun in installed Edge passed.
The first private preview browser runner lacked a Node require binding; corrected
private generation/output suppression passed. Restricted Docker/Go/Git calls were
rerun with approved tool access. These earlier failures are not verification passes.

## Review and unresolved gates

Parallel code-review skill reviews against main found two spec gaps (overrun audit
bounds, lost discovery diagnostics) and one standards contract gap (lost unexpected
HTTP status), plus custom error-join and duplicated paging judgment smells. The
three functional gaps were fixed and regression-tested; errors.Join replaced the
custom helper. Discovery and telemetry paging remain separate because telemetry
must retain suspect page evidence while discovery must exclude account/location
responses; common destination/retry/count conventions remain documented/tested.

Remaining protocol uncertainty is explicitly retained, not hidden: absolute
measurement time/stable cross-refresh identity and units/scaling/signs of most MIX
fields need stronger authoritative evidence. No instant/unit/key is invented.
Private validation proves this bounded day worked, not source-lifetime availability.
Task 06 owns automatic collection, resumable historical boundary search and a true
cross-process repair-enqueue CLI; task 06a owns full audit/evidence routes. Neither
was implemented. Actual ARM64 hardware and OMV acceptance remain later deployment gates.

Preview schema upgrades were backed up cleanly each time. The private verification
DB and synthetic browser DB are separate from preview; preview's existing data was
preserved and no fixture/private-db migration into it was performed.
