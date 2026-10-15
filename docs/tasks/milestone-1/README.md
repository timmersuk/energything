# Milestone 1 execution briefs

Status: tasks 01–04 are merged; task05 implementation/research is in PR #5.
Task05a aggregate-ingestion scope is agreed; implementation has not started.
Updated 15 October 2026. Read individual verification handoffs for actual gates.

Parent: [Milestone 1](../../plans/milestone-1-service-and-collection.md).
Direction: [Staged delivery](../../plans/goal.md).

## Start here

Read the [shared execution contract](execution-contract.md), the selected task,
and its dependency handoffs. The shared contract defines configuration, module
ownership, observation/window semantics, audit, HTTP routes and recovery. Each
brief supplies files/roles, ordered checkpoints, concrete verification and gates.
Do not treat task titles or this index as sufficient execution instructions.

[Logthing template evidence](template-evidence.md) records inspected deployment
conventions. Workbench task/context/plan examples informed the specificity of these
briefs; their workflow implementation and unrelated architecture are not adopted.
All energything edits belong in this repository; sibling repositories are references.

## Relation to the overall plan

This milestone establishes retained qualified observations and operational trust.
Milestone 2 can then calculate usage and tariff value from precise meter intervals;
milestone 3 can interpret solar/battery patterns from qualified Growatt history.
Neither later milestone can safely rely on invented zeros, guessed UTC or overwritten
rejected data. Milestones 4/5 remain separate suggestions/control decisions.

No gas, rates, money, charts, optimiser, inverter writes, local Growatt adapter,
private-history import or Discord posting is included. Evidence/audit remains
uncompacted initially; retention is revisited using observed storage growth.

## Task sequence and dependency graph

| Task | Deliverable | Requires |
| --- | --- | --- |
| [01 Scaffold and build gate](01-scaffold.md) | Embedded/authenticated empty application; compatible SQLite/runtime; logthing-shaped Docker build | None |
| [02 Durable storage](02-storage.md) | Versioned schema; accepted/rejected transactions; evidence, progress and audit; bounded queries | 01 |
| [03 Stored REST API and browsing](03-browsing.md) | Stable record/status contracts, local tables and qualified-time views | 02 |
| [04 Octopus](04-octopus.md) | Confirmed import/export discovery, bounded validated ingestion and audit | 02, 03 |
| [05 Growatt](05-growatt.md) | Verified protocol/time/unit gate; read-only latest/history ingestion and audit | 02, 03 |
| [05a Growatt generation aggregates](05a-growatt-aggregates.md) | Separate retained provider generation periods, bounded ingestion and authenticated table/API | 02, 03, 05 |
| [06 Scheduling and recovery](06-scheduling.md) | Durable independent jobs, bounded backfill/retries and secure refresh enqueue | 04, 05, 05a |
| [06a REST/UI audit](06a-audit.md) | Inspectable attempt history, source/time/reason filters and repeated-failure counts | 02, 03, 06 |
| [07 Release and operations](07-release-operations.md) | Synthetic CI, reproducible images, privacy review and backup/restore procedure | 01–06, 05a, 06a |
| [08 OMV acceptance](08-omv-acceptance.md) | Actual automatic collection, audit, independent failures and persistence/restore verified | 07 |

04 and 05 have no logical dependency on each other, but share reviewed storage/API
contracts. This is a dependency statement, not authorization to spawn agents.
The 05a task adds the agreed historical aggregate baseline before scheduling.
The 06a name preserves existing task filenames while adding the agreed audit scope.

## Decisions settled in the interview

- Use logthing deployment conventions: Linux amd64/arm64, Docker Hub, non-root,
  local /data bind mount and Compose restart unless-stopped.
- Initially retain all selected telemetry evidence and superseded versions;
  identical payloads can share storage, without losing retrieval/attempt history.
- Invalid or contradictory data rejects the whole bounded fetch window, preserving
  accepted data and its completion checkpoint. A changed valid reading remains a
  legitimate correction; an inconsistent window is not partially accepted.
- Retain timestamped source/window-attributed audit and suspect evidence, accessible
  via authenticated REST/UI so repeated issues remain observable after recovery.
- Defer Discord alerts. No webhook configuration or outbound notification work.

## Implementation gates rather than questions for the human

| Gate | Owner | Passing evidence |
| --- | --- | --- |
| Go/SQLite build/runtime compatibility | 01 | Exact pinned versions and executed Linux platform probes |
| Storage record/commit/query contracts | 02 | Documented types/schema and synthetic transaction/reopen proofs |
| Octopus endpoints/boundary/selection semantics | 04 | Reviewed generic reference logic plus synthetic/private read-only verification |
| Growatt read allowlist, family codes, units/time/key rules | 05 Gate A | Generic documented mappings and synthetic cases backed by verified evidence |
| Retry budgets, ranges and schedule behavior | 06 | Deterministic tests and documented final defaults |
| CI/image/platform/restore behavior | 07 | Executed checks with explicit unverified remote gates |
| Actual host/providers | 08 | Private observed acceptance, not assumed synthetic equivalence |

If a gate fails, preserve the failure and propose a bounded plan amendment. Do not
expand scope, guess provider semantics or claim an unrun check passed.

## Required execution handoff

Every task reports changed files, established public/module contracts, checks
actually executed and their outcomes, unverified gates and any approved deviation.
Human visual verification must use the task's actual build. Provider/live host
evidence stays private; public review uses synthetic examples and generic conclusions.
A reviewable application slice is not the same as completing deployed milestone 1.

The original planning session wrote documentation only. The user subsequently
approved implementation of task 01; that approval does not start later tasks.
