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
Initial task07 verification published no image/tag/release and attempted no
private host rollout. A later user-authorized real image push is recorded below.
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
Host filesystem/OMV acceptance remains unverified. Registry authentication and a
real image push were subsequently authorized and verified below. Local workflow validation does not prove
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
immutable-tag configuration and OMV host acceptance remain unverified
operator gates. No task08 implementation or private rollout is included.

## Logthing registry setup alignment

Removed the additional `dockerhub-release` environment gate. Publishing reads
DOCKERHUB_USERNAME/DOCKERHUB_TOKEN from GitHub repository Actions secrets, as
logthing does; DOCKERHUB_IMAGE is the repository variable `timmersuk/energything`.
Verified those names and the image variable through GitHub without reading secret
values. No environment or environment approval is required. Corrected token docs:
a personal Docker Hub Read & Write token is not limited to one image by its name.
Release-only eligibility, version/build tags, pinned actions and both platform
gates are unchanged. No release, image publication or preview update was performed.
Validation: actionlint v1.7.7, the Python release-policy event matrix and
`git diff --check` passed. This correction changes workflow credential lookup and
documentation only; application/container runtime code is unchanged.

## Authorized real Docker Hub push

The user explicitly authorized a real push after the initial no-publish task.
GitHub repository secrets successfully authenticated using the same pinned
docker/login-action as the release job. The same pinned build-push action built
and pushed linux/amd64 and linux/arm64 together to `timmersuk/energything`.

- Tag: `verify-sha-107de72aefbca4a91f479bcb6774657ab56e6619-run-39065834529`
- Index digest: `sha256:4423932cce00ef6c1398b44c352b8af6b0000a6406ee27f94afe2273961d3564`
- amd64 manifest: `sha256:5d588f29bc50f1b7a9bc617fc2357717068445b7017d34d5ece991dfa7ecd083`
- arm64 manifest: `sha256:71539b2d06e98fac46e052b62da7ee062dcc283de42c4ffb5cc105dbdd27951d`

[Push run](https://github.com/timmersuk/energything/actions/runs/39065834529)
passed tests, authentication and the actual multi-platform push. Its subsequent
amd64 pull/runtime check passed; pulling arm64 under the same index reference then
failed with the runner's classic image-store `cannot overwrite digest` limitation.
This was a post-push verification failure, not a failed registry push.

[Follow-up verification](https://github.com/timmersuk/energything/actions/runs/39066011317)
passed by pulling each published platform manifest digest independently, with no
additional image push. Both registry-pulled images passed the complete synthetic
operations probe: correct binary architecture/build ID, UID10001, CA/timezone,
SQLite, authenticated records/audit/evidence and redaction, graceful termination,
replacement, stopped full-directory backup/restore and wrong-ownership rejection.

The temporary branch-only verification workflow was removed after success. The
normal release workflow, repository secrets and existing image tags are unchanged.
The unique verification image remains as evidence. No Git tag/GitHub release,
merge, preview update or private host deployment was performed. Versioned release
event gating remains synthetically tested; actual registry auth/push/pull and both
published architecture runtimes are now verified, rather than inferred.
