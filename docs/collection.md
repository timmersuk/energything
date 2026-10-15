# Automatic collection and recovery

Task06 implementation introduces migrations006 and007 and two independent in-process
provider lanes. Discovery runs asynchronously after storage opens and HTTP binds.
Credentials and confirmed selections are sufficient; no initialization switch or
historical-start environment setting is used. Browsing reads local storage only.

Each selected Octopus import/export identity owns independent routine and history
plans. Each selected Growatt device owns latest, three recent provider-day routine
plans and detailed history; the selected plant owns separate generation routine
and history plans. Existing rows in any stream do not suppress another bootstrap.

Routine priority is reconsidered after every bounded unit. Octopus refreshes the
last seven absolute days through the latest elapsed half hour every hour. Growatt
latest runs every five minutes, each of today and the previous two provider dates
every hour. Generation refreshes seven inclusive dates every24h. Configured
positive intervals override the detailed/Octopus defaults. One provider outage
cannot hold the other provider lane or HTTP browsing.

## Durable work

Schedules retain identity, kind, policy, slot, current unit, accepted-attempt marker,
frontier, search limit, due time, state and provenance. Immutable jobs retain ranges,
retry/pause/running state and accepted checkpoints. Acceptance atomically commits
observations, evidence, completion, checkpoint and terminal audit. The next planner
transaction derives the frontier only from that accepted checkpoint, so a crash
between acceptance and planning cannot skip or repeat the accepted unit. Empty
accepted units advance fetched progress, not measurement coverage. Rejections and
failures preserve accepted rows/checkpoints and retry the same unit. Identical
previously accepted historical units are adopted. Partial overlapping legacy
windows may be reacquired; arbitrary retained rows never imply fetched coverage.

Routine requests coalesce against queued/running current units. The selected job
runs at most once per lane; its storage claim forbids overlap. Startup recovery
marks old running attempts interrupted and requeues their unfinished units.
Shutdown cancels HTTP/provider waits, joins both lanes, then closes storage. Each
collector has at most five seconds for cancellation audit cleanup.

## Continuous acquisition of the whole available history

There is no120-day lookback cap. Each selected stream acquires history backwards
from recent dates, using one detailed provider date or seven inclusive generation
dates per acquisition unit. Octopus uses at most14 absolute days per unit. The
scheduler yields between units, and retained progress survives restart.

When no authoritative oldest-reading boundary is known, `search_policy=calendar`
means the stored `search_limit` is the **current calendar target**, not a stopping
date. The first target is January1 of the current year. Reaching it automatically
extends the target to January1 of the preceding year in the same transaction that
plans the next bounded unit. This repeats through older years. Empty responses,
zero aggregates, the observed roughly95-day telemetry expiry edge, plant creation,
and old locally retained rows never stop or truncate that scan.

Consequently every successively older calendar period is eventually requested,
including older nonempty data after long empty gaps. The only date-domain guard is
January1 of year0001, the lower limit of the application's four-digit date/storage
representation. Reaching it records `calendar_domain_scanned`, which describes
requests made, not proof of physical measurement completeness or a provider-wide
retention promise. A provider date rejection is a failed unit, not a new oldest
boundary; it remains visible with its retry/pause state. Newly established source
semantics can narrow the scan to a verified observed boundary, as for Octopus.

Migration007 converts the old limited plans automatically, including plans already
stopped at the old cap. It preserves their frontier, current job/range, retry state,
accepted observations/versions/evidence, completion checkpoints and quota ledger.
It does not start over at the newest sample. Existing routine work and explicit
repair requests retain their independent policies.

Generation history has a separate frontier from device telemetry and retains each
day separately. Its documented quota remains five-minute spacing and ten local
reservations per rolling24h. With one routine request and single-page historical
units, nine historical requests can cover up to63 older dates per rolling24h;
errors, pagination, contention and quota deferrals can reduce that rate. Several
years can therefore take weeks to populate. No operator backfill date or manual
repair command is needed to progress beyond120 days.

Detailed readings, including SOC, are retained first while still accessible. A
provider-expired detailed sample cannot be recovered from a daily aggregate or
manufactured from power. The dated95-day observation remains installation-specific
availability evidence, not a configured scan limit or a claim about aggregate
retention. The official aggregate/history contracts do not expose a verified
all-time oldest-reading index, so the status stays honest about unknown source
lifetime boundaries while acquisition continues.

