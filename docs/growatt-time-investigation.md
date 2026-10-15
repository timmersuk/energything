# Growatt timestamp comparison outcome

Investigated 15 October 2026 after the user requested a conclusion from bounded
read-only comparisons. This is a protocol investigation, not a normalization or
data migration. No accepted observations, schema or preview image changed.

## Practical conclusion

The investigated installation's current readable clock behaves as UTC+01:00,
consistent with Europe/London during British Summer Time. Current recent readings
can be interpreted that way with strong empirical confidence. The calendar epoch
is not a trustworthy independent measurement instant for these responses: the
same readable wall clock is represented in an Asia/Shanghai calendar, producing
a different instant. Treating that epoch as the real observation would introduce
a multi-hour shift. This is consistent with a server-side parsing/serialization
artifact; the provider implementation itself has not been inspected.

This resolves the current discrepancy's practical explanation. It does not prove
that the provider automatically follows Europe/London through winter and daylight
saving transitions. A fixed UTC+01:00 device/cloud clock would also fit the observed
October samples. Do not hardcode an installation-specific offset for all devices,
dates or seasons on the strength of these results.

## Comparisons actually performed

Private throwaway Go code used only the existing allowlisted MIX latest/history
paths and a serial already established by prior discovery. Six total POST requests:
two fresh latest snapshots, one first-page historical request under each of three
requested IANA timezones, and one first-page winter-day availability probe. Each
request used exact HTTPS host/path, no redirects, a 30-second timeout, 4 MiB limit,
100-row page limit and a three-minute overall context. Requests were separated by
12 seconds; no retries, full archive scan, settings/control calls or new discovery.
The first pages were comparison probes only, not accepted as complete history.

A separate local authenticated REST comparison examined every retained sample in
one previously accepted complete provider day, using bounded cursor pages without
opening the live database. Credentials stayed in process memory. Selected clock
fields and numeric summaries remain under ignored `.cache/task05-time-comparison`;
no raw timestamps, provider identifiers, credentials or payloads are copied here.

Observed generic relationships:

- Both fresh latest readings were recent under the London/BST interpretation.
  A UTC interpretation put them in the future; Shanghai put them hours in the past.
  The readable clock advanced along with retrieval time, rather than remaining stale.
- Every compared retained history sample had a positive createTime-minus-London-clock
  difference, typically seconds. Some were delayed by minutes, so a fixed tiny
  receipt delay must not be assumed.
- The raw calendar representation encoded the same wall-clock components using
  Shanghai context. Its two epoch properties agreed with each other; those two
  properties are representations of the same calendar, not independent clocks.
- The three timezone requests returned the same count and identical compared
  clock/calendar/createTime tuples. This shows no display/selection difference for
  those exact requests; it does not establish that the parameter is always ignored
  (the provider might reject/ignore particular string values or apply it elsewhere).
- The selected winter day returned an empty array and count zero. This proves only
  observed unavailability for that request, not retention policy, absence of all
  winter data or a seasonal clock rule.
- createTime behaves consistently with server receipt/persistence time in these
  comparisons, not proof of an exact measurement-time field or guaranteed upload delay.

## Rule supported for the next implementation step

Use the selected installation's independently confirmed regional timezone and
readable wall clock to construct candidate measurement instants; the current
comparison supports its UTC+01:00 period. Keep the evidence/provenance and make
this scoped assumption explicit. Do not use the Shanghai calendar epoch or the
requested timezone parameter as independent disambiguation evidence.

For a normal local time there is one regional-zone candidate. For a spring gap
there are none; retain qualified evidence rather than shifting it automatically.
For an autumn overlap there are two: do not let a library choose silently. A
validated absolute receipt time can rule out a candidate after receipt, subject
to clock precision/tolerance. If both candidates precede receipt, observed quick
uploads alone do not prove which one occurred; keep it ambiguous unless provider
offset/identity or stronger independent evidence resolves it. A single observed
maximum delay is not a guaranteed future delay bound.

Before general seasonal normalization, confirm the installation timezone and
provider clock behavior on an available winter/transition sample or an explicitly
verified provider rule. Record that remaining gate separately from the now-supported
current-period interpretation. Existing accepted raw evidence makes later
reconciliation possible; storage changes must preserve retrieval provenance,
conflict validation and old evidence. This note does not authorize inventing stable
sample IDs from receipt time, page position or clock text.

The current normalizer and stored `unresolved_time` records remain unchanged by
this investigation. Task06's explicit time gate should implement the evidenced
policy or record the precise remaining seasonal/overlap blocker, rather than repeat
the original blanket uncertainty without using these results.

## Primary source context

See [public source notes](growatt-time-source-notes.md) for Oracle Date/Calendar/
ZoneRules citations and the limits of freshly accessible Growatt documentation.
See [the existing Growatt protocol gate](growatt-protocol.md) for the verified
four-endpoint allowlist and earlier official provider documentation review.
Authenticated provider responses are the primary empirical source for the scoped
observations above; their selected private evidence is deliberately not published.
