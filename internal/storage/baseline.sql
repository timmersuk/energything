-- Fresh-install schema at version 8. No development migrations are executed.
-- Keep legacy migrations for existing databases; post-baseline upgrades still apply.

CREATE TABLE aggregate_calls (reserved INTEGER NOT NULL);

CREATE TABLE attempts (
 id INTEGER PRIMARY KEY, job_id INTEGER NOT NULL REFERENCES jobs(id), identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, start INTEGER NOT NULL, end INTEGER NOT NULL, provider_date TEXT NOT NULL,
 started INTEGER NOT NULL, occurred INTEGER NOT NULL, finished INTEGER,
 outcome TEXT NOT NULL CHECK(outcome IN ('running','success','empty','rejected','failed','cancelled','interrupted')),
 error_class TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '', message TEXT NOT NULL DEFAULT '',
 page INTEGER, retry_count INTEGER NOT NULL, next_retry INTEGER, retry_details TEXT NOT NULL DEFAULT '[]',
 accepted_rows INTEGER NOT NULL DEFAULT 0
, diagnostic TEXT NOT NULL DEFAULT '{}');

CREATE TABLE completions (
 job_id INTEGER NOT NULL REFERENCES jobs(id), start INTEGER NOT NULL, end INTEGER NOT NULL,
 attempt_id INTEGER NOT NULL REFERENCES attempts(id), fetched INTEGER NOT NULL,
 expected INTEGER NOT NULL, valid INTEGER NOT NULL, missing INTEGER NOT NULL, qualified INTEGER NOT NULL,
 measurement_complete INTEGER NOT NULL, PRIMARY KEY(job_id,start,end)
);

CREATE TABLE discovery_states (
 provider TEXT NOT NULL CHECK(provider='octopus'),
 stream TEXT NOT NULL CHECK(stream IN ('import','export')),
 state TEXT NOT NULL CHECK(state IN ('discovering','selected','unavailable','selection_required','conflict','failed')),
 occurred INTEGER NOT NULL, failure TEXT NOT NULL DEFAULT '{}',
 PRIMARY KEY(provider,stream)
);

CREATE TABLE energy_aggregates (
 identity_id INTEGER NOT NULL REFERENCES identities(id), metric TEXT NOT NULL,
 resolution TEXT NOT NULL, period TEXT NOT NULL, value TEXT NOT NULL, unit TEXT NOT NULL,
 calendar TEXT NOT NULL, quality TEXT NOT NULL, retrieved INTEGER NOT NULL,
 retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,metric,resolution,period)
);

CREATE TABLE evidence (id INTEGER PRIMARY KEY, digest TEXT NOT NULL UNIQUE, content TEXT NOT NULL);

CREATE TABLE growatt_discovery (
 stream TEXT PRIMARY KEY CHECK(stream IN('telemetry','generation')),
 state TEXT NOT NULL CHECK(state IN('discovering','selected','unavailable','selection_required','failed')),
 occurred INTEGER NOT NULL,
 failure TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE history_boundaries (
 identity_id INTEGER PRIMARY KEY REFERENCES identities(id),
 oldest INTEGER, observed INTEGER NOT NULL, next_due INTEGER NOT NULL,
 state TEXT NOT NULL, provenance TEXT NOT NULL, evidence TEXT NOT NULL
);

CREATE TABLE identities (
 id INTEGER PRIMARY KEY, provider TEXT NOT NULL CHECK(provider IN ('octopus','growatt')),
 stream TEXT NOT NULL, stable_key TEXT NOT NULL, selected INTEGER NOT NULL CHECK(selected IN (0,1)),
 metadata TEXT NOT NULL, UNIQUE(provider,stream,stable_key)
);

CREATE TABLE intervals (
 identity_id INTEGER NOT NULL REFERENCES identities(id), start INTEGER NOT NULL, end INTEGER NOT NULL CHECK(end>start),
 kwh TEXT NOT NULL, retrieved INTEGER NOT NULL, retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,start)
);

CREATE TABLE jobs (
 id INTEGER PRIMARY KEY, identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, policy TEXT NOT NULL, start INTEGER NOT NULL, end INTEGER NOT NULL CHECK(end>start),
 provider_date TEXT NOT NULL, request_id TEXT REFERENCES range_requests(request_id),
 next_due INTEGER NOT NULL, checkpoint INTEGER, retry_count INTEGER NOT NULL DEFAULT 0,
 paused INTEGER NOT NULL DEFAULT 0, running_attempt INTEGER REFERENCES attempts(id)
);

CREATE TABLE range_requests (request_id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL);

CREATE TABLE retrievals (
 id INTEGER PRIMARY KEY, evidence_id INTEGER NOT NULL REFERENCES evidence(id),
 identity_id INTEGER NOT NULL REFERENCES identities(id), attempt_id INTEGER NOT NULL REFERENCES attempts(id),
 retrieved INTEGER NOT NULL, complete INTEGER NOT NULL CHECK(complete IN (0,1)),
 UNIQUE(attempt_id,evidence_id,retrieved,complete)
);

CREATE TABLE "samples" (
 identity_id INTEGER NOT NULL REFERENCES identities(id),
 provider_date TEXT NOT NULL,
 provider_time TEXT NOT NULL CHECK(length(provider_time)=19 AND substr(provider_time,1,10)=provider_date),
 attempt_id INTEGER NOT NULL REFERENCES attempts(id),
 instant INTEGER, original_time TEXT NOT NULL, quality TEXT NOT NULL,
 values_json TEXT NOT NULL, retrieved INTEGER NOT NULL,
 retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,provider_date,provider_time)
);

CREATE TABLE schedules (
 identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, policy TEXT NOT NULL, slot INTEGER NOT NULL DEFAULT 0,
 frontier INTEGER NOT NULL, search_limit INTEGER NOT NULL,
 interval_us INTEGER NOT NULL, next_due INTEGER NOT NULL,
 current_job INTEGER REFERENCES jobs(id),
 after_attempt INTEGER NOT NULL DEFAULT 0,
 state TEXT NOT NULL DEFAULT 'active',
 provenance TEXT NOT NULL, search_policy TEXT NOT NULL DEFAULT 'bounded'
 CHECK(search_policy IN('bounded','calendar')),
 PRIMARY KEY(identity_id,kind,policy,slot)
);

CREATE INDEX aggregate_calls_time ON aggregate_calls(reserved);

CREATE INDEX aggregates_period ON energy_aggregates(period,identity_id);

CREATE INDEX attempts_occurrence ON attempts(occurred DESC,id DESC);

CREATE UNIQUE INDEX attempts_one_running ON attempts(job_id) WHERE outcome='running';

CREATE INDEX attempts_outcome_time ON attempts(outcome,occurred DESC,id DESC);

CREATE INDEX attempts_source_time ON attempts(identity_id,occurred DESC,id DESC);

CREATE INDEX intervals_time ON intervals(start,identity_id);

CREATE INDEX jobs_due ON jobs(paused,next_due,id) WHERE running_attempt IS NULL;

CREATE UNIQUE INDEX jobs_identity ON jobs(identity_id,kind,policy,start,end,provider_date,coalesce(request_id,''));

CREATE INDEX retrievals_attempt ON retrievals(attempt_id,id);

CREATE INDEX retrievals_source_time ON retrievals(identity_id,retrieved);

CREATE INDEX samples_quality_date ON samples(quality,provider_date,retrieved);

CREATE INDEX samples_retrieved ON samples(retrieved);

CREATE INDEX samples_source_time ON samples(identity_id,instant);

CREATE INDEX samples_time ON samples(instant);
