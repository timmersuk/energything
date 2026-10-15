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
