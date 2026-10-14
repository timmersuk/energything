CREATE TABLE identities (
 id INTEGER PRIMARY KEY, provider TEXT NOT NULL CHECK(provider IN ('octopus','growatt')),
 stream TEXT NOT NULL, stable_key TEXT NOT NULL, selected INTEGER NOT NULL CHECK(selected IN (0,1)),
 metadata TEXT NOT NULL, UNIQUE(provider,stream,stable_key)
);
CREATE TABLE range_requests (request_id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL);
CREATE TABLE jobs (
 id INTEGER PRIMARY KEY, identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, policy TEXT NOT NULL, start INTEGER NOT NULL, end INTEGER NOT NULL CHECK(end>start),
 provider_date TEXT NOT NULL, request_id TEXT REFERENCES range_requests(request_id),
 next_due INTEGER NOT NULL, checkpoint INTEGER, retry_count INTEGER NOT NULL DEFAULT 0,
 paused INTEGER NOT NULL DEFAULT 0, running_attempt INTEGER REFERENCES attempts(id)
);
CREATE UNIQUE INDEX jobs_identity ON jobs(identity_id,kind,policy,start,end,provider_date,coalesce(request_id,''));
CREATE TABLE attempts (
 id INTEGER PRIMARY KEY, job_id INTEGER NOT NULL REFERENCES jobs(id), identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, start INTEGER NOT NULL, end INTEGER NOT NULL, provider_date TEXT NOT NULL,
 started INTEGER NOT NULL, occurred INTEGER NOT NULL, finished INTEGER,
 outcome TEXT NOT NULL CHECK(outcome IN ('running','success','empty','rejected','failed','cancelled','interrupted')),
 error_class TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '', message TEXT NOT NULL DEFAULT '',
 page INTEGER, retry_count INTEGER NOT NULL, next_retry INTEGER, retry_details TEXT NOT NULL DEFAULT '[]',
 accepted_rows INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX attempts_one_running ON attempts(job_id) WHERE outcome='running';
CREATE INDEX attempts_occurrence ON attempts(occurred DESC,id DESC);
CREATE INDEX attempts_source_time ON attempts(identity_id,occurred DESC,id DESC);
CREATE INDEX attempts_outcome_time ON attempts(outcome,occurred DESC,id DESC);
CREATE INDEX jobs_due ON jobs(paused,next_due,id) WHERE running_attempt IS NULL;
CREATE TABLE evidence (id INTEGER PRIMARY KEY, digest TEXT NOT NULL UNIQUE, content TEXT NOT NULL);
CREATE TABLE retrievals (
 id INTEGER PRIMARY KEY, evidence_id INTEGER NOT NULL REFERENCES evidence(id),
 identity_id INTEGER NOT NULL REFERENCES identities(id), attempt_id INTEGER NOT NULL REFERENCES attempts(id),
 retrieved INTEGER NOT NULL, complete INTEGER NOT NULL CHECK(complete IN (0,1)),
 UNIQUE(attempt_id,evidence_id,retrieved,complete)
);
CREATE INDEX retrievals_attempt ON retrievals(attempt_id,id);
CREATE INDEX retrievals_source_time ON retrievals(identity_id,retrieved);
CREATE TABLE intervals (
 identity_id INTEGER NOT NULL REFERENCES identities(id), start INTEGER NOT NULL, end INTEGER NOT NULL CHECK(end>start),
 kwh TEXT NOT NULL, retrieved INTEGER NOT NULL, retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,start)
);
CREATE INDEX intervals_time ON intervals(start,identity_id);
CREATE TABLE interval_versions (
 id INTEGER PRIMARY KEY, identity_id INTEGER NOT NULL REFERENCES identities(id), start INTEGER NOT NULL, end INTEGER NOT NULL,
 kwh TEXT NOT NULL, retrieval_id INTEGER NOT NULL REFERENCES retrievals(id), UNIQUE(identity_id,start,retrieval_id)
);
CREATE TABLE samples (
 id INTEGER PRIMARY KEY, identity_id INTEGER NOT NULL REFERENCES identities(id), resolved_key TEXT,
 attempt_id INTEGER NOT NULL REFERENCES attempts(id), ordinal INTEGER NOT NULL, instant INTEGER,
 provider_date TEXT NOT NULL, original_time TEXT NOT NULL, quality TEXT NOT NULL, values_json TEXT NOT NULL,
 retrieved INTEGER NOT NULL, retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 UNIQUE(identity_id,resolved_key), UNIQUE(attempt_id,ordinal)
);
CREATE INDEX samples_source_time ON samples(identity_id,instant,id);
CREATE INDEX samples_time ON samples(instant,id);
CREATE INDEX samples_quality_date ON samples(quality,provider_date,retrieved,id);
CREATE INDEX samples_retrieved ON samples(retrieved,id);
CREATE TABLE sample_versions (
 id INTEGER PRIMARY KEY, sample_id INTEGER NOT NULL REFERENCES samples(id), retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 resolved_key TEXT, instant INTEGER, original_time TEXT NOT NULL, quality TEXT NOT NULL, values_json TEXT NOT NULL,
 UNIQUE(sample_id,retrieval_id)
);
CREATE TABLE completions (
 job_id INTEGER NOT NULL REFERENCES jobs(id), start INTEGER NOT NULL, end INTEGER NOT NULL,
 attempt_id INTEGER NOT NULL REFERENCES attempts(id), fetched INTEGER NOT NULL,
 expected INTEGER NOT NULL, valid INTEGER NOT NULL, missing INTEGER NOT NULL, qualified INTEGER NOT NULL,
 measurement_complete INTEGER NOT NULL, PRIMARY KEY(job_id,start,end)
);
