# Growatt read-only protocol gate (task 05)

Gate A recorded 14 October 2026 before normalization. Sources are Growatt_API's
[interface description](https://www.showdoc.com.cn/262556420217021/1494054927748011),
[plant list](https://www.showdoc.com.cn/262556420217021/1494058730404880),
[device list](https://www.showdoc.com.cn/262556420217021/6117958613377445),
[SPH latest](https://www.showdoc.com.cn/262556420217021/6129764434976910),
[SPH history](https://www.showdoc.com.cn/262556420217021/6129765461123058), and
[global errors](https://www.showdoc.com.cn/262556420217021/1494055648380019).
The older 2016 OpenAPI PDF is insufficient for MIX telemetry. Public documentation
examples contain provider demonstration identities; they are not fixture data.
Private read-only discovery/latest/history-page probing confirmed current shapes.

## Allowlist and discovery

Only HTTPS exact host `openapi.growatt.com`, no port/userinfo/fragment/escaped path.
Token header is attached only after destination/method/form validation. Redirects
are disabled. No configurable base URL or public generic request API exists.

| Method/path | Parameters | Successful data |
| --- | --- | --- |
| GET /v1/plant/list | page, perpage=100 | count, plants array; plant_id |
| GET /v1/device/list | plant_id, page, perpage=100 | count, devices array; device_sn, type |
| POST /v1/device/mix/mix_last_data | mix_sn | telemetry object |
| POST /v1/device/mix/mix_data | mix_sn, start_date=end_date, timezone_id, page, perpage=100 | count, datas array |

Envelope requires error_code=0 (number or integer string). HTTP 200 with nonzero
code fails. 10011 means permission denied; 10012 is provider frequency limitation;
10013/10014 are pagination constraints. Other provider codes retain their numeric
code as safe diagnostic context, never unchecked error_msg. No basic device details
are needed: mix_data_info exposes control settings and is excluded.

Only device/list type exactly 5 establishes SPH/MIX eligibility. Missing family,
other types, duplicate contradictory identities and unmatched explicit selection
fail safely. Select a unique eligible plant/device pair; multiple candidates require
GROWATT_PLANT_ID/GROWATT_DEVICE_SERIAL. A private CLI report supplies candidates.
No selection-management routes. Plant discovery data is ephemeral; only plant ID,
device serial and family are stored, never location/account/address material.

## Field matrix and exact values

All listed fields are optional. Missing/null remains a null value, never zero.
JSON numbers and numeric strings are expanded exactly without float64, rounding
or guessed scale. Scalars have a 128-character decimal bound. Unknown fields remain
in selected telemetry evidence. Credential/transcript/account/address-shaped keys
are removed recursively; the numeric Modbus `address` field is also excluded.

| Provider field | Meaning | Unit/scaling/sign rule |
| --- | --- | --- |
| soc | Inverter battery remaining capacity | %, identity scale; official socText percent example; 0..100 |
| bmsSOC | BMS remaining capacity, distinct from soc | unit unresolved; numeric 0..100, no equality requirement |
| ppv | Total PV input power | W, identity scale established by official ppvText and live unit label; nonnegative |
| plocalLoadTotal | Total local load power | unit/sign unresolved; retain exact scalar |
| pacToUserTotal | Grid downstream power | unit/sign unresolved; retain exact scalar |
| pacToGridTotal | Grid reverse power | unit/sign unresolved; retain exact scalar |
| pcharge1 / pdischarge1 | Battery charge/discharge power | unit/sign unresolved; retain exact scalars separately |
| epvTotal / epvtoday / epv1Today / epv2Today | PV counters | units unresolved; counters, not intervals |
| etoUserToday / etoUserTotal / etoGridToday / etogridTotal | Grid counters | units unresolved; counters, not intervals |
| echarge1Today / echarge1Total / edischarge1Today / edischarge1Total | Battery counters | units unresolved; counters, not intervals |
| elocalLoadToday / elocalLoadTotal / eacToday / eacTotal | Load/output counters | units unresolved; counters, not intervals |

Documentation names power and energy meanings but does not specify units/scaling
for most MIX fields. Names, register conventions, neighboring inverter endpoints
and household magnitudes are insufficient evidence. These fields deliberately carry
`unit_unresolved`; counter resets are retained, never converted to interval energy.
Malformed known scalar, impossible SOC, negative verified PV power or conflicting
verified ppvText rejects the whole unit. Unknown optional types do not reject.

## Time and identity

Preserve time, utcTime, createTime and full calendar JSON as original fields.
Documentation calls calendar time but does not prove whether its epoch represents
an absolute observation or a server serialization of the readable local clock.
Private samples showed readable time agreeing with Java calendar components and
an Asia/Shanghai calendar; that is not proof of the installation timezone. A
requested timezone_id is a display request, not independent UTC evidence. utcTime
and createTime are not documented measurement instants. Their presence does not
supply precedence over time. No runtime/host timezone or fixed observed offset is used.

Precedence: retain readable provider date when parseable; retain other originals;
qualify `unresolved_time` with null instant and null resolved key. Missing readable
time is `missing_time`; malformed text is `unresolved_time`. Calendar disagreement
is `contradictory_time`, still qualified evidence because semantics are unverified.
A history readable date outside the requested provider day is a hard rejection.
Spring gaps and autumn repeated clocks stay unresolved, including equal clock
strings with differing calendar epochs. Neither page ordinal, content hash nor local
clock is promoted to a stable provider identity. Storage associates these occurrences
with attempt/ordinal and evidence. Latest/history cannot yet be reconciled into one
stable observation; repeated retrievals remain honest separate occurrences.

## Acquisition and task 06 handoff

Sequential page assembly; stable first-page count, nonempty pages until count,
exact assembled count, no repeated nonempty page. Identical page replay is a cycle;
unknown-identity rows are retained as occurrences, never overwritten by time.
All rows validate before acceptance. Engineering budgets: 100 rows/page, 100 pages,
10,000 rows, 4 MiB/page, 30s/request; these are limits, not promised cadence/retention.
Provider docs permit 100/page, <=250 pages, <=7-day history; this adapter narrows
history to one provider day. Private day count showed minute-scale cadence, so a
288-row assumption would truncate valid telemetry. Moving counts fail the unit.
Empty is observed unavailability, never zero or proof no older data exists.

Client alone owns at most three HTTP requests/page for network/408/429/5xx;
1s then 2s plus <=250ms jitter. Retry-After seconds/date >30s returns a deferred
instant. Provider 10012 defers five minutes without inline retry. Other provider,
JSON/validation/auth/destination errors have no inner retry. Collection records
attempt, complete accepted evidence or suspect failed evidence through typed storage;
no timer or retry executor exists. Task 06 must not multiply the inner budget.
Unresolved units, absolute time and identity remain explicit field/source gates;
automatic historical boundary discovery belongs to task 06.

## Timestamp comparison follow-up (15 October 2026)

A user-authorized bounded private comparison now supports the installation's
current readable clock as UTC+01:00 (consistent with London/BST), with createTime
behaving as later receipt/persistence time and the Shanghai calendar epoch behaving
as a wall-clock interpretation artifact. Changing timezone_id across London/UTC/
Shanghai produced identical compared responses. This narrows the uncertainty;
see [the investigation](growatt-time-investigation.md) and
[public-source notes](growatt-time-source-notes.md). Winter/seasonal behavior and
some repeated-hour occurrences still lack independent evidence. The existing
normalizer/storage are unchanged until an explicit evidenced policy is implemented;
do not treat this installation-specific comparison as a universal Growatt rule.

## Observed detailed-history window (15 October 2026)

A [bounded availability investigation](growatt-history-investigation.md) found a
moving edge approximately 95 elapsed days old for this selected installation's
MIX detailed-history endpoint. Nearby older days and sampled much older dates were
empty. Two edge-page observations showed old data expire minute by minute while
oldest createTime age remained close to 95 days. The official history documentation
does not promise this retention duration: treat it as scoped empirical evidence,
not a universal guaranteed limit. Partial availability probes were not ingested.
Task06 must retain local history beyond cloud expiry and preserve/revisit boundary
provenance; no scheduler or complete available-history claim is implemented here.
