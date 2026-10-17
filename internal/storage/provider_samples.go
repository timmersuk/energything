package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// This is lexical ordering of a reported wall clock, never an absolute instant.
// Canonical source periods are unique. Internal rowids are paging tie-breakers
// across devices, not reading identities.
type providerCursor struct {
	Binding, Date, Clock string
	ID                   int64
}

func providerDateValid(value string) bool {
	t, e := time.Parse("2006-01-02", value)
	return e == nil && t.Year() >= 1
}

func (s *Store) queryProviderSamples(ctx context.Context, q SampleQuery) (page SamplePage, err error) {
	if q.IdentityID < 0 || q.Direction != "" || q.QualityView || q.From != nil || q.To != nil || !providerDateValid(q.ProviderFrom) || !providerDateValid(q.ProviderTo) || q.ProviderFrom > q.ProviderTo || !validDate(q.ProviderDate) || (q.Quality != "" && !validText(q.Quality)) {
		return page, ErrInvalid
	}
	q.Limit, err = pageLimit(q.Limit)
	if err != nil {
		return
	}
	raw := q.Cursor
	q.Cursor = ""
	bind := binding(q)
	var c *providerCursor
	if raw != "" {
		if len(raw) > 1024 {
			return page, ErrInvalid
		}
		b, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil {
			return page, ErrInvalid
		}
		c = &providerCursor{}
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.DisallowUnknownFields()
		if !json.Valid(b) || d.Decode(c) != nil || c.Binding != bind || c.ID <= 0 || !providerDateValid(c.Date) || len(c.Clock) > 19 {
			return page, ErrInvalid
		}
		if c.Clock != "" {
			if _, e := time.Parse("2006-01-02 15:04:05", c.Clock); e != nil || !strings.HasPrefix(c.Clock, c.Date+" ") {
				return page, ErrInvalid
			}
		}
	}
	where, args := predicates(q.IdentityID, nil, nil, "")
	where += " AND provider_date>=? AND provider_date<=?"
	args = append(args, q.ProviderFrom, q.ProviderTo)
	if q.ProviderDate != "" {
		where += " AND provider_date=?"
		args = append(args, q.ProviderDate)
	}
	if q.Quality != "" {
		where += " AND quality=?"
		args = append(args, q.Quality)
	} else {
		// Enumerate every retained label so the existing (quality,provider_date)
		// index can serve date ranges without scanning historical payload pages.
		where += " AND quality IN (SELECT DISTINCT quality FROM samples)"
	}
	if c != nil {
		where += " AND provider_date<=?"
		args = append(args, c.Date)
	}
	// Select dates cheaply first: a wide range must not decode every historical
	// timestamp document just to return the first page of recent readings.
	tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return page, e
	}
	defer tx.Rollback()
	dateArgs := append(append([]any{}, args...), q.Limit+2)
	dateRows, e := tx.QueryContext(ctx, `SELECT DISTINCT provider_date FROM samples WHERE `+where+` ORDER BY provider_date DESC LIMIT ?`, dateArgs...)
	if e != nil {
		return page, e
	}
	days := []string{}
	for dateRows.Next() {
		var date string
		if e = dateRows.Scan(&date); e != nil {
			dateRows.Close()
			return page, e
		}
		days = append(days, date)
	}
	e = dateRows.Err()
	dateRows.Close()
	if e != nil {
		return page, e
	}
	keys := []providerCursor{}
	for _, date := range days {
		rowArgs := append(append([]any{}, args...), date)
		rows, e := tx.QueryContext(ctx, `SELECT rowid,original_time FROM samples WHERE `+where+` AND provider_date=?`, rowArgs...)
		if e != nil {
			return page, e
		}
		dayKeys := []providerCursor{}
		for rows.Next() {
			var id int64
			var original string
			if e = rows.Scan(&id, &original); e != nil {
				break
			}
			var fields struct {
				Time string `json:"time"`
			}
			if e = json.Unmarshal([]byte(original), &fields); e != nil {
				break
			}
			clock := ""
			_ = json.Unmarshal([]byte(fields.Time), &clock)
			if _, parseErr := time.Parse("2006-01-02 15:04:05", clock); parseErr != nil || len(clock) != 19 || !strings.HasPrefix(clock, date+" ") {
				clock = ""
			}
			if c != nil && date == c.Date && (clock > c.Clock || (clock == c.Clock && id >= c.ID)) {
				continue
			}
			dayKeys = append(dayKeys, providerCursor{bind, date, clock, id})
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return page, e
		}
		sort.Slice(dayKeys, func(i, j int) bool {
			if dayKeys[i].Clock != dayKeys[j].Clock {
				return dayKeys[i].Clock > dayKeys[j].Clock
			}
			return dayKeys[i].ID > dayKeys[j].ID
		})
		remaining := q.Limit + 1 - len(keys)
		if len(dayKeys) > remaining {
			dayKeys = dayKeys[:remaining]
		}
		keys = append(keys, dayKeys...)
		if len(keys) > q.Limit {
			break
		}
	}
	page.Records = []Sample{}
	if len(keys) > 0 {
		placeholders := make([]string, len(keys))
		ids := make([]any, len(keys))
		for i, key := range keys {
			placeholders[i] = "?"
			ids[i] = key.ID
		}
		rows, e := tx.QueryContext(ctx, `SELECT rowid,identity_id,instant,provider_date,original_time,quality,values_json,retrieved,retrieval_id FROM samples WHERE rowid IN (`+strings.Join(placeholders, ",")+`)`, ids...)
		if e != nil {
			return page, e
		}
		records := map[int64]Sample{}
		for rows.Next() {
			var x Sample
			var original, values string
			if e = rows.Scan(&x.ID, &x.IdentityID, &x.Instant, &x.ProviderDate, &original, &x.Quality, &values, &x.Retrieved, &x.RetrievalID); e != nil {
				break
			}
			if e = json.Unmarshal([]byte(original), &x.OriginalTime); e != nil {
				break
			}
			if e = json.Unmarshal([]byte(values), &x.Values); e != nil {
				break
			}
			records[x.ID] = x
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return page, e
		}
		for _, key := range keys {
			x, ok := records[key.ID]
			if !ok {
				return page, ErrNotFound
			}
			page.Records = append(page.Records, x)
		}
	}
	if len(page.Records) > q.Limit {
		page.Records = page.Records[:q.Limit]
		b, _ := json.Marshal(keys[q.Limit-1])
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	err = tx.Commit()
	return
}
