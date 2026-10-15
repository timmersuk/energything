# Growatt aggregate history source notes

Public documentation investigation, 15 October 2026. This note makes no production
allowlist change and contains no private telemetry. No authenticated requests were
made by the source researcher.

## A different read endpoint exists for aggregate generation

Growatt_API section 2.8 documents **GET `/v1/plant/energy`**. Required parameters
are `plant_id`, `start_date`, `end_date`, and `time_unit`; the latter supports
`day`, `month`, `year`. Optional `page` / `perpage` support pagination, with
maximum 100 per page. The response has `data.count`, `time_unit`, and `energys`
entries containing `date` and `energy`; the documented metric is power generation
in kWh. The page states a five-minute call frequency and a maximum seven-day date
interval. Month/year date formats and the interaction with that interval are not
sufficiently established to justify guessing broad query ranges. These statements
come from a fresh public page-info read by the parent investigation.
[Official plant energy documentation](https://www.showdoc.com.cn/262556420217021/1494061730868556)

This is aggregate generation, not the MIX minute-level telemetry endpoint with
another argument. Its documented schema does not provide separate battery charge,
battery discharge, grid import/export, household load, or SOC history. Availability
older than detailed telemetry must be tested separately; the existence of an
aggregate endpoint supplies no retention guarantee or completeness guarantee.

## Other documented candidates

Section 3.41 documents **GET `/v1/device/inverter/day_energy`** with `device_sn`
and `date`. Applicability to the selected family-5 SPH/MIX device has not been
established. It is a candidate for further documentation/permission review, not
an approved family-independent fallback. This endpoint information also comes
from the parent's fresh public page-info read.
[Official device daily generation documentation](https://www.showdoc.com.cn/262556420217021/6118030327488881)

The plant overview documents current day/month/year/lifetime energy totals in kWh.
Those current counters do not supply arbitrary historical periods.
[Official plant overview](https://www.showdoc.com.cn/262556420217021/1494061093808613)

The older Growatt-authored OpenAPI PDF, version 1.0.1 from 2016, independently
describes plant energy with day/month/year aggregation and kWh generation. It is
historical corroboration only; current endpoint requirements take precedence.
[Legacy protocol PDF, section 4.2.6 and appendix 1](https://growatt.pl/wp-content/uploads/2020/01/Growatt-Server-Open-API-protocol-standards.pdf)

## Rich battery/grid/load aggregates remain a separate question

A newer official catalog lists device historical generation, SPH device history,
and detailed device data for one day. Titles alone do not establish a long-lived
daily energy series covering every MIX battery/grid/load metric. No such richer
aggregate contract, units/signs, retention promise or equivalence to website charts
was established by the content accessible to this researcher.
[Newer official catalog](https://www.showdoc.com.cn/2598832417617967)

Candidate pages for further public documentation review are
[historical device generation](https://www.showdoc.com.cn/2598832417617967/11558385232366637),
[SPH history](https://www.showdoc.com.cn/2598832417617967/11558381065875758), and
[detailed device day](https://www.showdoc.com.cn/2598832417617967/11558381061377651).
They returned client-rendering shells or timeouts through this researcher's web
retrieval; direct public API requests previously failed shell DNS resolution.

Website aggregates may be useful independent data, but they must retain their
source, period, metric and qualification. They must not be represented as recovered
minute samples, or used to fill missing detailed telemetry with invented values.

The parent subsequently fetched the three newer public page-info documents.
The historical device-generation entry repeats `/v1/device/inverter/day_energy`.
The device-day entry documents POST `/v4/new-api/queryHistoricalData` with
`deviceSn`, `deviceType`, `date`; the SPH page describes the full detailed-day
telemetry schema. None of these descriptions establishes a separate long-lived
battery/grid/load aggregate feed. Their public response descriptions were reviewed,
but no authenticated device/day-energy or V4 request was made.

A subsequent single bounded authenticated plant/energy day probe successfully
returned a positive older winter-day generation aggregate, confirmed with exact
arithmetic. This is empirical older aggregate availability, not an official
retention promise. Exact values and identity remain private; see
[the aggregate investigation](growatt-aggregate-investigation.md).
