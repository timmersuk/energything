package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"strings"
)

type RecordQuery struct {
	IdentityID int64
	Direction  string // Octopus import/export; empty selects both.
	From, To   *Instant
	Limit      int
	Cursor     string
}
type SampleQuery struct {
	RecordQuery
	// QualityView switches ordering/range to retrieval time, allowing null instants.
	QualityView           bool
	Quality, ProviderDate string
}
type AuditQuery struct {
	IdentityID                    int64
	Provider, Outcome, ErrorClass string
	From, To                      *Instant
	Limit                         int
	Cursor                        string
}
type IntervalPage struct {
	Records    []Interval
	NextCursor string
}
type SamplePage struct {
	Records           []Sample
	NextCursor        string
	OmittedUnresolved int64
}
type AuditCounts struct{ Total, Failed, Rejected int64 }
type AuditPage struct {
	Records    []Audit
	NextCursor string
	Counts     AuditCounts
}
type cursor struct {
	Version int
	Binding string
	Time    Instant
	ID      int64
}

func pageLimit(n int) (int, error) {
	if n == 0 {
		return 100, nil
	}
	if n < 1 || n > 500 {
		return 0, ErrInvalid
	}
	return n, nil
}
func rangeValid(from, to *Instant) bool { return from == nil || to == nil || *from < *to }
func binding(v any) string {
	b, _ := json.Marshal(v)
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}
func decodeCursor(raw, bind string) (*cursor, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 1024 {
		return nil, ErrInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	var c cursor
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if !json.Valid(b) || d.Decode(&c) != nil || c.Version != 1 || c.Binding != bind || c.ID <= 0 || !validInstant(c.Time) {
		return nil, ErrInvalid
	}
	return &c, nil
}
func encodeCursor(bind string, t Instant, id int64) string {
	b, _ := json.Marshal(cursor{1, bind, t, id})
	return base64.RawURLEncoding.EncodeToString(b)
}
func predicates(identity int64, from, to *Instant, field string) (string, []any) {
	where := "1=1"
	args := []any{}
	if identity != 0 {
		where += " AND identity_id=?"
		args = append(args, identity)
	}
	if from != nil {
		where += " AND " + field + ">=?"
		args = append(args, *from)
	}
	if to != nil {
		where += " AND " + field + "<?"
		args = append(args, *to)
	}
	return where, args
}
func (s *Store) QueryIntervals(ctx context.Context, q RecordQuery) (page IntervalPage, err error) {
	if q.IdentityID < 0 || !rangeValid(q.From, q.To) || (q.Direction != "" && q.Direction != "import" && q.Direction != "export") {
		return page, ErrInvalid
	}
	q.Limit, err = pageLimit(q.Limit)
	if err != nil {
		return
	}
	raw := q.Cursor
	q.Cursor = ""
	bind := binding(q)
	c, err := decodeCursor(raw, bind)
	if err != nil {
		return page, err
	}
	where, args := predicates(q.IdentityID, q.From, q.To, "start")
	if q.Direction != "" {
		where += " AND identity_id IN (SELECT id FROM identities WHERE provider='octopus' AND stream=?)"
		args = append(args, q.Direction)
	}
	if c != nil {
		where += " AND (start,identity_id)>(?,?)"
		args = append(args, c.Time, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT identity_id,start,end,kwh,retrieved,retrieval_id FROM intervals WHERE `+where+` ORDER BY start,identity_id LIMIT ?`, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Records = []Interval{}
	for rows.Next() {
		var x Interval
		if err = rows.Scan(&x.IdentityID, &x.Start, &x.End, &x.KWh, &x.Retrieved, &x.RetrievalID); err != nil {
			return
		}
		page.Records = append(page.Records, x)
	}
	if err = rows.Err(); err != nil {
		return
	}
	if len(page.Records) > q.Limit {
		page.Records = page.Records[:q.Limit]
		x := page.Records[q.Limit-1]
		page.NextCursor = encodeCursor(bind, x.Start, x.IdentityID)
	}
	return
}
func (s *Store) QuerySamples(ctx context.Context, q SampleQuery) (page SamplePage, err error) {
	if q.IdentityID < 0 || q.Direction != "" || !rangeValid(q.From, q.To) || !validDate(q.ProviderDate) || (q.Quality != "" && !validText(q.Quality)) || (!q.QualityView && (q.Quality != "" || q.ProviderDate != "")) {
		return page, ErrInvalid
	}
	q.Limit, err = pageLimit(q.Limit)
	if err != nil {
		return
	}
	raw := q.Cursor
	q.Cursor = ""
	bind := binding(q)
	c, err := decodeCursor(raw, bind)
	if err != nil {
		return page, err
	}
	field := "instant"
	if q.QualityView {
		field = "retrieved"
	}
	where, args := predicates(q.IdentityID, q.From, q.To, field)
	if q.QualityView {
		if q.Quality != "" {
			where += " AND quality=?"
			args = append(args, q.Quality)
		}
		if q.ProviderDate != "" {
			where += " AND provider_date=?"
			args = append(args, q.ProviderDate)
		}
	} else {
		where += " AND instant IS NOT NULL"
	}
	// One read transaction keeps omission counts and page coherent.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	if !q.QualityView {
		omit := "instant IS NULL"
		omitArgs := []any{}
		if q.IdentityID != 0 {
			omit += " AND identity_id=?"
			omitArgs = append(omitArgs, q.IdentityID)
		}
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM samples WHERE `+omit, omitArgs...).Scan(&page.OmittedUnresolved); err != nil {
			return
		}
	}
	if c != nil {
		where += " AND (" + field + ",id)>(?,?)"
		args = append(args, c.Time, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := tx.QueryContext(ctx, `SELECT id,identity_id,resolved_key,instant,provider_date,original_time,quality,values_json,retrieved,retrieval_id FROM samples WHERE `+where+` ORDER BY `+field+`,id LIMIT ?`, args...)
	if err != nil {
		return page, err
	}
	page.Records = []Sample{}
	for rows.Next() {
		var x Sample
		var original, values string
		if err = rows.Scan(&x.ID, &x.IdentityID, &x.ResolvedKey, &x.Instant, &x.ProviderDate, &original, &x.Quality, &values, &x.Retrieved, &x.RetrievalID); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(original), &x.OriginalTime); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(values), &x.Values); err != nil {
			break
		}
		page.Records = append(page.Records, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return
	}
	if len(page.Records) > q.Limit {
		page.Records = page.Records[:q.Limit]
		x := page.Records[q.Limit-1]
		t := x.Retrieved
		if !q.QualityView {
			t = *x.Instant
		}
		page.NextCursor = encodeCursor(bind, t, x.ID)
	}
	err = tx.Commit()
	return
}

const auditSelect = `SELECT a.id,a.job_id,a.identity_id,i.provider,i.stream,a.kind,a.start,a.end,a.provider_date,a.started,a.occurred,a.finished,a.outcome,a.error_class,a.error_code,a.message,a.page,a.retry_count,a.next_retry,a.retry_details,a.accepted_rows,i.metadata FROM attempts a JOIN identities i ON i.id=a.identity_id`

func scanAudit(row scanner) (x Audit, err error) {
	var retries, metadata string
	err = row.Scan(&x.ID, &x.JobID, &x.IdentityID, &x.Provider, &x.Stream, &x.Kind, &x.Start, &x.End, &x.ProviderDate, &x.Started, &x.Occurred, &x.Finished, &x.Outcome, &x.ErrorClass, &x.ErrorCode, &x.Message, &x.Page, &x.RetryCount, &x.NextRetry, &retries, &x.AcceptedRows, &metadata)
	if err == nil {
		err = json.Unmarshal([]byte(retries), &x.Retries)
	}
	if err == nil {
		err = json.Unmarshal([]byte(metadata), &x.Metadata)
	}
	x.DisplayIdentity = x.Metadata.DisplayIdentity(x.Provider, x.Stream, x.IdentityID)
	return
}
func evidenceRefs(ctx context.Context, tx *sql.Tx, id int64) ([]EvidenceRef, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.evidence_id,e.digest,r.retrieved,r.complete FROM retrievals r JOIN evidence e ON e.id=r.evidence_id WHERE r.attempt_id=? ORDER BY r.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EvidenceRef{}
	for rows.Next() {
		var x EvidenceRef
		if err = rows.Scan(&x.RetrievalID, &x.EvidenceID, &x.Digest, &x.Retrieved, &x.Complete); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) QueryAudit(ctx context.Context, q AuditQuery) (page AuditPage, err error) {
	if q.IdentityID < 0 || !rangeValid(q.From, q.To) || (q.Provider != "" && q.Provider != "octopus" && q.Provider != "growatt") {
		return page, ErrInvalid
	}
	if q.Outcome != "" && !validOutcome(q.Outcome) {
		return page, ErrInvalid
	}
	if q.ErrorClass != "" {
		if _, ok := errorMessages[q.ErrorClass]; !ok {
			return page, ErrInvalid
		}
	}
	q.Limit, err = pageLimit(q.Limit)
	if err != nil {
		return
	}
	raw := q.Cursor
	q.Cursor = ""
	bind := binding(q)
	c, err := decodeCursor(raw, bind)
	if err != nil {
		return page, err
	}
	where, args := predicates(q.IdentityID, q.From, q.To, "occurred")
	where = strings.ReplaceAll(where, "identity_id", "a.identity_id")
	if q.Provider != "" {
		where += " AND i.provider=?"
		args = append(args, q.Provider)
	}
	if q.Outcome != "" {
		where += " AND outcome=?"
		args = append(args, q.Outcome)
	}
	if q.ErrorClass != "" {
		where += " AND error_class=?"
		args = append(args, q.ErrorClass)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(outcome='failed'),0),coalesce(sum(outcome='rejected'),0) FROM attempts a JOIN identities i ON i.id=a.identity_id WHERE `+where, args...).Scan(&page.Counts.Total, &page.Counts.Failed, &page.Counts.Rejected)
	if err != nil {
		return
	}
	if c != nil {
		where += " AND (occurred,a.id)<(?,?)"
		args = append(args, c.Time, c.ID)
	}
	args = append(args, q.Limit+1)
	rows, err := tx.QueryContext(ctx, auditSelect+` WHERE `+where+` ORDER BY occurred DESC,a.id DESC LIMIT ?`, args...)
	if err != nil {
		return page, err
	}
	page.Records = []Audit{}
	for rows.Next() {
		x, e := scanAudit(rows)
		if e != nil {
			err = e
			break
		}
		page.Records = append(page.Records, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return
	}
	if len(page.Records) > q.Limit {
		page.Records = page.Records[:q.Limit]
		x := page.Records[q.Limit-1]
		page.NextCursor = encodeCursor(bind, x.Occurred, x.ID)
	}
	for i := range page.Records {
		page.Records[i].Evidence, err = evidenceRefs(ctx, tx, page.Records[i].ID)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

func validOutcome(s string) bool {
	switch s {
	case "running", "success", "empty", "rejected", "failed", "cancelled", "interrupted":
		return true
	}
	return false
}

// QueryAttempt returns sanitized audit context and evidence references, never a
// provider identity or telemetry blob. It supports a later authenticated detail API.
func (s *Store) QueryAttempt(ctx context.Context, id int64) (a Audit, err error) {
	if id <= 0 {
		return a, ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return a, err
	}
	defer tx.Rollback()
	a, err = scanAudit(tx.QueryRowContext(ctx, auditSelect+` WHERE a.id=?`, id))
	if err == sql.ErrNoRows {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	a.Evidence, err = evidenceRefs(ctx, tx, id)
	if err != nil {
		return a, err
	}
	err = tx.Commit()
	return
}
func (s *Store) QueryStatus(ctx context.Context) (out []Status, err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.provider,i.stream,i.selected,i.metadata,
  (SELECT count(*) FROM intervals WHERE identity_id=i.id),(SELECT count(*) FROM samples WHERE identity_id=i.id),
  (SELECT count(*) FROM samples WHERE identity_id=i.id AND instant IS NULL),
  (SELECT max(retrieved) FROM retrievals WHERE identity_id=i.id),
  (SELECT max(t) FROM (SELECT max(end) t FROM intervals WHERE identity_id=i.id UNION ALL SELECT max(instant) t FROM samples WHERE identity_id=i.id))
  FROM identities i ORDER BY i.id`)
	if err != nil {
		return nil, err
	}
	out = []Status{}
	for rows.Next() {
		var x Status
		var metadata string
		err = rows.Scan(&x.IdentityID, &x.Provider, &x.Stream, &x.Selected, &metadata, &x.IntervalCount, &x.SampleCount, &x.UnresolvedCount, &x.LastRetrieval, &x.NewestMeasurement)
		if err != nil {
			break
		}
		if err = json.Unmarshal([]byte(metadata), &x.Metadata); err != nil {
			break
		}
		out = append(out, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return
	}
	for n := range out {
		x := &out[n]
		a, e := scanAudit(tx.QueryRowContext(ctx, auditSelect+` WHERE a.identity_id=? ORDER BY occurred DESC,a.id DESC LIMIT 1`, x.IdentityID))
		if e != nil && e != sql.ErrNoRows {
			return nil, e
		}
		if e == nil {
			x.LastAttempt = &a
		}
		accepted, e := scanAudit(tx.QueryRowContext(ctx, auditSelect+` WHERE a.identity_id=? AND outcome IN ('success','empty') ORDER BY occurred DESC,a.id DESC LIMIT 1`, x.IdentityID))
		if e != nil && e != sql.ErrNoRows {
			return nil, e
		}
		if e == nil {
			x.LastAcceptedAttempt = &accepted
		}
		if err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM jobs WHERE identity_id=?),(SELECT count(*) FROM completions c JOIN jobs j ON j.id=c.job_id WHERE j.identity_id=?)`, x.IdentityID, x.IdentityID).Scan(&x.JobCount, &x.WindowCount); err != nil {
			return
		}
		rows, e = tx.QueryContext(ctx, jobSelect+` WHERE identity_id=? ORDER BY id DESC LIMIT 500`, x.IdentityID)
		if e != nil {
			return nil, e
		}
		x.Jobs = []Job{}
		for rows.Next() {
			j, e := scanJob(rows)
			if e != nil {
				err = e
				break
			}
			x.Jobs = append(x.Jobs, j)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return
		}
		rows, e = tx.QueryContext(ctx, `SELECT c.job_id,c.attempt_id,c.start,c.end,c.fetched,c.expected,c.valid,c.missing,c.qualified,c.measurement_complete FROM completions c JOIN jobs j ON j.id=c.job_id WHERE j.identity_id=? ORDER BY c.start DESC,c.job_id DESC LIMIT 500`, x.IdentityID)
		if e != nil {
			return nil, e
		}
		x.Windows = []WindowStatus{}
		for rows.Next() {
			var w WindowStatus
			err = rows.Scan(&w.JobID, &w.AttemptID, &w.Start, &w.End, &w.Fetched, &w.Coverage.Expected, &w.Coverage.Valid, &w.Coverage.Missing, &w.Coverage.Qualified, &w.Coverage.MeasurementComplete)
			if err != nil {
				break
			}
			x.Windows = append(x.Windows, w)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}
