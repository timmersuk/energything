# Growatt detailed-history availability investigation

Investigated 15 October 2026 after the user asked how far back current data could
be retrieved. No production code, accepted observations, preview image or database
changed. This is empirical availability research, not a completed historical backfill
or the task06 automatic historical-boundary policy.

## Conclusion

The investigated SPH/MIX history endpoint currently behaves like a rolling window
of approximately 95 days of detailed telemetry for the selected installation.
This is strongly supported observed behavior, not a documented universal retention
promise or proof about other Growatt products, aggregate data or future API behavior.

Nonempty daily responses were found at 90 through 95 days back. The 95-day edge
was a partial day, with the oldest returned record's createTime almost exactly
95 elapsed days before retrieval. Two bounded edge-page requests separated by
about a minute showed the oldest returned clock advance a minute and reported day
count decrease by one; the oldest receipt age remained close to 95 days. This
moving edge supports expiry rather than a fixed installation-start date. Necessary
previously retained discovery metadata also placed plant creation before the edge,
but plant creation is not itself a measurement-history boundary.

Adjacent days at 96 through 102 days back were empty, as were the sampled dates
approximately six months, one year, two years, three years and five years back.
Empty days alone do not prove no isolated older data exists; the moving, near-95-day
edge is the stronger evidence for the practical lookback conclusion. No monotonic
availability assumption or binary search was used.

## Bounded work actually performed

- Sixteen fixed day probes at ages 1, 7, 14, 21, 28, 30, 31, 35, 45, 60, 90,
  180, 365, 730, 1095 and 1825 days, one first page per day.
- Twelve consecutive daily probes at ages 91 through 102 days to characterize
  the neighborhood of the oldest nonempty sparse probe.
- Two page-six probes of the edge day, containing its partial final page, to
  observe the oldest returned clock/receipt age and whether the edge moves.
- Thirty total authenticated read-only requests, using only the existing
  /v1/device/mix/mix_data POST allowlist and previously confirmed family5 identity.
  No new discovery, settings/control endpoints, retries or automatic scheduler.
- 30-second requests, 4 MiB response bound, 100 rows/page, 12-second spacing
  between ordinary probes; edge observations separated by 65 seconds. Each batch
  had a finite overall context and stopped on any provider/transport error.
- All 28 daily first-page probes had successful provider envelopes, expected
  first-page sizes for their reported counts, and rows within the requested day.
  The edge responses likewise succeeded. No failure was classified as absence.
- These partial-page probes were not accepted as complete day ingestion. No
  checkpoints, local sample timestamps or retained-history coverage were changed.

Exact requested dates/counts, selected timestamp evidence, reports and throwaway
code remain ignored under `.cache/task05-history-range`, `task05-history-refinement`
and `task05-history-edge`. Runtime credentials stayed in process memory; private
identifiers and raw payloads are not published here.

## Documentation and handoff

The current official [SPH/MIX history page](https://www.showdoc.com.cn/262556420217021/6129765461123058)
was refreshed through its public page-info API. It states a maximum seven-day
request span, at most 100 rows/page and acquisition frequency once per ten seconds;
it does not state a total 95-day retention promise. See
[public history source notes](growatt-history-source-notes.md) for other official
sources and why request span/privacy retention do not establish API lookback.

For task06, use this as scoped empirical boundary evidence, preserve its observation
time and revisit it rather than declaring a permanent provider-wide limit. The edge
can advance during backfill; recovery must preserve accepted local data and handle
expiry/count changes honestly. Coarse older empty probes are not a universal proof
of no older readings, nor are they an implemented finite search/stopping policy.

For the timestamp investigation, the currently observed detailed-history window
does not reach a UK winter or previous autumn transition. Thus the planned check
after the coming BST transition remains useful. A lack of accessible winter data
must not be replaced by an assumed fixed offset or silent repeated-hour selection.
