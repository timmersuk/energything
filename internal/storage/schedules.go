package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Schedule owns progress independently of retained observations. For calendar
// scans SearchLimit is the current annual target, never an archive stopping date.
type Schedule struct {
	ConfiguredBoundary bool          `json:"-"`
	SearchPolicy       string        `json:"search_policy"`
	IdentityID         int64         `json:"identity_id"`
	Kind               string        `json:"kind"`
	Policy             string        `json:"policy"`
	Slot               int           `json:"slot"`
	Frontier           Instant       `json:"frontier"`
	SearchLimit        Instant       `json:"search_limit"`
	Interval           time.Duration `json:"-"`
	NextDue            Instant       `json:"next_due"`
	CurrentJob         *int64        `json:"current_job"`
	AfterAttempt       int64         `json:"-"`
	State              string        `json:"state"`
	Provenance         string        `json:"provenance"`
}

func (s *Store) Identities(ctx context.Context) ([]Identity, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,provider,stream,stable_key,selected,metadata FROM identities ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Identity{}
	for rows.Next() {
		var i Identity
		var m string
		if e = rows.Scan(&i.ID, &i.Provider, &i.Stream, &i.StableKey, &i.Selected, &m); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(m), &i.Metadata); e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

const scheduleSelect = `SELECT identity_id,kind,policy,slot,frontier,search_limit,interval_us,next_due,current_job,after_attempt,state,provenance,search_policy FROM schedules`

func scanSchedule(row scanner) (p Schedule, e error) {
	var us int64
	e = row.Scan(&p.IdentityID, &p.Kind, &p.Policy, &p.Slot, &p.Frontier, &p.SearchLimit, &us, &p.NextDue, &p.CurrentJob, &p.AfterAttempt, &p.State, &p.Provenance, &p.SearchPolicy)
	p.Interval = time.Duration(us) * time.Microsecond
	return
}
func (s *Store) QuerySchedules(ctx context.Context) ([]Schedule, error) {
	rows, e := s.db.QueryContext(ctx, scheduleSelect+` ORDER BY identity_id,policy,kind,slot`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		p, e := scanSchedule(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PrepareSchedule coalesces routine triggers and resumes a single historical
// frontier. The planner runs inside the transaction, and must do no I/O.
func (s *Store) PrepareSchedule(ctx context.Context, seed Schedule, now Instant, force bool, plan func(Schedule) Job) (*Job, error) {
	var out *Job
	if seed.SearchPolicy == "" {
		seed.SearchPolicy = "bounded"
	}
	e := s.write(ctx, func(tx *sql.Tx) error {
		if seed.Interval <= 0 || seed.IdentityID <= 0 || seed.Frontier < seed.SearchLimit || (seed.SearchPolicy != "bounded" && seed.SearchPolicy != "calendar") || (seed.SearchPolicy == "calendar" && seed.Policy != "backfill") {
			return ErrInvalid
		}
		_, e := tx.ExecContext(ctx, `INSERT INTO schedules(identity_id,kind,policy,slot,frontier,search_limit,interval_us,next_due,provenance,search_policy) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, seed.IdentityID, seed.Kind, seed.Policy, seed.Slot, seed.Frontier, seed.SearchLimit, seed.Interval.Microseconds(), now, seed.Provenance, seed.SearchPolicy)
		if e != nil {
			return e
		}
		args := []any{seed.IdentityID, seed.Kind, seed.Policy, seed.Slot}
		p, e := scanSchedule(tx.QueryRowContext(ctx, scheduleSelect+` WHERE identity_id=? AND kind=? AND policy=? AND slot=?`, args...))
		if e != nil {
			return e
		}
		update := func() error {
			_, e := tx.ExecContext(ctx, `UPDATE schedules SET frontier=?,next_due=?,current_job=?,after_attempt=?,state=?,interval_us=?,search_limit=?,provenance=?,search_policy=? WHERE identity_id=? AND kind=? AND policy=? AND slot=?`, append([]any{p.Frontier, p.NextDue, p.CurrentJob, p.AfterAttempt, p.State, seed.Interval.Microseconds(), p.SearchLimit, p.Provenance, p.SearchPolicy}, args...)...)
			return e
		}
		// Apply current configuration to persisted plans without resetting progress.
		if seed.ConfiguredBoundary {
			p.SearchPolicy, p.SearchLimit, p.Provenance = seed.SearchPolicy, seed.SearchLimit, seed.Provenance
			if p.CurrentJob != nil {
				j, err := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id=?`, *p.CurrentJob))
				if err != nil {
					return err
				}
				// Leave in-flight work owned until it finishes; replace queued work
				// crossing the new boundary with a freshly bounded unit.
				if j.Start < p.SearchLimit && j.RunningAttempt == nil {
					var accepted bool
					if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM attempts WHERE job_id=? AND id>? AND outcome IN ('success','empty'))`, j.ID, p.AfterAttempt).Scan(&accepted); err != nil {
						return err
					}
					if !accepted {
						p.CurrentJob = nil
					}
				}
			}
		}
		if p.CurrentJob != nil {
			j, e := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id=?`, *p.CurrentJob))
			if e != nil {
				return e
			}
			var accepted int64
			if e = tx.QueryRowContext(ctx, `SELECT coalesce(max(id),0) FROM attempts WHERE job_id=? AND outcome IN ('success','empty')`, j.ID).Scan(&accepted); e != nil {
				return e
			}
			if accepted <= p.AfterAttempt || j.RunningAttempt != nil {
				out = &j
				p.NextDue = j.NextDue
				p.State = "queued"
				if j.RetryCount > 0 {
					p.State = "retry_due"
				}
				if j.Paused {
					p.State = "paused"
				}
				if j.RunningAttempt != nil {
					p.State = "running"
				}
				return update()
			}
			p.CurrentJob = nil
			if p.Policy == "backfill" {
				p.Frontier = j.Start
				p.NextDue = now
			} else {
				p.NextDue = j.NextDue + Instant(seed.Interval.Microseconds())
			}
		}
		if p.Policy == "backfill" && p.Frontier <= p.SearchLimit {
			if p.SearchPolicy == "calendar" {
				frontier := time.UnixMicro(int64(p.Frontier)).UTC()
				if frontier.Year() == 1 && frontier.Month() == time.January && frontier.Day() == 1 {
					p.State = "calendar_domain_scanned"
					return update()
				}
				p.SearchLimit = At(time.Date(frontier.Year()-1, time.January, 1, 0, 0, 0, 0, time.UTC))
			} else {
				p.State = "search_limit_boundary_unresolved"
				if seed.ConfiguredBoundary {
					p.State = "configured_boundary_reached"
				}
				var bounded bool
				if e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM history_boundaries WHERE identity_id=? AND oldest= ? AND state='observed')`, p.IdentityID, p.SearchLimit).Scan(&bounded); e != nil {
					return e
				}
				if bounded {
					p.State = "complete_at_observed_boundary"
				}
				return update()
			}
		}
		if p.Policy == "routine" && !force && p.NextDue > now {
			return update()
		}
		j := plan(p)
		j.IdentityID = p.IdentityID
		j.Kind = p.Kind
		j.Policy = p.Policy
		j.NextDue = now
		jobs, e := ensureJobs(ctx, tx, []Job{j})
		if e != nil {
			return e
		}
		j = jobs[0]
		// Adopt an already accepted historical unit; the next tick advances it.
		if p.Policy == "routine" {
			if e = tx.QueryRowContext(ctx, `SELECT coalesce(max(id),0) FROM attempts WHERE job_id=?`, j.ID).Scan(&p.AfterAttempt); e != nil {
				return e
			}
			if j.RunningAttempt == nil && !j.Paused {
				if _, e = tx.ExecContext(ctx, `UPDATE jobs SET next_due=? WHERE id=?`, now, j.ID); e != nil {
					return e
				}
				j.NextDue = now
			}
		}
		p.CurrentJob = &j.ID
		p.NextDue = j.NextDue
		p.State = "active"
		out = &j
		return update()
	})
	return out, e
}

// ScheduledJobs excludes accepted stale routine windows and orders routine work
// ahead of historical repair. Only the server executes these claims.
func (s *Store) ScheduledJobs(ctx context.Context, now Instant) ([]Job, error) {
	rows, e := s.db.QueryContext(ctx, jobSelect+` WHERE paused=0 AND running_attempt IS NULL AND next_due<=? AND identity_id IN(SELECT id FROM identities WHERE selected=1) AND ((policy='range' AND checkpoint IS NULL) OR id IN(SELECT current_job FROM schedules WHERE current_job IS NOT NULL AND NOT EXISTS(SELECT 1 FROM attempts a WHERE a.job_id=current_job AND a.id>after_attempt AND a.outcome IN('success','empty')))) ORDER BY CASE policy WHEN 'routine' THEN 0 WHEN 'range' THEN 1 ELSE 2 END,next_due,id`, now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
