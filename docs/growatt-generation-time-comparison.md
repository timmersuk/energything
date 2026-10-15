# Comparing aggregate generation with detailed telemetry

Investigated 15 October 2026. This read-only comparison does not change
normalization, retained records, schema, preview image or provider allowlists.
Private identities, sample timestamps and energy values remain ignored locally;
only relationships and limitations are recorded here.

## What the comparison can establish

Agreement between a plant aggregate's date and the detailed feed's daily counter
can support **the same calendar-day grouping**. It does not independently identify
that calendar's UTC offset. Both feeds may use the same unresolved local clock.
In particular, relabelling all detailed clocks with an assumed zone and then
matching their date labels back to the aggregate is circular evidence about UTC.
This is an inference from the available contracts, not a documented Growatt rule.
The [aggregate source review](growatt-aggregate-source-notes.md) records generation
in kWh but no timezone; the [current normalizer](../internal/growatt/normalize.go)
preserves readable dates and leaves measurement instants unresolved.

Daily solar generation is especially weak for distinguishing nearby midnight
boundaries. If all energy between two candidate boundaries is zero, both candidate
days have exactly the same total. Exact agreement then cannot choose between them.
An offset that moves an active generation interval across a boundary might be
rejected, but that does not establish that the remaining candidate is unique.
This follows directly from comparing sums over the shared interval and the two
nonoverlapping boundary intervals; it requires no assumption about this plant's
actual night generation.

The earlier [timestamp investigation](growatt-time-investigation.md) independently
supports a current UTC+01:00 readable clock for this installation. It explicitly
does not distinguish a fixed offset from seasonal Europe/London behavior.
Generation comparisons confined to a period when those rules have the same offset
cannot distinguish them either. Even winter totals may remain inconclusive when
their differing boundaries contain no generation. Independent clock evidence in
both seasons, or an independently established provider rule, is needed to resolve
that remaining question.

## Compare like quantities first

The plant endpoint documents generation in kWh. Detailed MIX fields include
`epvtoday`, `epv1Today`, `epv2Today`, `eacToday` and lifetime counters. Their names
alone do not prove which quantity equals plant generation, whether all plant
devices are covered, or the units/scales. The current normalizer deliberately
keeps these fields as `unit_unresolved`; only `ppv` has a verified W unit among
these generation-related measurements. See the [field matrix](growatt-protocol.md)
and [normalizer](../internal/growatt/normalize.go).

Do not sum cumulative daily counter readings. Compare a validated final daily
counter level, or justified differences of a monotonic lifetime counter across
boundaries, preserving counter resets and missing samples. A maximum daily value
is a useful diagnostic when the entire day is covered and the counter behaves
monotonically; it is not automatically the day's final authoritative total.
Counter equivalence and scale must be established rather than fitted separately
for every day until the numbers agree. Compare several varied complete days and
report missing data, resets, precision and corrections.

