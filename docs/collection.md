# Automatic collection and recovery

Task06 implementation introduces migration006 and two independent in-process
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

## Historical boundary gate remains open

Automatic exploration currently has a **120-calendar-day engineering budget**,
anchored once per identity at bootstrap, plus the bootstrap current day for Growatt.
Octopus steps backward in at most14 absolute days; detailed Growatt in one date;
generation in seven inclusive dates, shortened at the limit. Every unit is fetched
and validated completely. Empty gaps do not stop progression; generated zeros do
not establish availability. At the limit the persisted state is
`search_limit_boundary_unresolved`, never archive complete. Restarts retain the
same target/frontier and do not restart the search. Repair can request older data.

This is a limited exploratory recovery policy, **not all discoverably available
history**, and is not a verified provider-supported lifetime domain. Task06 cannot
be declared complete while this gate remains unresolved. The 15 October detailed
MIX observation of about95 days is dated, installation-scoped evidence only;
aggregate history is independent and known to include older dates.

The freshly reviewed [Octopus endpoint documentation](https://developer.octopus.energy/guides/rest/api-endpoints/)
supports ascending period ordering and bounded pagination, but warns that omitted
period bounds cause an assumed range. An unbounded-date one-record probe therefore
cannot yet prove the meter's lifetime boundary. Concrete follow-up: establish and
verify an explicit supported date domain/earliest metadata contract, then test a
one-record oldest probe across gaps and identities before replacing the budget.
Growatt needs authoritative aggregate/detail retention or a supported finite
calendar domain; plant creation, retained observations and empty probes are not
substitutes. No binary search or consecutive-empty stopping rule is used.

## Retry ownership and quotas

Octopus and detailed Growatt clients retain their existing maximum three HTTP
transmissions/page, with bounded inline delays and cancellable requests. Every
Growatt discovery/detailed transmission is paced at least ten seconds apart,
including pages and inline retries. Historical units yield between days.
Scheduler reattempts create new attempt IDs and defer one minute doubling to one
hour, plus0–1 second jitter. Provider DeferredUntil is a lower bound; long waits
are stored as job due times rather than holding a provider lane. Authentication and
configuration pause the affected job. A process restart rediscovering configuration
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
and never fetches. SQLite immediate transactions serialize cross-process enqueue
and claims. The running server performs all provider requests.

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
