package storage

import (
	"context"
	"database/sql"
	"encoding/json"
)

type HistoryBoundary struct {
	IdentityID        int64
	Oldest            *Instant
	Observed, NextDue Instant
	State, Provenance string
	Evidence          json.RawMessage
}

func (s *Store) HistoryBoundary(ctx context.Context, id int64) (HistoryBoundary, error) {
	var b HistoryBoundary
	b.IdentityID = id
	var raw string
	e := s.db.QueryRowContext(ctx, `SELECT oldest,observed,next_due,state,provenance,evidence FROM history_boundaries WHERE identity_id=?`, id).Scan(&b.Oldest, &b.Observed, &b.NextDue, &b.State, &b.Provenance, &raw)
	b.Evidence = json.RawMessage(raw)
	return b, e
}
func (s *Store) RecordHistoryBoundary(ctx context.Context, b HistoryBoundary) error {
	content, _, e := canonicalEvidence(b.Evidence)
	if e != nil {
		return e
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `INSERT INTO history_boundaries VALUES(?,?,?,?,?,?,?) ON CONFLICT(identity_id) DO UPDATE SET oldest=excluded.oldest,observed=excluded.observed,next_due=excluded.next_due,state=excluded.state,provenance=excluded.provenance,evidence=excluded.evidence`, b.IdentityID, b.Oldest, b.Observed, b.NextDue, b.State, b.Provenance, content); e != nil {
			return e
		}
		if b.Oldest != nil {
			_, e := tx.ExecContext(ctx, `UPDATE schedules SET search_limit=?,provenance=?,state=CASE WHEN frontier<=? THEN 'complete_at_observed_boundary' ELSE 'active' END WHERE identity_id=? AND policy='backfill'`, *b.Oldest, b.Provenance, *b.Oldest, b.IdentityID)
			return e
		}
		return nil
	})
}
