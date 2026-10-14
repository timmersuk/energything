# 07 — CI, release image and operational readiness

Status: planning draft. Depends on: 01–06 plus 06a. Unlocks: 08.
Read: [execution contract](execution-contract.md), [template evidence](template-evidence.md).

## Objective and boundary

Make the implemented collector/audit application reproducibly releasable and safely
operable as one non-root OMV container. This task builds and documents release
mechanisms; publishing and private host rollout occur only when explicitly authorized.
No public workflow may access household data or private deployment credentials.

## File responsibilities

| Path | Responsibility |
| --- | --- |
| .github/workflows/ci.yml | Synthetic tests, Go build, container matrix and controlled publish |
| Makefile, Dockerfile | Reconcile packaging from 01 with final runtime and build ID |
| docker-compose.yml, .env.example | Final documented process/Compose configuration |
| .gitignore, .dockerignore | Runtime secrets/data/evidence exclusions |
| README.md, docs/operations.md | Build, deployment, pinned updates, permissions, recovery |
| docs/configuration.md, docs/http-api.md | Complete names/defaults/error behavior and audit API |
| Test support | Synthetic container replacement/backup/restore checks |

## Implementation checkpoints

1. Compare logthing's CI again at implementation time. Port its test→docker jobs,
   build ID, QEMU/Buildx, platform binary layout, metadata and Docker Hub convention.
   Remove Node/pnpm/frontend steps and unrelated binaries. Pin the actual Go version
   established in 01 and reviewed action revisions; do not blindly copy old majors.
2. PR and ordinary branch builds run go test ./..., build embedded Go assets,
   exercise SQLite tests and build images without registry login/publish. Fork PRs
   must receive no registry secrets. Use narrow default read permissions and an
   explicit release-only publication condition. Manual dry builds never imply push.
3. Versioned release/tag builds publish to the configured Docker Hub energything
   image using repository-scoped credentials. Registry credentials are separate
   from household provider credentials. Define immutable version tags and a build
   ID; docs use pinned version/digest updates rather than depending on latest.
4. Build both linux/amd64 and linux/arm64. Verify actual target-platform binary COPY,
   cgo-disabled SQLite operation, CA trust, timezone, graceful SIGTERM and non-root
   /data access. Include full Go test/race checks where supported by the runner;
   race runtime build requirements are separate from the production cgo-disabled build.
5. Add secret scanning using a documented maintained scanner pinned during
   implementation. Scan tracked source/fixtures/build context; no live secret
   verification or household cloud requests. Validate synthetic credential sentinel exclusion (useful synthetic identifiers remain visible)
   in logs/API and inspect image layers/context for excluded files.
6. Finalize placeholders and config docs for all scaffold/provider/backfill/range
   refresh variables. Explain independent unconfigured/provider errors and fatal
   global/storage errors. Compose has no default login/password or provider values.
7. Document OMV host local directory, ownership matching UID 10001, writable SQLite
   companion files, one instance per DB, port choice, restart policy and LAN/HTTPS
   access. Use generic host path placeholders, never discovered private paths.
8. Document pinned image update: record current version; stop/start replacement
   against the same data mount; inspect liveness, authenticated status, audit and
   resumed collection. Do not promise rollback across irreversible migrations;
   restore a matching prior backup/image when schema incompatibility requires it.
9. Document supported backup: stop application, verify exit, copy the whole data
   directory (including companions), retain image/schema version metadata, and
   restart. Restore into an isolated local test directory with the matching image,
   verify records/checkpoints/audit, then make it available for a controlled rollout.
   Never advise copying only live energything.sqlite.
10. Document database-size/free-space inspection and storage-failure behavior.
    Retain all selected evidence and audit initially. Disk pressure prompts review;
    the application must not silently prune or aggregate to recover space.

## Verification matrix

| Check | Expected evidence |
| --- | --- |
| PR workflow | Synthetic tests and images pass; publish/login condition false |
| Release condition dry evaluation | Only explicit versioned release/tag can publish |
| Both platform images | Correct binary, SQLite write/reopen, timezone and TLS trust |
| Non-root mounted directory | Writable succeeds, wrong ownership gives clear failure |
| Synthetic provider retry/rejection | Collection and audit behave identically in image |
| Container replacement with same mount | Observations, evidence, checkpoints and audit retained |
| Clean-stop backup and isolated restore | Same records/progress/audit, no DB repair required |
| Secret scan and image context/layer inspection | No private/runtime files or sentinel secrets |
| Provider outage after startup | UI/audit/local reads work; healthy provider continues |

Run only needed checks once the final changes pass. Record tools/commands/results
and any unavailable architecture/CI capability. An unrun remote workflow is not
verified merely because its YAML parses. Never create a real release/tag or post
messages as a side effect of testing this task.

## Handoff

Supply reproducible release/deploy/backup/restore procedures, version/schema
compatibility notes, UID/data requirements, published configuration/API docs and
actual verification status. Host acceptance remains a separate task; CI green alone
does not finish milestone 1. Discord remains absent from config/image/docs.
