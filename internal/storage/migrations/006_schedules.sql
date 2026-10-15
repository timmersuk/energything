CREATE TABLE schedules (
 identity_id INTEGER NOT NULL REFERENCES identities(id),
 kind TEXT NOT NULL, policy TEXT NOT NULL, slot INTEGER NOT NULL DEFAULT 0,
 frontier INTEGER NOT NULL, search_limit INTEGER NOT NULL,
 interval_us INTEGER NOT NULL, next_due INTEGER NOT NULL,
 current_job INTEGER REFERENCES jobs(id),
 after_attempt INTEGER NOT NULL DEFAULT 0,
 state TEXT NOT NULL DEFAULT 'active',
 provenance TEXT NOT NULL,
 PRIMARY KEY(identity_id,kind,policy,slot)
);
ALTER TABLE growatt_discovery RENAME TO growatt_discovery_old;
CREATE TABLE growatt_discovery (
 stream TEXT PRIMARY KEY CHECK(stream IN('telemetry','generation')),
 state TEXT NOT NULL CHECK(state IN('discovering','selected','unavailable','selection_required','failed')),
 occurred INTEGER NOT NULL,
 failure TEXT NOT NULL DEFAULT '{}'
);
INSERT INTO growatt_discovery SELECT * FROM growatt_discovery_old;
DROP TABLE growatt_discovery_old;
CREATE TABLE history_boundaries (
 identity_id INTEGER PRIMARY KEY REFERENCES identities(id),
 oldest INTEGER, observed INTEGER NOT NULL, next_due INTEGER NOT NULL,
 state TEXT NOT NULL, provenance TEXT NOT NULL, evidence TEXT NOT NULL
);
ALTER TABLE discovery_states ADD COLUMN failure TEXT NOT NULL DEFAULT '{}';
