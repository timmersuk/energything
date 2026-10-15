# Growatt timestamp source notes

Investigated 15 October 2026. This note contains public-source analysis only;
no credentials, private identifiers, telemetry examples or authenticated requests.
It does not change the normalization contract in `growatt-protocol.md`.

## What the Java fields establish

Java `Date.getTime()` represents milliseconds since the UTC/GMT epoch. Its legacy
component getters use a local timezone; `year` is relative to 1900, `month` is
zero-based, and `timezoneOffset` has the opposite sign to an east-positive UTC
offset. These conventions explain the unusual component representation without
making it corrupt data. They do not establish the business meaning of a Growatt
field that happens to contain a millisecond integer.
[Oracle Date](https://docs.oracle.com/javase/8/docs/api/java/util/Date.html)

Java `Calendar.getTimeInMillis()` also returns UTC epoch milliseconds, while
`getTime()` returns a `Date` for the same value. Calendar fields and the epoch are
two representations connected through the calendar's timezone. A calendar can be
constructed using an explicit timezone or the process default. Consequently,
agreement between `calendar.timeInMillis` and `calendar.time.time` would be an
internal consistency check, not two independent observation clocks. An
`Asia/Shanghai` calendar identifies that calendar's timezone, not necessarily the
inverter installation's timezone.
[Oracle Calendar](https://docs.oracle.com/javase/8/docs/api/java/util/Calendar.html)

The JSON resembles property serialization of these APIs, but the documentation
does not establish Growatt's serializer or parsing implementation. In particular,
an epoch computed by parsing a wall clock in the wrong timezone is still a valid
Java epoch value; library semantics alone cannot establish that it represents the
physical observation instant.

## What daylight saving establishes

A local clock plus a regional timezone normally identifies one instant. During
the spring gap it can identify none; during the autumn overlap it can identify
two. Choosing a library's default offset does not supply missing evidence about
which occurrence the provider intended. A reliable independent instant or offset
can disambiguate the two candidates; merely repeating the local clock in calendar
components cannot.
[Oracle ZoneRules](https://docs.oracle.com/javase/8/docs/api/java/time/zone/ZoneRules.html)

This explains why a repeated local hour can be a daylight-saving effect. It does
not explain an arbitrary multi-hour disagreement between distinct representations.

## Provider meanings still requiring evidence

The existing protocol note references Growatt_API's
[SPH latest](https://www.showdoc.com.cn/262556420217021/6129764434976910) and
[SPH history](https://www.showdoc.com.cn/262556420217021/6129765461123058).
During this investigation these pages exposed only their client-rendering shell
through web retrieval. Public page API attempts were unavailable, and a shell
request failed DNS resolution. Therefore this investigation did not freshly verify
their parameter/field descriptions. The prior protocol note remains the record
of the earlier documentation review.

Neither Java documentation nor a field name establishes whether `createTime` is
sample time, receipt time, persistence time, or another event. Likewise `utcTime`
must not become authoritative solely because of its name. The exact effect of
history's `timezone_id` on row selection and returned clocks needs a controlled
comparison; echoing a requested timezone is not independent proof.

Useful bounded comparisons are: current latest versus request start/end instants;
matching latest/history samples; the same historical request with different
timezone IDs; and winter versus summer samples. Compare scalar clock fields and
timing differences privately, retaining original evidence. Consistently small
`createTime` minus a proposed local-to-UTC measurement conversion would support a
receipt-time hypothesis, but does not prove a guaranteed maximum upload delay.

An autumn-overlap rule based on receipt time additionally needs an established
delay bound or provider evidence: delayed upload can otherwise place either
candidate before receipt. A successful recent comparison can justify a scoped,
explicitly evidenced installation interpretation while leaving untested periods,
overlap occurrences and provider-wide semantics qualified.
