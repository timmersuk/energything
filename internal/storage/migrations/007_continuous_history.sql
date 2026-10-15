ALTER TABLE schedules ADD COLUMN search_policy TEXT NOT NULL DEFAULT 'bounded'
 CHECK(search_policy IN('bounded','calendar'));

-- Preserve every current unit, retry, checkpoint and frontier. A previous
-- engineering lookback is replaced by a calendar target that keeps expanding.
UPDATE schedules SET
 search_policy='calendar',
 search_limit=CAST(strftime('%s',strftime('%Y-01-01',frontier/1000000,'unixepoch')) AS INTEGER)*1000000,
 state=CASE WHEN state='search_limit_boundary_unresolved' THEN 'active' ELSE state END,
 provenance='Continuous backward calendar acquisition; annual targets extend automatically; empty responses and zeros are not archive boundaries'
WHERE policy='backfill' AND NOT EXISTS(
 SELECT 1 FROM history_boundaries b WHERE b.identity_id=schedules.identity_id AND b.oldest IS NOT NULL
);

UPDATE schedules SET provenance='Automatic recent collection; independent of historical acquisition'
WHERE policy='routine';
