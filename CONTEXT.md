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
