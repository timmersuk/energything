package collection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOneShotCorrectionFailureAndRetainedCoverage(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	mode := "valid"
	calls := 0
	c := octopus.NewClient("synthetic-key", transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"properties":[{"electricity_meter_points":[{"mpan":"synthetic-import","is_export":false,"meters":[{"serial_number":"i"}]},{"mpan":"synthetic-export","is_export":true,"meters":[{"serial_number":"e"}]}]}]}`
		status := 200
		if strings.Contains(r.URL.Path, "consumption") {
			value := "0.100000000000000001"
			if mode == "correction" {
				value = "0.120"
			}
			if mode == "invalid" {
				value = "-1"
			}
			body = fmt.Sprintf(`{"count":1,"next":null,"results":[{"consumption":%s,"interval_start":"2026-01-01T00:00:00Z","interval_end":"2026-01-01T00:30:00Z","unknown_telemetry":"retained"}]}`, value)
			if mode == "empty" {
				body = `{"count":0,"next":null,"results":[]}`
			}
			if mode == "auth" {
				status = 401
				body = "PRIVATE MUST NOT PERSIST"
			}
			if mode == "second-page" {
				if r.URL.Query().Get("page") == "2" {
					status = 401
					body = "private"
				} else {
					u := *r.URL
					q := u.Query()
					q.Set("page", "2")
					u.RawQuery = q.Encode()
					body = strings.Replace(body, `"next":null`, `"next":"`+u.String()+`"`, 1)
					body = strings.Replace(body, `"count":1`, `"count":2`, 1)
				}
			}
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	p := config.Provider{Account: "synthetic-account"}
	run := func(expectErr bool) {
		t.Helper()
		err := OctopusOnce(ctx, db, c, p, from, to)
		if (err != nil) != expectErr {
			t.Fatal(mode, err)
		}
	}
	run(false)
	status, err := db.QueryStatus(ctx)
	if err != nil || len(status) != 2 || status[0].IntervalCount != 1 || status[1].IntervalCount != 1 || status[0].Windows[0].Coverage.Missing != 1 {
		t.Fatal(status, err)
	}
	mode = "invalid"
	run(true)
	records, _ := db.QueryIntervals(ctx, storage.RecordQuery{From: instantPtr(from), To: instantPtr(to)})
	if len(records.Records) != 2 || records.Records[0].KWh != "0.100000000000000001" {
		t.Fatal("rejected window changed rows", records)
	}
	status, _ = db.QueryStatus(ctx)
	if status[0].LastAttempt.Outcome != "rejected" || status[0].LastAcceptedAttempt.Outcome != "success" {
		t.Fatal(status)
	}
	audit, err := db.QueryAttempt(ctx, status[0].LastAttempt.ID)
	if err != nil || len(audit.Evidence) != 1 {
		t.Fatal("rejected evidence", audit, err)
	}
	mode = "correction"
	run(false)
	records, _ = db.QueryIntervals(ctx, storage.RecordQuery{From: instantPtr(from), To: instantPtr(to)})
	if records.Records[0].KWh != "0.120" {
		t.Fatal("correction", records)
	}
	mode = "empty"
	run(false)
	status, _ = db.QueryStatus(ctx)
	if status[0].IntervalCount != 1 || status[0].Windows[0].Coverage.Valid != 0 || status[0].Windows[0].Coverage.Missing != 2 || status[0].LastAttempt.Outcome != "empty" {
		t.Fatal("retained vs latest", status)
	}
	mode = "second-page"
	run(true)
	status, _ = db.QueryStatus(ctx)
	audit, _ = db.QueryAttempt(ctx, status[0].LastAttempt.ID)
	if audit.Outcome != "failed" || len(audit.Evidence) != 1 || audit.Evidence[0].Complete || status[0].LastAcceptedAttempt.Outcome != "empty" {
		t.Fatal("partial pages committed", audit, status)
	}
	mode = "auth"
	run(true)
	status, _ = db.QueryStatus(ctx)
	if !status[0].Jobs[0].Paused {
		t.Fatal("auth not paused")
	}
	states, _ := db.QueryDiscovery(ctx)
	raw, _ := json.Marshal(states)
	if len(states) != 2 || strings.Contains(string(raw), "synthetic-import") {
		t.Fatal("discovery leak", states)
	}
	if calls != 20 {
		t.Fatal("unexpected calls", calls)
	}
}

// Consumer-owned wrapper injects failures at the typed write seam, never SQL.
type failingStore struct {
	*storage.Store
	begin, commit, audit bool
}

func (s failingStore) BeginAttempt(ctx context.Context, r storage.AttemptRequest) (int64, error) {
	if s.begin {
		return 0, errors.New("PRIVATE")
	}
	return s.Store.BeginAttempt(ctx, r)
}
func (s failingStore) CommitAcceptedWindow(ctx context.Context, r storage.AcceptedWindow) error {
	if s.commit {
		return errors.New("PRIVATE")
	}
	return s.Store.CommitAcceptedWindow(ctx, r)
}
func (s failingStore) RecordRejectedOrFailedAttempt(ctx context.Context, r storage.FailedAttempt) error {
	if s.audit {
		return errors.New("PRIVATE")
	}
	return s.Store.RecordRejectedOrFailedAttempt(ctx, r)
}
func TestStorageFailureAndCancellation(t *testing.T) {
	for _, which := range []string{"begin", "commit", "audit", "cancel"} {
		t.Run(which, func(t *testing.T) {
			ctx := context.Background()
			db, err := storage.Open(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ids, err := db.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "synthetic", Selected: true}})
			if err != nil {
				t.Fatal(err)
			}
			from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			jobs, err := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(from.Add(time.Hour)), NextDue: storage.At(time.Now())}})
			if err != nil {
				t.Fatal(err)
			}
			requestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			calls := 0
			c := octopus.NewClient("synthetic", transport(func(r *http.Request) (*http.Response, error) {
				calls++
				if which == "cancel" {
					cancel()
					return nil, r.Context().Err()
				}
				status := 200
				body := `{"count":0,"next":null,"results":[]}`
				if which == "audit" {
					status = 401
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			}))
			store := failingStore{db, which == "begin", which == "commit", which == "audit"}
			err = CollectOctopusWindow(requestCtx, store, c, jobs[0], octopus.Meter{MPAN: "synthetic", Serial: "meter", Direction: "import"})
			if err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal(err)
			}
			states, _ := db.QueryStatus(ctx)
			if states[0].IntervalCount != 0 || states[0].Jobs[0].Checkpoint != nil {
				t.Fatal("false completion", states)
			}
			if which == "begin" {
				if calls != 0 || states[0].LastAttempt != nil {
					t.Fatal("fetch after begin failure")
				}
			} else if which == "audit" {
				if states[0].LastAttempt.Outcome != "running" {
					t.Fatal("false audit")
				}
			} else {
				if states[0].LastAttempt.Outcome == "running" {
					t.Fatal("failure not durable")
				}
			}
		})
	}
}
func instantPtr(t time.Time) *storage.Instant { v := storage.At(t); return &v }
func TestNullCountCannotAdvanceCheckpoint(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := octopus.NewClient("synthetic", transport(func(r *http.Request) (*http.Response, error) {
		body := `{"properties":[{"electricity_meter_points":[{"mpan":"synthetic","is_export":false,"meters":[{"serial_number":"synthetic"}]}]}]}`
		if strings.Contains(r.URL.Path, "consumption") {
			body = `{"count":null,"results":[],"next":null}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	err = OctopusOnce(ctx, db, c, config.Provider{Account: "synthetic"}, from, from.Add(time.Hour))
	if err == nil {
		t.Fatal("null count accepted")
	}
	states, _ := db.QueryStatus(ctx)
	if len(states) != 1 || states[0].Jobs[0].Checkpoint != nil || states[0].LastAttempt.Outcome != "failed" || states[0].WindowCount != 0 {
		t.Fatal(states)
	}
}
