# 08 — Private OMV deployed acceptance

Status: planning draft. Depends on: 07 and [07a](07a-online-backups.md), including
06a audit. Completes milestone 1. Task07a is required before deployed acceptance.
Read: [execution contract](execution-contract.md), final operations/configuration docs.

## Objective and boundary

Verify the actual deployed application with both read-only providers, retained
history, automatic work, audit visibility and restoration. A synthetic demo,
local binary, cross-build or published image alone cannot satisfy this task.
No inverter writes, public household artifacts or automatic private-host CI access.

## Inputs and private handling

Inputs are a tested pinned image, generic operations docs, actual host access,
privately supplied runtime credentials, confirmed selected meters/device and a
suitable local data directory. Record private acceptance evidence in an ignored
energything-private directory in this workspace (excluded from Git and Docker
contexts) or another explicitly designated private store. Do not use octopull
exports as public fixtures or copy its .env into source/build context.

Public docs may contain generic check procedures and synthetic examples only.
Do not commit real identifiers, raw errors/payloads, household record counts,
costs, screenshots, host paths or private verification findings. Do not place
real secrets in commands that are echoed to chat, shell history or CI logs.

## Check sequence and observable criteria

1. **Host preflight.** Verify architecture is supported by the image, the mount is
   local rather than SMB/NFS, disk space is adequate and UID 10001 can create/write
   database companions. Confirm one instance per DB. Choose a free HTTP port and
   intended trusted LAN or HTTPS reverse-proxy access. Logthing is the deployment
   pattern; do not overwrite its container, mount or occupied port.
2. **Deployment.** Configure required credentials and chosen identities privately;
   deploy the pinned image with Compose and the persistent dump directory. The live
   SQLite database must be ephemeral, never mounted. Record image
   digest/build ID and schema version in private evidence. No public workflow connects
   to this host. Verify non-root operation and configured restart behavior.
3. **Authentication.** Unauthenticated /healthz is fixed/detail-free. Pages, assets,
   status, records and audit API require auth. Wrong auth reveals no data. A refresh
   without a valid CSRF token/origin is rejected without enqueuing a job.
4. **Discovery.** Actual confirmed import/export meter selection and supported
   Growatt device match private expectations. Unknown/ambiguous selection is not
   auto-resolved by guesses. No settings endpoints are invoked.
5. **Ingestion.** Observe bounded Octopus import/export history and Growatt latest/
   history arriving. Validate representative private readings against provider
   semantics. Missing/uncertain data is labelled; retrieval vs measurement times
   remain distinct. Verify no guessed Growatt UTC or repeated-clock deduplication.
6. **Automatic work.** Observe scheduled Octopus and Growatt work and bounded
   backfill without manual triggers. Verify next due/checkpoints advance only after
   accepted units. Wait the documented schedule or use a temporary documented short
   test interval privately, then restore normal config; record which method was used.
7. **Independent outage.** Use a reversible test-specific failure injection or
   provider-specific configuration failure; do not disrupt the host's whole network.
   Verify existing browsing/audit works and the healthy source continues. Restore
   configuration and observe resumed collection without full archive refetch.
8. **Audit.** Verify failed/rejected attempts show timestamp, source/window and
   sanitized reason through REST and UI, including repeat counts and retained prior
   accepted data. Use synthetic rejection injection if actual provider data does
   not fail validation; do not fabricate live invalid readings in accepted history.
   Verify historical failure remains inspectable after a later success/restart.
9. **Restart.** Gracefully restart, then replace the container including its
   ephemeral writable layer, keeping only the mounted dump. Verify startup restores
   observations, evidence, audit and checkpoints from the published dump and pending
   bounded work resumes. Test abrupt termination separately: verify the documented
   last-snapshot recovery gap and conservative quota restart behavior.
10. **Backup/restore.** Keep Energything running while the actual host backup
    service copies its published dump. Verify this while collection and dump
    publication continue. Restore the external backup into a separate dump mount
    and a new container with the matching image and provider credentials omitted.
    Validate records/checkpoints/audit/evidence and DB health. No shutdown of the
    source service, copying live SQLite, second live-DB recovery process or parallel
    restored collectors with live credentials may be required for routine backups.
11. **Operational handoff.** Confirm update, permissions, storage monitoring,
    graceful stop, outage interpretation and backup/restore instructions are usable.
    Record any deviations and unresolved gates privately with explicit disposition.

## Completion checklist

- [ ] Both electricity directions and Growatt actual observations retained and browsable.
- [ ] Automatic schedules and bounded backfill operate on OMV.
- [ ] Provider failure is independent and local reads remain usable.
- [ ] REST/UI durable audit shows error reasons and repeated attempts.
- [ ] Restart/image replacement retain history and progress.
- [ ] External backup copies atomic dumps while service runs; isolated restore verified.
- [ ] Ephemeral live database reconstructed from mounted dump after container removal.
- [ ] Auth, privacy, non-root storage and read-only provider behavior verified.
- [ ] Generic operations docs complete; all private evidence excluded from public artifacts.

Each check needs observed evidence or explicit human confirmation when agent host
access is unavailable. Unavailable access leaves this task incomplete; it does not
justify an invented pass. Actual deployment/publishing is a later explicit action,
not authorization implied by this planning session. Discord is deferred.
