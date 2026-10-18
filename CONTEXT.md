# Energything

Energything retains home electricity and inverter observations and makes their
coverage and quality visible.

## Language

**Electricity interval**: An import or export energy reading for a stated start
and end time, measured in kWh. It belongs to one identified meter and direction.

**Import**: Electricity supplied from the grid to the household.

**Export**: Electricity supplied from the household to the grid. Export is not
synonymous with solar generation.

**Growatt sample**: An inverter observation containing measured power, battery
state or energy counters at a provider-reported time. It is not a billing interval.

**Measurement time**: The time an observation describes.
_Avoid_: Last fetched time

**Retrieval time**: The time Energything obtained an observation from a provider.

**Coverage**: The presence, absence and quality of observations over a stated
period. Missing observations do not represent zero consumption or generation.

**Backfill**: Collection of historical observations over a bounded requested period.

**Payload evidence**: Retained provider telemetry fields supporting interpretation
or later reprocessing of an observation.

**Collection window**: A bounded set of observations fetched and validated together.
If its data is invalid or contradictory, the entire window is rejected.

**Accepted observation**: An observation from a validated collection window retained
as usable source history. A rejected refresh does not replace it.

**Correction**: A changed valid observation obtained in a later accepted window.
It differs from contradictory readings returned within one window.

**Collection attempt**: One effort to fetch and validate a bounded window, with a
start, source, requested period and recorded outcome.

**Collection audit**: Retained history of collection attempts and their outcomes,
including when, which source and period, and why an attempt failed or was rejected.

## Task06 scheduling handoff

See [collection contracts](docs/collection.md) for the current automatic routine/bootstrap, repair CLI,
retry/quota and qualified time policy. ONCE/BACKFILL_START/startup RANGE_REFRESH
controls are obsolete and ignored. Migration006 retains independent schedules;
OpenQueue deliberately performs no startup recovery. The authenticated status
includes plans and explicit search-limit/boundary-unresolved state. A short search
is not an archive. Seasonal clock and stable repeated-hour identity remain gated;
old evidence and accepted observations are retained. Task06a must consume these
stored states without inferring history completion or adding another scheduler.
Older one-shot procedures above describe prior task verification, not current startup.

Task06 whole-history correction: migration007 replaces the rejected120-day stop
with persistent calendar scans. Completing a year automatically plans older
bounded units; empty/zero years never terminate acquisition. Existing frontiers,
unfinished jobs, records/checkpoints/retries/quotas are preserved. Growatt
source-lifetime metadata remains qualified but does not prevent ongoing older
collection. Octopus's verified oldest bound remains separate. Several years of
generation can take weeks under the unchanged ten-per-rolling24h quota.

Task06a now exposes local collection audit and bounded selected telemetry evidence.
Older whole-history passages above are superseded by ENERGYTHING_HISTORY_START,
default 2022-01-01 inclusive. Audit and generation progress views do not infer zeros,
outages, provider retention or authoritative instants from unresolved clocks.


Schema008 (user correction) replaces Growatt attempt/ordinal identity with the
natural primary key (source identity, provider date, reported timestamp).
Refetches upsert; accepted changes overwrite. No sample/interval/aggregate value
history tables remain. This supersedes all prior separate-occurrence/version
rules. Reported clock keys do not resolve measurement instants. Conflicting
same-clock rows within a unit reject; no invented occurrence identifier. Migration
collapses old copies to latest accepted rows, retaining separate audit/evidence,
progress, source selection and quota reservations. Take a stopped backup first.

Task07a: live SQLite is ephemeral; `/data/latest.tar` holds a complete standalone
snapshot/manifest published online, atomically. Every startup restores it. Final
shutdown export follows joined writers. Crash loses unsnapshotted state; aggregate
restart holds are five minutes or 24 hours with the daily cap. Legacy preview
conversion requires a deliberate clean-stop copied-directory transition and
isolated validation; never automatically switch its retained mount.
