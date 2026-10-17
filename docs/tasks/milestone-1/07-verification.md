# Task07 verification

Implementation started 17 October2026 on `feat/task07-release-operations` from
latest origin/main `d56a1af94675737dc0806ec320242524fa5df655`. No applicable AGENTS.md
was found in the repository or ancestors. Rechecked sibling logthing CI/packaging;
see [template evidence](template-evidence.md).

Changes: pinned CI actions and Go1.26.4, offline pinned secret scanner, synthetic
test/vet/race and platform runtime jobs, separate release-only Docker Hub job,
immutable version/build tags, fixed enqueue context inclusion, Compose stop grace,
complete configuration reference and pinned OMV/update/backup/restore procedures.
No schema or collection policy change. Existing version8 preview needs no migration.

Final local runtime checks and independent review are recorded below.
No image/tag/release was published and no private host rollout was attempted.
Only disposable synthetic Docker volumes/images are used for task07 checks. The
existing preview was observed running on its original `/data` mount and is left
untouched. No second process opened its database. No private runtime file changed.

Full Windows `go test -timeout 3m ./...`, `go vet ./...` and both cgo-disabled
production builds passed. Release-policy event matrix passed (PR, pushes, manual,
draft/prerelease/malformed version refused; explicit stable published release
accepted). Native Windows race testing is unavailable without cgo/compiler.
Baseline tests cover final version8 fresh creation, poisoned old migrations skipped,
full fresh/upgrade schema equivalence, reopen, future post-baseline changes and
atomic baseline failure. Added existing-upgrade failure test verifies unchanged
data/schema/version after partial DDL/data changes followed by an error.

Actual HTTP429 shape remains unestablished from retained live evidence. Retry-hint
tests use synthetic responses only. No provider probes/rate-limit experiments run.
Host filesystem/OMV acceptance and real registry credentials/publishing remain
unverified until separately authorized. Local workflow validation does not prove
remote GitHub Actions success.

## Final local results

- Full Go tests and vet passed after the migration-test addition. Native Windows
  race was unavailable; full `CGO_ENABLED=1 go test -race -timeout 5m ./...` passed
  in `golang:1.26.4-bookworm` using only an archived tracked-source snapshot.
- `actionlint` v1.7.7 passed `.github/workflows/ci.yml`; Python release-policy
  matrix passed. Action commits were resolved from upstream v6/v4/v7 tags on
  implementation day, rather than copied blindly from logthing.
- Gitleaks v8.30.1 `dir --redact --no-banner` found no leaks in the tracked-source
  archive. Exported actual Docker context scanning also passed. Gitleaks scans text;
  binary/context safety additionally relies on explicit file allowlisting and
  byte-sentinel/layer checks, not secret pattern recognition inside executable code.
- Both Linux images built with task07-operations build ID. Full cgo-disabled
  storage and collection test executables passed inside each architecture image
  (arm64 via Docker Desktop emulation), including synthetic retry/rejection and
  independent-provider behavior. No provider credentials were supplied.
- `verify-operations.py` passed both platforms: verifies server/enqueue ELF target
  architecture, UID10001, populated records/audit/selected-evidence API, build ID,
  log/API password/provider sentinel exclusion, CA root-pool loading, London summer
  offset, SQLite integrity/foreign keys, graceful SIGTERM exit0, writable mounted
  directory and actionable wrong-ownership startup failure. This tests local CA
  trust loading, not a live-provider HTTPS handshake.
- Replacement on the same disposable volume and clean-stop whole-directory backup
  followed by an isolated restore passed on both platforms. Every row/column of
  every application table was hashed and compared (including samples, intervals,
  aggregates, audit, evidence/retrievals, jobs/checkpoints, schedules/completions and
  aggregate reservations). Populated record/audit/evidence API responses also
  compared identically before/after. No collectors configured, no cloud traffic.
- Context verification initially caught extra files admitted by directory
  exceptions. Tightened `.dockerignore` now exports exactly the four release
  binaries and Dockerfile (BuildKit may consume Dockerfile). Synthetic `.env.*`,
  SQLite, private, cache and backup sentinels are excluded. Saved-image layer
  inspection verifies no sentinel/runtime paths. The enqueue binary is included;
  probe/test binaries are excluded. Existing private files were never edited.
- Independent code-review Standards: no violations/blockers. Spec: one log
  redaction test gap (stderr omitted) fixed; follow-up confirmed resolved. Removed
  old task01 probe assumptions about unimplemented APIs and starting a second
  server against an active DB. Fixed empty-volume ownership fixture to avoid
  Docker copy-up resetting an empty volume's mode before the denial check.

Reproduction: build via `go run ./scripts/build.go -build-id task07-operations`,
build one image per `--platform linux/amd64` / `linux/arm64`, cross-compile storage
and collection with `CGO_ENABLED=0 go test -c`, mount each test executable as
`/probe:ro` and run with `--network none --entrypoint /probe -test.timeout=10m`.
Compile `./scripts/operations-probe` for each target under `bin/probes/<arch>`;
run `python scripts/verify-operations.py --image <image> --platform <platform>
--build-id task07-operations`. Run context/layer verification with
`python scripts/verify-image-context.py --image <image> --scanner <gitleaks>`.
CI contains the exact Linux commands; PowerShell has `scripts/verify-container.ps1`.

## Remote workflow results

Implementation commit `1675267` passed actual GitHub Actions on 17 October2026:
[PR workflow](https://github.com/timmersuk/energything/actions/runs/39044089613)
and [ordinary branch push](https://github.com/timmersuk/energything/actions/runs/39044068039).
Each completed test (full Go suite, vet, release-policy matrix, race and pinned
source scan), docker(amd64), and docker(arm64) successfully. Both platform jobs
built images and passed in-image SQLite/collection, lifecycle/restore, context,
scanner and saved-layer checks. The publish job was **skipped** in both events;
neither registry login nor image push executed. These are actual remote results,
not YAML-only inference. Final evidence-only commits may trigger equivalent reruns.

Draft [PR10](https://github.com/timmersuk/energything/pull/10) remains unmerged.
Release-event eligibility is dry-tested; real release publication, Docker Hub
environment/immutable-tag configuration and OMV host acceptance remain unverified
operator gates. No task08 implementation or private rollout is included.
