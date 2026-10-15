package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

const AggregateCalendar = "provider_date_zone_unverified"

type Aggregate struct {
	IdentityID  int64   `json:"identity_id"`
	PlantID     string  `json:"plant_id"`
	Metric      string  `json:"metric"`
	Resolution  string  `json:"resolution"`
	Period      string  `json:"period"`
	Value       string  `json:"value"`
	Unit        string  `json:"unit"`
	Calendar    string  `json:"calendar"`
	Quality     string  `json:"quality"`
	Retrieved   Instant `json:"-"`
	RetrievalID int64   `json:"retrieval_id"`
	Evidence    int     `json:"-"`
}

func AggregateQuality(value string) string {
	if sameDecimal(value, "0") {
		return "reported_zero_availability_unknown"
	}
	return "provider_reported"
}
func validateAggregates(a Audit, in AcceptedWindow) ([]Aggregate, error) {
	out := []Aggregate{}
	seen := map[string]Aggregate{}
	for _, x := range in.Aggregates {
		if a.Kind != "aggregate" || (x.IdentityID != 0 && x.IdentityID != a.IdentityID) || x.Period == "" || !validDate(x.Period) || x.Period != a.ProviderDate || x.Metric != "generation" || x.Resolution != "day" || x.Unit != "kWh" || x.Calendar != AggregateCalendar || len(x.Value) > 128 || !decimal.MatchString(x.Value) || x.Quality != AggregateQuality(x.Value) || x.Evidence < 0 || x.Evidence >= len(in.Evidence) {
			return nil, ErrInvalid
		}
		if old, ok := seen[x.Period]; ok {
			if !sameDecimal(old.Value, x.Value) {
				return nil, ErrInvalid
			}
			continue
		}
		seen[x.Period] = x
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	return out, nil
}

// ReserveAggregateCall conservatively enforces 5m spacing and ten reservations
// per rolling 24h across restarts. Failed transmissions still consume a reservation.
// Other software's calls cannot be observed by this local ledger.
func (s *Store) ReserveAggregateCall(ctx context.Context, now Instant) (due time.Time, err error) {
	if !validInstant(now) {
		return due, ErrInvalid
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT reserved FROM aggregate_calls WHERE reserved>? ORDER BY reserved`, now-Instant(24*time.Hour/time.Microsecond))
		if e != nil {
			return e
		}
		var calls []Instant
		for rows.Next() {
			var x Instant
			if e = rows.Scan(&x); e != nil {
				rows.Close()
				return e
			}
			calls = append(calls, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		var next Instant
		if len(calls) > 0 {
			next = calls[len(calls)-1] + Instant(5*time.Minute/time.Microsecond)
		}
		if len(calls) >= 10 {
			quota := calls[len(calls)-10] + Instant(24*time.Hour/time.Microsecond)
			if quota > next {
				next = quota
			}
		}
		if next > now {
			due = time.UnixMicro(int64(next))
			return nil
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO aggregate_calls(reserved) VALUES(?)`, now)
		return e
	})
	return
}

type AggregateQuery struct {
	PlantID, Metric, Resolution, From, To string
	Limit                                 int
	Cursor                                string
}
type AggregatePage struct {
	Records    []Aggregate
	NextCursor string
}

func (s *Store) QueryAggregates(ctx context.Context, q AggregateQuery) (AggregatePage, error) {
	out := AggregatePage{Records: []Aggregate{}}
	limit, e := pageLimit(q.Limit)
	if e != nil {
		return out, e
	}
	q.Limit = limit
	if (q.Metric != "" && q.Metric != "generation") || (q.Resolution != "" && q.Resolution != "day") || !validDate(q.From) || !validDate(q.To) || (q.From != "" && q.To != "" && q.From > q.To) || len(q.PlantID) > 100 {
		return out, ErrInvalid
	}
	rawCursor := q.Cursor
	q.Cursor = ""
	bind := binding(q)
	cur, e := decodeCursor(rawCursor, bind)
	if e != nil {
		return out, e
	}
	where := "i.provider='growatt' AND i.stream='generation'"
	args := []any{}
	if q.PlantID != "" {
		where += " AND json_extract(i.metadata,'$.plant_id')=?"
		args = append(args, q.PlantID)
	}
	if q.From != "" {
		where += " AND a.period>=?"
		args = append(args, q.From)
	}
	if q.To != "" {
		where += " AND a.period<=?"
		args = append(args, q.To)
	}
	if cur != nil {
		date := time.UnixMicro(int64(cur.Time)).UTC().Format("2006-01-02")
		where += " AND (a.period>? OR (a.period=? AND a.identity_id>?))"
		args = append(args, date, date, cur.ID)
	}
	args = append(args, limit+1)
	rows, e := s.db.QueryContext(ctx, `SELECT a.identity_id,json_extract(i.metadata,'$.plant_id'),a.metric,a.resolution,a.period,a.value,a.unit,a.calendar,a.quality,a.retrieved,a.retrieval_id FROM energy_aggregates a JOIN identities i ON i.id=a.identity_id WHERE `+where+` ORDER BY a.period,a.identity_id LIMIT ?`, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x Aggregate
		if e = rows.Scan(&x.IdentityID, &x.PlantID, &x.Metric, &x.Resolution, &x.Period, &x.Value, &x.Unit, &x.Calendar, &x.Quality, &x.Retrieved, &x.RetrievalID); e != nil {
			return out, e
		}
		out.Records = append(out.Records, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Records) > limit {
		out.Records = out.Records[:limit]
		last := out.Records[limit-1]
		date, _ := time.Parse("2006-01-02", last.Period)
		out.NextCursor = encodeCursor(bind, At(date), last.IdentityID)
	}
	return out, nil
}

// ValidateAggregateJob binds every request selector to persisted work, including
// units whose provider response contains no rows to validate afterward.
func (s *Store) ValidateAggregateJob(ctx context.Context, job Job, plant string) error {
	var actual Job
	var provider, stream, key, metadata string
	err := s.db.QueryRowContext(ctx, `SELECT j.identity_id,j.kind,j.start,j.end,j.provider_date,i.provider,i.stream,i.stable_key,i.metadata FROM jobs j JOIN identities i ON i.id=j.identity_id WHERE j.id=?`, job.ID).Scan(&actual.IdentityID, &actual.Kind, &actual.Start, &actual.End, &actual.ProviderDate, &provider, &stream, &key, &metadata)
	if err != nil {
		return err
	}
	var m DiscoveryMetadata
	if json.Unmarshal([]byte(metadata), &m) != nil || provider != "growatt" || stream != "generation" || plant == "" || plant != key || plant != m.PlantID || m.Serial != "" || actual.Kind != "aggregate" || job.Kind != actual.Kind || job.IdentityID != actual.IdentityID || job.Start != actual.Start || job.End != actual.End || job.ProviderDate != actual.ProviderDate {
		return ErrInvalid
	}
	return nil
}
