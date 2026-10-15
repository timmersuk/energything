package storage

import (
	"context"
	"database/sql"
)

// SelectGeneration keeps plant selection independent of device telemetry selection.
func (s *Store) SelectGeneration(ctx context.Context, plant string) (Identity, error) {
	var i Identity
	e := s.write(ctx, func(tx *sql.Tx) error {
		if plant != "" && !validText(plant) {
			return ErrInvalid
		}
		if _, e := tx.ExecContext(ctx, `UPDATE identities SET selected=0 WHERE provider='growatt' AND stream='generation'`); e != nil {
			return e
		}
		state := "selected"
		if plant == "" {
			state = "selection_required"
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO growatt_discovery(stream,state,occurred,failure) VALUES('generation',?,?,'{}') ON CONFLICT(stream) DO UPDATE SET state=excluded.state,occurred=excluded.occurred,failure='{}'`, state, At(s.now())); e != nil {
			return e
		}
		if plant == "" {
			return nil
		}
		metadata, e := marshalDiscovery(DiscoveryMetadata{PlantID: plant})
		if e != nil {
			return e
		}
		i = Identity{Provider: "growatt", Stream: "generation", StableKey: plant, Selected: true, Metadata: DiscoveryMetadata{PlantID: plant}}
		return tx.QueryRowContext(ctx, `INSERT INTO identities(provider,stream,stable_key,selected,metadata) VALUES('growatt','generation',?,1,?) ON CONFLICT(provider,stream,stable_key) DO UPDATE SET selected=1 RETURNING id`, plant, metadata).Scan(&i.ID)
	})
	return i, e
}

// ResumeDiscoveredJobs is called once after startup discovery succeeds, never on
// a timer or a refresh request. Failed authentication stays paused until restart.
func (s *Store) ResumeDiscoveredJobs(ctx context.Context, provider string, now Instant) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `UPDATE history_boundaries SET next_due=? WHERE state='paused' AND identity_id IN(SELECT id FROM identities WHERE provider=? AND selected=1)`, now, provider); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `UPDATE jobs SET paused=0,next_due=? WHERE paused=1 AND running_attempt IS NULL AND identity_id IN(SELECT id FROM identities WHERE provider=? AND selected=1)`, now, provider)
		return e
	})
}
