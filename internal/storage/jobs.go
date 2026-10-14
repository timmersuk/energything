package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func (s *Store) write(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage write: %w", err)
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	if s.fault != nil {
		if err = s.fault("before_commit"); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("storage commit failed; no completion recorded: %w", err)
	}
	return nil
}

func (s *Store) UpsertIdentities(ctx context.Context, in []Identity) ([]Identity, error) {
	out := append([]Identity(nil), in...)
	err := s.write(ctx, func(tx *sql.Tx) error {
		for n, x := range out {
			if (x.Provider != "octopus" && x.Provider != "growatt") || !validText(x.StableKey) || !validText(x.Stream) {
				return ErrInvalid
			}
			if x.Provider == "octopus" && x.Stream != "import" && x.Stream != "export" {
				return ErrInvalid
			}
			metadata, _ := json.Marshal(x.Metadata)
			err := tx.QueryRowContext(ctx, `INSERT INTO identities(provider,stream,stable_key,selected,metadata) VALUES(?,?,?,?,?)
    ON CONFLICT(provider,stream,stable_key) DO UPDATE SET selected=excluded.selected,metadata=excluded.metadata RETURNING id`,
				x.Provider, x.Stream, x.StableKey, x.Selected, string(metadata)).Scan(&out[n].ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func validateJob(x Job) error {
	if x.IdentityID <= 0 || (x.Kind != "history" && x.Kind != "latest") || (x.Policy != "routine" && x.Policy != "backfill" && x.Policy != "range") || !validInstant(x.Start) || !validInstant(x.End) || x.End <= x.Start || !validDate(x.ProviderDate) {
		return ErrInvalid
	}
	return nil
}

func ensureJobs(ctx context.Context, tx *sql.Tx, in []Job) ([]Job, error) {
	out := append([]Job(nil), in...)
	for n, x := range in {
		if err := validateJob(x); err != nil {
			return nil, err
		}
		var provider string
		if err := tx.QueryRowContext(ctx, `SELECT provider FROM identities WHERE id=?`, x.IdentityID).Scan(&provider); err != nil {
			return nil, err
		}
		if provider == "octopus" && (x.End-x.Start > Instant(14*24*60*60*1e6) || x.Start%Instant(30*60*1e6) != 0 || x.End%Instant(30*60*1e6) != 0) {
			return nil, ErrInvalid
		}
		if provider == "growatt" && x.Kind == "history" && (x.ProviderDate == "" || x.End-x.Start > Instant(25*60*60*1e6)) {
			return nil, ErrInvalid
		}
		// Conflicts coalesce without resetting due/checkpoint/retry state.
		var request any
		if x.RequestID != "" {
			request = x.RequestID
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO jobs(identity_id,kind,policy,start,end,provider_date,request_id,next_due) VALUES(?,?,?,?,?,?,?,?)
   ON CONFLICT DO NOTHING`, x.IdentityID, x.Kind, x.Policy, x.Start, x.End, x.ProviderDate, request, x.NextDue)
		if err != nil {
			return nil, err
		}
		row := tx.QueryRowContext(ctx, jobSelect+` WHERE identity_id=? AND kind=? AND policy=? AND start=? AND end=? AND provider_date=? AND coalesce(request_id,'')=?`, x.IdentityID, x.Kind, x.Policy, x.Start, x.End, x.ProviderDate, x.RequestID)
		j, err := scanJob(row)
		if err != nil {
			return nil, err
		}
		if j.RequestID != x.RequestID {
			return nil, ErrConflict
		}
		out[n] = j
	}
	return out, nil
}
func (s *Store) EnsureJobs(ctx context.Context, in []Job) (out []Job, err error) {
	for _, j := range in {
		if j.RequestID != "" {
			return nil, ErrInvalid
		}
	}
	err = s.write(ctx, func(tx *sql.Tx) error { out, err = ensureJobs(ctx, tx, in); return err })
	return
}
func (s *Store) EnsureRange(ctx context.Context, id string, in []Job) (out []Job, err error) {
	if !validText(id) || len(in) == 0 {
		return nil, ErrInvalid
	}
	// Due times are runtime state, not configuration identity.
	type bound struct {
		Identity     int64
		Kind, Policy string
		Start, End   Instant
		Date         string
	}
	bounds := make([]bound, len(in))
	jobs := append([]Job(nil), in...)
	for i, j := range jobs {
		jobs[i].RequestID = id
		bounds[i] = bound{j.IdentityID, j.Kind, j.Policy, j.Start, j.End, j.ProviderDate}
	}
	b, _ := json.Marshal(bounds)
	d := sha256.Sum256(b)
	fingerprint := hex.EncodeToString(d[:])
	err = s.write(ctx, func(tx *sql.Tx) error {
		if _, e := tx.ExecContext(ctx, `INSERT INTO range_requests VALUES(?,?) ON CONFLICT DO NOTHING`, id, fingerprint); e != nil {
			return e
		}
		var stored string
		if e := tx.QueryRowContext(ctx, `SELECT fingerprint FROM range_requests WHERE request_id=?`, id).Scan(&stored); e != nil {
			return e
		}
		if stored != fingerprint {
			return fmt.Errorf("range request ID already binds different work: %w", ErrConflict)
		}
		out, err = ensureJobs(ctx, tx, jobs)
		return err
	})
	return
}

const jobSelect = `SELECT id,identity_id,kind,policy,start,end,provider_date,coalesce(request_id,''),next_due,checkpoint,retry_count,paused,running_attempt FROM jobs`

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (j Job, err error) {
	err = row.Scan(&j.ID, &j.IdentityID, &j.Kind, &j.Policy, &j.Start, &j.End, &j.ProviderDate, &j.RequestID, &j.NextDue, &j.Checkpoint, &j.RetryCount, &j.Paused, &j.RunningAttempt)
	return
}
func (s *Store) DueJobs(ctx context.Context, now Instant, limit int) ([]Job, error) {
	limit, err := pageLimit(limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, jobSelect+` WHERE paused=0 AND running_attempt IS NULL AND next_due<=? AND (policy='routine' OR checkpoint IS NULL OR checkpoint<end) AND identity_id IN (SELECT id FROM identities WHERE selected=1) ORDER BY next_due,id LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
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
func (s *Store) BeginAttempt(ctx context.Context, in AttemptRequest) (id int64, err error) {
	if !validInstant(in.Started) {
		return 0, ErrInvalid
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		j, e := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id=?`, in.JobID))
		if e != nil {
			return e
		}
		var selected bool
		if e = tx.QueryRowContext(ctx, `SELECT selected FROM identities WHERE id=?`, j.IdentityID).Scan(&selected); e != nil {
			return e
		}
		if !selected || j.RunningAttempt != nil || j.Paused {
			return ErrConflict
		}
		e = tx.QueryRowContext(ctx, `INSERT INTO attempts(job_id,identity_id,kind,start,end,provider_date,started,occurred,outcome,retry_count)
   VALUES(?,?,?,?,?,?,?,?,'running',?) RETURNING id`, j.ID, j.IdentityID, j.Kind, j.Start, j.End, j.ProviderDate, in.Started, in.Started, j.RetryCount).Scan(&id)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `UPDATE jobs SET running_attempt=? WHERE id=?`, id, j.ID)
		return e
	})
	return
}
func (s *Store) RecoverInterruptedAttempts(ctx context.Context, now Instant) (count int64, err error) {
	err = s.write(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, `UPDATE attempts SET outcome='interrupted',finished=max(started,?),occurred=max(started,?),error_class='network',error_code='interrupted',message='Collection interrupted by process restart',next_retry=? WHERE outcome='running'`, now, now, now)
		if e != nil {
			return e
		}
		count, e = r.RowsAffected()
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `UPDATE jobs SET running_attempt=NULL,next_due=?,paused=0 WHERE running_attempt IS NOT NULL`, now)
		return e
	})
	return
}

// ResumeJob is for a caller that has resolved a configuration/discovery gate.
func (s *Store) ResumeJob(ctx context.Context, id int64, nextDue Instant) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `UPDATE jobs SET paused=0,next_due=? WHERE id=? AND running_attempt IS NULL`, nextDue, id)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrConflict
		}
		return nil
	})
}
