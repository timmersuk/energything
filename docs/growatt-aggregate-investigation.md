# Growatt aggregate-history investigation

Investigated 15 October 2026 after the user reported that the website shows
longer-term aggregate data. Aggregate generation is available through a different
read-only API, rather than a coarser-resolution argument to MIX telemetry.

## Verified documented contract

**GET `https://openapi.growatt.com/v1/plant/energy`**

| Argument | Meaning |
| --- | --- |
| plant_id | Confirmed plant identity |
| start_date / end_date | Requested dates; documentation states span <=7 days |
| time_unit | day, month or year |
| page / perpage | Paged results; maximum 100/page |

Success envelope has error_code=0 and data.count, data.time_unit, data.energys.
Each energy entry has a date and energy; the official metric is generation in kWh.
Documented acquisition frequency is once per five minutes. The page does not
establish a total retention guarantee. Month/year date formatting and its interaction
with the stated seven-day date span still need validation before implementation.
[Official plant energy documentation](https://www.showdoc.com.cn/262556420217021/1494061730868556)

Illustrative one-day request with synthetic identity:

```text
GET /v1/plant/energy?plant_id=SYNTHETIC&start_date=2026-01-15&end_date=2026-01-15&time_unit=day&page=1&perpage=100
```

Token goes in the token header, never the URL. The synthetic plant placeholder is
not an identifier format recommendation; it represents a discovered private value.

## Private verification actually performed

After reviewing the official public page-info response, one authenticated GET was
made for one older winter day, outside the observed detailed-telemetry window.
The provider returned success, count one, time_unit day, one matching date and an
exact positive generation value. Independent exact-rational parsing confirmed
positivity without printing the amount. Earlier detailed MIX requests for that day
were empty. This establishes older aggregate availability for the investigated
installation; it does not establish the earliest aggregate date or full completeness.

The throwaway probe used a literal research allowlist of HTTPS exact
openapi.growatt.com, GET /v1/plant/energy, paired identical dates, day unit, page1,
perpage100, a previously confirmed plant identity, no redirects, 30-second request
and 40-second overall limits, 4 MiB body bound and no retries. Only one authenticated
aggregate request was made, respecting the documented five-minute call frequency.
Credentials stayed in process memory; exact private values/results/code remain
ignored under `.cache/task05-aggregate-research`. No accepted data, schema, preview
image, production endpoint allowlist or scheduler changed.

## What this does and does not recover

This is a plant-level generation aggregate feed. Its documented response does not
provide separate battery charge/discharge, grid import/export, household load or
SOC series. Do not label generation as grid export/import, reconstruct minute
samples from it, or use daily totals to infer the DST occurrence of a local clock.
It can complement detailed telemetry if retained as a distinct period/metric/source.

The official device-generation endpoint
[GET /v1/device/inverter/day_energy](https://www.showdoc.com.cn/262556420217021/6118030327488881)
returns a scalar daily generation example; selected SPH/MIX applicability was not
established, so it was not called. The plant overview supplies current counters,
not arbitrary historic ranges.

Newer official documentation also describes
[POST /v4/new-api/queryHistoricalData](https://www.showdoc.com.cn/2598832417617967/11558381061377651)
with deviceSn, deviceType and one date, returning full detailed device-day telemetry.
Its [SPH schema](https://www.showdoc.com.cn/2598832417617967/11558381065875758)
contains battery/grid/load fields, but that is detailed telemetry, not proof of
long-term aggregate retention. No authenticated V4 call or claim of longer V4
availability was made. No website session/settings APIs were invoked.

## Handoff

The observed approximately 95-day boundary must be described specifically as the
MIX detailed-telemetry boundary, not all Growatt data. Daily generation demonstrably
extends farther back. Day/month/year aggregate ingestion needs its own exact period,
timezone, metric, value/correction, completeness and evidence contracts, plus a
validated earliest-availability policy. It must not be stuffed into half-hour
Octopus intervals or unresolved instantaneous samples. The current task06 scheduling
plan should account for this distinction and propose a contract/data-model amendment
before adding aggregate ingestion. This research alone does not authorize that work.

See [source notes](growatt-aggregate-source-notes.md) and
[detailed-history investigation](growatt-history-investigation.md).
