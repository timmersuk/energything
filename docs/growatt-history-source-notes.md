# Growatt historical availability: source notes

Public-source investigation on 15 October 2026. No credentials, private caches,
authenticated requests, telemetry payloads or new provider endpoints were used.

## Request span is different from retention

The existing `growatt-protocol.md` records a maximum seven-day history request
span from the earlier review of Growatt_API's
[SPH/MIX history documentation](https://www.showdoc.com.cn/262556420217021/6129765461123058).
The adapter deliberately requests one day at a time. Neither limit means that
only seven days of past telemetry are accessible.

This investigation could not freshly read the ShowDoc page content: web retrieval
returned only `INDEX_HTML` / `ITEM_HTML`; a public page-info POST with the page's
Origin and Referer failed DNS resolution in the shell. Thus it established no new
official SPH/MIX retention period or earliest accessible date from that document.

The parent investigation subsequently refreshed the official page using its public
page-info API with same-site Origin/Referer headers. It confirmed the maximum
seven-day request span and acquisition frequency once per ten seconds, but found
no total retention duration. The earlier access failures above describe the initial
source-only attempt, not the final state of the provider documentation review.

## Other official sources do not establish an API lookback

Growatt's [monitoring FAQ](https://en.growatt.com/support/faq/monitoring), under
"How to view historical device data", describes navigation to historical device
data through OSS and ShineServer. Its instructions do not state an earliest date
or a guaranteed telemetry retention period.

The [ShineServer privacy policy](https://shineserver.growatt.com/pdf/html/privacy-policy-en.html)
lists device/product data retention during device connection and deletion or
anonymisation after an additional 90 days following unbinding. It separately lists
station-data and backup retention. These describe privacy/data-lifecycle policy;
they do not promise how much minute-level telemetry the SPH/MIX history endpoint
will return. A backup-retention duration is not an API lookback duration.

## What a bounded probe can conclude

An accepted nonempty historical day establishes that records for that day are
currently retrievable for the selected device/account through the allowed
endpoint. A few successful day probes can establish a minimum observed lookback,
not the earliest available date or completeness of intervening days.

An empty day alone cannot establish a retention boundary: missing collection,
installation age, outages, timezone selection or other gaps can also produce no
records. An error or throttled request is likewise not an empty-history result.
Discovering a boundary needs a separate bounded policy with explicit unavailable,
failed, complete and deferred outcomes; this note does not implement a scan or
scheduler. No universal retention guarantee has been established by the sources
successfully read here.

