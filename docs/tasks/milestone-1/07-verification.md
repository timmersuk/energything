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

Verification results are finalized below after runtime checks and independent review.
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
