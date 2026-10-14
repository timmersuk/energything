# Durable storage contract (task 02)

All absolute instants are signed integer UTC Unix microseconds. Go callers use
`Instant`/`At(time.Time)`; source clock text is independent. Windows are half-open.
Requested bounds are restricted to years 0001–9999 to prevent arithmetic overflow.
Electricity is nonnegative exact decimal TEXT (no exponent, float or rounding).
Octopus slots are aligned 30-minute intervals, bounded to 14 absolute days.

Synthetic examples established before the migration:

- Import identity `synthetic-mpan/synthetic-meter/import`, slot 00:00–00:30,
  `0.100`: replay keeps one current interval and one evidence blob, with separate
  retrieval/attempt links. A later accepted `0.120` replaces the current value;
  the earlier observation version and evidence remain.
- Two slots submitted with one invalid decimal or conflicting duplicate: the
  entire commit fails validation. The caller records a rejected attempt with
  suspect sanitized evidence; current observations and checkpoint do not change.
- Fully fetched empty Growatt history: terminal `empty`, fetched window recorded,
  zero measurement coverage, earlier local samples remain. Empty never means zero.
- Two `2026-10-25 01:30` samples without established key/time: each is retained
  under attempt plus observation ordinal, with null normalized time and key.
  A replay deduplicates evidence bytes but retains separate unresolved occurrences.
  Task 05 establishes resolution and reconciliation; task 02 guesses neither.
- Missing SOC is JSON null; explicit SOC `0` is a decimal string with its unit.
  An unestablished unit also remains JSON null.
  Unknown selected fields and original timestamp fields survive in telemetry
  evidence and original-time JSON. Unresolved device discovery evidence must be
  held by the future provider gate until a source identity is established.

## Logical schema

Source identity is unique by provider/stream/stable key. Octopus stable key must
encode MPAN and serial; Growatt must encode established plant/device identity.
Storage treats that key as opaque. Deselection never removes history. Necessary
discovery metadata is restricted to named meter/plant/device fields.

Jobs have a stable identity/kind/policy key, bounded current window, provider date,
optional range-request ID, due time, running attempt, retry count, pause flag and
nullable completed checkpoint. Different refresh/backfill policies have different
jobs. A range request ID binds an immutable ordered list of source/kind/bounds;
reuse with different work fails. No timer or provider request exists here.
Kinds are `history`/`latest`; policies are `routine`/`backfill`/`range`. Each bounded
historical window is immutable work; the later scheduler ensures the next unit
between fetches, rather than mutating a running window. Completed backfill/range
units are excluded from due work. Explicit acceptance replays are still supported.

Attempts have immutable source/job/window context, start/occurrence/finish,
terminal outcome, structured sanitized error and bounded retry/page details.
Evidence blobs are SHA-256 deduplicated canonical selected JSON. Each retrieval
links blob, source, attempt, retrieval time and completeness independently.
Current observations point to a retrieval; append-only versions retain all prior
values and retrievals. Unresolved Growatt rows use attempt/ordinal, never clock
text. Resolved keys are supplied only by the future protocol gate.

Window completions record latest accepted attempt, fetched state, expected/valid/
missing/quality slot counts and measurement-complete flag separately. Missing
slots do not delete old rows. Electricity expected slots are computed from elapsed
window bounds at attempt finish; selected samples do not imply full Growatt
measurement coverage. Completion checkpoint means fetched work, not full readings.

## Typed methods and guarantees

`Open(ctx, dir) (*Store, error)` opens, atomically migrates and reconciles old
running attempts; `Close()` closes last at shutdown. Database handles and SQL
remain private. Writes use a single connection and short transactions. Every
method takes context. Transactions are rolled back on cancellation or error.

- `UpsertIdentities(ctx, []Identity) ([]Identity, error)` retains historical sources.
- `EnsureJobs(ctx, []Job) ([]Job, error)` creates/coalesces immutable work;
  `EnsureRange(ctx, requestID, []Job)` atomically registers a configuration range.
- `DueJobs(ctx, now, limit)` returns eligible selected, unpaused, nonrunning work.
- `ResumeJob(ctx, jobID, nextDue)` explicitly unpauses unfinished work after a
  future configuration/discovery gate resolves an error; no checkpoint changes.
- `BeginAttempt(ctx, AttemptRequest) (int64, error)` atomically claims a job.
- `CommitAcceptedWindow(ctx, AcceptedWindow) error` validates the whole unit then
  atomically writes observations, versions, evidence/retrievals, window completion,
  checkpoint, terminal success/empty and next due. No absence deletes history.
- `RecordRejectedOrFailedAttempt(ctx, FailedAttempt) error` writes terminal audit,
  suspect/incomplete evidence and retry/pause state only; no accepted progress.
- `RecoverInterruptedAttempts(ctx, now) (int64, error)` marks running attempts
  interrupted and makes the same unfinished jobs due, preserving checkpoints.
- `QueryIntervals`, `QuerySamples`, `QueryAudit` return bounded typed pages;
  `QueryAudit` also returns matching total/failed/rejected counts and evidence links.
  `QueryAttempt(ctx, id)` retrieves one sanitized audit with evidence references
  or `ErrNotFound`; no database or SQL handle is exposed.
  `QueryStatus` returns aggregate counts, last retrieval/measurement, attempt state,
  jobs and explicit fetched/measurement window coverage.

Example: ensure an Octopus `history` job with start/end in microseconds; begin it
with `Started=At(now)`; accept `Intervals=[{Start,End,KWh:"0.100",Evidence:0}]`
and `Evidence=[{Content: selectedJSON,Retrieved:At(now),Complete:true}]`.
An evidence index is local to this commit, not a SQL ID. Query results expose
decimal strings, nullable sample instants/values and opaque next cursors.
Even empty accepted responses require complete selected evidence. Successful
attempts may carry bounded retry details and a timestamp-quality notice while
retaining their success outcome. Evidence canonicalization/digesting happens
before the write transaction; cloud work never happens inside it.

Query limits default to 100 and cap at 500. Ascending records use instant/identity
or instant/row-ID; descending audit uses occurrence/ID. Opaque versioned cursors
bind all filters and page size; malformed or mismatched cursors fail validation.
Sample absolute ranges omit null instants and report their count; quality queries
use provider date or retrieval range. Source IDs in results are local IDs; audit
display identity is masked. Raw stable identifiers and evidence content are never
included in audit/status results. Detailed evidence access is not a public API.
Status includes latest attempt and latest accepted attempt separately. It returns
at most 500 recent jobs/windows per identity plus total counts; accepted coverage
is always coverage of the latest fetched response, not old retained rows.

Evidence inputs must already be selected telemetry, not HTTP transcripts or
discovery responses. Storage additionally rejects credential/header/address keys
recursively; unknown telemetry fields are retained. Arbitrary raw error strings
are not accepted: public messages derive from enumerated error classes/codes.
Retry codes are enumerated and carry no URL/body. This avoids attempting to redact
unknown secrets from free text. Selection, unit and timestamp protocol validation
belong to tasks 04/05; no consumer may bypass these transactions with ad hoc SQL.