Integrating `ppv` over sample intervals supplies a secondary estimate, not an exact
generation total: the sampled signal leaves unobserved behavior between readings,
gaps and boundary intervals. PV input power also need not equal the documented
plant-generation measurement point. Test that equivalence before attributing a
residual to timezone. These limitations follow from the different documented
metrics in the [protocol field matrix](growatt-protocol.md) and
[official plant energy documentation](https://www.showdoc.com.cn/262556420217021/1494061730868556).

## Bounded empirical method

1. Inventory existing accepted aggregate periods and detailed provider days,
   including neighboring days needed for shifted boundaries. Establish overlap
   before spending any limited aggregate requests. An older aggregate alone cannot
   be compared with detailed data that have expired.
2. Check that the accepted detailed attempt covers the provider's complete
   reported response, then separately assess physical-day coverage: accepted
   pagination does not prove uninterrupted sampling or boundary measurements.
   Retain occurrences without merging unresolved repeated local clocks.
3. Compare candidate daily counters against the aggregate for each shared date,
   using exact arithmetic and one explicit common scale only where independently
   justified. Record a mismatch as a mismatch; do not invent missing samples.
4. For an independently anchored detailed timeline, integrate or difference the
   comparable metric within each candidate calendar window. Include plausible
   UTC, fixed-offset and regional-zone rules. Report whether they actually select
   different nonzero energy, and the uncertainty caused by gaps or sampling.
5. Separate the conclusions: metric/date agreement; offsets excluded; current
   absolute-clock evidence; seasonal behavior; unresolved repeated-hour identity.
   State explicitly when several candidates predict the same totals.

These analysis steps require evidence for each premise; acceptance alone does not
satisfy them. Accepted collection and occurrence rules are described in
[the protocol gate](growatt-protocol.md); observed detailed availability is scoped
in [the history investigation](growatt-history-investigation.md).

## Autumn overlap remains a separate identity problem

A regional local clock can have no valid offset during a spring gap and two valid
offsets during an autumn overlap. A daily total cannot assign individual equal
clock strings to the first or second repeated hour, even if the daily generation
sum is exact. Nor does a daily counter become a unique sample identifier merely
because it differs in two records. An independent instant, verified offset or
provider occurrence identity is still required. See
[Oracle ZoneRules](https://docs.oracle.com/javase/8/docs/api/java/time/zone/ZoneRules.html)
and the existing [timestamp policy analysis](growatt-time-investigation.md).

## Empirical results

The local server's authenticated REST API was queried for three neighboring
provider dates, with a maximum of20 pages/date and500 rows/page. One day had1,427
retained detailed occurrences, the preceding day had none, and the following day
had only one. The analyzed day belongs to one device matching the aggregate's
plant. Its earliest and latest readable-clock samples are within five minutes of
the respective provider-day ends. This is useful boundary coverage, not a claim
that every physical instant was sampled.

The already retained aggregates covered an older, nonoverlapping week. Therefore
one additional read-only seven-date aggregate request, ending on the detailed
day, was necessary. It used the existing validated aggregate client, exact HTTPS
destination, disabled redirects, a40s overall context, existing page/body limits,
one transmission/page and a permit that refused any second page. The five-minute
interval was checked against the preview's last aggregate acceptance before the
call; it was also later than this conversation's previous manual aggregate read.
The result was one complete page with seven daily rows. No database was opened
by a CLI, no observations were ingested, and no accepted progress changed.

Exact rational arithmetic, with no floating-point conversion or scale fitting,
established for the overlapping day:

- The aggregate equals the sum of the observed maxima of `epv1Today` and
  `epv2Today`. Both daily counters begin at zero and never decrease in the retained
  day, so this does not sum repeated cumulative readings.
- Independently, the aggregate equals `max(epvTotal)-min(epvTotal)`. This lifetime
  counter never decreases in the retained day.
- `epvtoday` and `eacToday` do **not** individually equal the aggregate. Field names
  are insufficient to select an equivalent counter. The exact matches above are
  scoped empirical corroboration for this device/day, not a provider-wide unit or
  multi-device aggregation contract amendment.
- All60 retained samples in each of the first and last readable-clock hours have
  zero `ppv`, and the PV lifetime/channel daily counters are constant within each
  of those hours.
- Reinterpreting the observed clocks as either UTC or Europe/London moves **no
  positive-PV samples** from the analyzed calendar date into another UTC date.
  These candidates therefore remain indistinguishable by that observed generation
  membership test. Some positive samples move under a Shanghai interpretation,
  but absent neighboring-day coverage prevents a full shifted-window total;
  this investigation does not claim that test independently excludes Shanghai.

The complete neighboring detailed days needed to compare full shifted24h windows
are not locally retained. No additional detailed archive fetch or integration of
power was performed: even a perfect solar-total comparison during this October
period cannot distinguish fixed UTC+01:00 from Europe/London, since those rules
currently agree. Neither the within-hour zero observations nor missing neighboring
data justify inventing unmeasured energy or proving an unobserved reset instant.

Private inputs, counter extrema/differences and bounded analysis code are under
ignored `.cache/generation-time-comparison`. Ordinary/session output contained
only counts, comparisons and booleans, not plant/device identifiers or amounts.

### Conclusion and next evidence

Generation feeds agree exactly on a useful shared daily quantity. This is a
successful cross-check, **not a definitive timezone determination**. It neither
overturns nor independently strengthens the earlier absolute-clock result beyond
relative feed/day consistency: current readable clock evidence already supports
UTC+01:00 for this installation. Fixed UTC+01:00 versus seasonal Europe/London,
and individual repeated-hour identities, remain the distinct unanswered questions.

The discriminating follow-up is an independently time-anchored fresh reading after
the regional autumn clock change (or a verified provider seasonal-clock rule),
compared with the current-season evidence. Confirm freshness and clock progression;
do not infer a guaranteed upload delay from a single receipt. A daily total cannot
assign an ambiguous repeated-hour occurrence even if its generation matches.

## Source-access qualification

The official Growatt plant-energy and MIX-history pages were attempted during this
review. Plant-energy returned an error and MIX-history exposed only its rendering
shell, so their content was not freshly verified here. Provider contract claims
above rely on the repository's recorded earlier primary-document reviews, linked
alongside those claims. Oracle's ZoneRules reference was accessible. No secondary
implementation was used as authority for Growatt's behavior.
