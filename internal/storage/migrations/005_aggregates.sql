CREATE TABLE energy_aggregates (
 identity_id INTEGER NOT NULL REFERENCES identities(id), metric TEXT NOT NULL,
 resolution TEXT NOT NULL, period TEXT NOT NULL, value TEXT NOT NULL, unit TEXT NOT NULL,
 calendar TEXT NOT NULL, quality TEXT NOT NULL, retrieved INTEGER NOT NULL,
 retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,metric,resolution,period)
);
CREATE INDEX aggregates_period ON energy_aggregates(period,identity_id);
CREATE TABLE aggregate_versions (
 id INTEGER PRIMARY KEY, identity_id INTEGER NOT NULL REFERENCES identities(id),
 metric TEXT NOT NULL, resolution TEXT NOT NULL, period TEXT NOT NULL, value TEXT NOT NULL,
 unit TEXT NOT NULL, calendar TEXT NOT NULL, quality TEXT NOT NULL,
 retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 UNIQUE(identity_id,metric,resolution,period,retrieval_id)
);
CREATE TABLE aggregate_calls (reserved INTEGER NOT NULL);
CREATE INDEX aggregate_calls_time ON aggregate_calls(reserved);