The freshly reviewed [Octopus endpoint documentation](https://developer.octopus.energy/guides/rest/api-endpoints/)
supports ascending period ordering and bounded pagination, but warns that omitted
period bounds cause an assumed range. Task06 therefore uses **explicit** UTC
`period_from=0001-01-01T00:00:00Z`, elapsed `period_to`, `page_size=1` and
`order_by=period`. One bounded private request per selected import/export stream
accepted these bounds on15 October2026. This verifies the canonical supported
storage-date domain for this installation, not a provider-wide retention promise.
The server repeats that one-page metadata probe at most weekly per identity,
validates its returned half-hour reading and persists the observed oldest bound,
observation time, provenance and selected evidence without ingesting it. No next
page is downloaded and the probe cannot become an unbounded observation scan.
A fully validated nonempty oldest response bounds backward acquisition through
`complete_at_observed_boundary`. Empty probe results establish no earliest date.
Failed probes leave recent collection running; a new older boundary extends the
existing frontier without restarting newer accepted units. Transport inline
retry ownership is unchanged; transient probe failures defer at least15 minutes,
long provider deferrals prevail, and authentication/configuration pauses until
successful startup discovery after configuration correction. If no verified bound
is available, the calendar scan continues backwards without a fixed-age stop.
Unknown Growatt earliest-history metadata no longer blocks older acquisition.
No binary search, consecutive-empty cutoff or plant-creation assumption is used.

## Retry ownership and quotas

Octopus and detailed Growatt clients retain their existing maximum three HTTP
transmissions/page, with bounded inline delays and cancellable requests. Every
Growatt discovery/detailed transmission is paced at least ten seconds apart,
including pages and inline retries. Historical units yield between days.
Scheduler reattempts create new attempt IDs and defer one minute doubling to one
hour, plus0–1 second jitter. Provider DeferredUntil is a lower bound; long waits
are stored as job due times rather than holding a provider lane. Transient discovery errors retry after at least15 minutes (or a later provider
deferral), independently of healthy streams; a plant can collect generation while
device discovery retries. Authentication and
configuration pause the affected job; permanent discovery failures also pause
automatic discovery until restart, with a durable class/code diagnostic. A process restart rediscovering configuration
is the recovery mechanism; audit and retained observations remain intact.

Generation has exactly one transmission/page, no inline retries. All attempts share
the existing durable reservation ledger: at least five minutes and at most ten
reservations per rolling24h, including failed transmissions. Local deferrals leave
the lane available. Other databases/software are outside that ledger and must be
coordinated separately. No feed is added to another feed or inferred from power.

## Refresh and exceptional repair

GET /api/v1/status supplies a process-lifetime CSRF token, also embedded in the
status page. Authenticated POST /api/v1/refresh requires X-CSRF-Token, same Origin
when supplied, and a <=1024-byte JSON body such as
`{"source":"growatt","stream":"generation"}`. Optional stream selects import/export
or telemetry/generation; no ranges are accepted. Response202 includes job_ids.
The endpoint only enqueues routine work. Unknown selectors, bad JSON and extra
fields are400; wrong method405; failed token/origin403; authentication401.

Exceptional repair uses the same server queue:

```text
go run ./cmd/enqueue -data-dir <existing-server-directory> -identity <selected-local-id> -from 2026-01-01 -to 2026-01-07 -operation-id repair-unique-id
```

Inclusive dates cover at most31 dates per operation. Octopus dates are UTC bounds;
Growatt dates are preserved provider dates, with UTC bookkeeping bounds. Work splits
into14-day Octopus, one-day detailed, seven-date generation units. The operation ID
binds the immutable ordered identity/range list: identical replays coalesce; changed
bounds reject; a new ID deliberately requests another correction. OpenQueue requires
the current existing schema, performs no migrations or interrupted-attempt recovery,
and never fetches. Run a container server's enqueue command in its Linux lock domain, never a Windows
host executable against a live Docker bind-mounted WAL. SQLite immediate transactions serialize cross-process enqueue
and claims. The running server performs all provider requests. The image includes
`energything-enqueue`; invoke it with docker exec and `-data-dir /data` to share
the server's Linux SQLite lock domain.

## Time interpretation

New detailed acquisitions preserve original timestamp fields and add readable-clock
candidate interpretations under the configured regional zone and fixed UTC+1.
Round-trip candidate enumeration explicitly identifies spring gaps and both autumn
occurrences. Neither a library default, Shanghai calendar epoch nor receipt delay
chooses an occurrence. Candidate interpretation does not invent a stable sample ID
or rewrite old rows; samples remain occurrence-based and excluded from authoritative
absolute-time queries while the seasonal/identity gate is unresolved.

Current installation BST clocks already have strong independent empirical support;
exact generation/counter agreement cannot distinguish UTC versus BST when boundary
hours contain no positive PV. Fixed UTC+1 versus seasonal London and repeated-hour
identity remain open. The existing 26 October2026 10:00 UK automation will collect
post-transition evidence; it was not duplicated or altered. See the preserved time,
history and generation-comparison notes for qualifications and private evidence.
