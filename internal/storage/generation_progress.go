package storage

import (
	"context"
	"database/sql"
	"encoding/json"
)

// GenerationProgress projects automatic history and its exact retained current
// job. It does not depend on the capped recent job/coverage lists used by status.
type GenerationProgress struct {
	Plan        Schedule
	Identity    Identity
	CurrentJob  *Job
	LastAttempt *Audit
}

func (s *Store) QueryGenerationProgress(ctx context.Context, plantID string) ([]GenerationProgress, error) {
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, scheduleSelect+` WHERE kind='aggregate' AND policy='backfill' AND identity_id IN (SELECT id FROM identities WHERE provider='growatt' AND stream='generation') ORDER BY identity_id`)
	if e != nil {
		return nil, e
	}
	out := []GenerationProgress{}
	for rows.Next() {
		p, e := scanSchedule(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, GenerationProgress{Plan: p})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	filtered := []GenerationProgress{}
	for _, p := range out {
		var metadata string
		e = tx.QueryRowContext(ctx, `SELECT id,provider,stream,selected,metadata FROM identities WHERE id=?`, p.Plan.IdentityID).Scan(&p.Identity.ID, &p.Identity.Provider, &p.Identity.Stream, &p.Identity.Selected, &metadata)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(metadata), &p.Identity.Metadata); e != nil {
			return nil, e
		}
		if plantID != "" && p.Identity.Metadata.PlantID != plantID {
			continue
		}
		if p.Plan.CurrentJob != nil {
			job, e := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id=? AND identity_id=?`, *p.Plan.CurrentJob, p.Identity.ID))
			if e != nil {
				return nil, e
			}
			p.CurrentJob = &job
		}
		a, e := scanAudit(tx.QueryRowContext(ctx, auditSelect+` WHERE a.identity_id=? ORDER BY occurred DESC,a.id DESC LIMIT 1`, p.Identity.ID))
		if e != nil && e != sql.ErrNoRows {
			return nil, e
		}
		if e == nil {
			p.LastAttempt = &a
		}
		filtered = append(filtered, p)
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return filtered, nil
}
