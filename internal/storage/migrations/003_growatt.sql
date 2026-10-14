CREATE TABLE growatt_discovery (
 stream TEXT PRIMARY KEY CHECK(stream='telemetry'),
 state TEXT NOT NULL CHECK(state IN ('discovering','selected','unavailable','selection_required','failed')),
 occurred INTEGER NOT NULL
);
ALTER TABLE attempts ADD COLUMN diagnostic TEXT NOT NULL DEFAULT '{}';
