package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type aggregateTransport func(*http.Request) (*http.Response, error)

func (f aggregateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAggregateCollectionAtomicDiagnosticAndQuota(t *testing.T) {
	for _, mode := range []string{"accepted", "invalid", "auth", "cancelled", "wrongplant", "wrongplantempty", "wrongdateempty", "wrongendempty"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s, e := storage.Open(ctx, t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-PLANT", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-PLANT"}}})
			if e != nil {
				t.Fatal(e)
			}
			day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
			jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(day), End: storage.At(day.AddDate(0, 0, 1)), ProviderDate: "2026-01-15", NextDue: storage.At(time.Now())}})
			if e != nil {
				t.Fatal(e)
			}
			calls := 0
			client := growatt.NewClient("synthetic", aggregateTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if strings.HasSuffix(mode, "empty") {
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error_code":0,"data":{"count":0,"time_unit":"day","energys":[]}}`))}, nil
				}
				value := `"1.234567890123456789"`
				if mode == "invalid" {
					value = `null`
				}
				status := 200
				if mode == "auth" {
					status = 403
				}
				if mode == "cancelled" {
					return nil, context.Canceled
				}
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error_code":0,"data":{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":` + value + `}]}}`))}, nil
			}))
			plant := "SYN-PLANT"
			if mode == "wrongplant" || mode == "wrongplantempty" {
				plant = "SYN-OTHER"
			}
			if mode == "wrongdateempty" {
				jobs[0].ProviderDate = "2026-01-16"
			}
			if mode == "wrongendempty" {
				jobs[0].End += storage.Instant(24 * time.Hour / time.Microsecond)
			}
			e = CollectGrowattAggregateUnit(ctx, s, client, jobs[0], plant)
			if (e == nil) != (mode == "accepted") {
				t.Fatal(e)
			}
			a, e := s.QueryAudit(ctx, storage.AuditQuery{})
			if e != nil || len(a.Records) != 1 {
				t.Fatal(a, e)
			}
			p, e := s.QueryAggregates(ctx, storage.AggregateQuery{})
			if e != nil {
				t.Fatal(e)
			}
			if mode == "accepted" {
				if len(p.Records) != 1 || p.Records[0].Value != "1.234567890123456789" {
					t.Fatal(p)
				}
				if e = CollectGrowattAggregateUnit(ctx, s, client, jobs[0], plant); e == nil || calls != 1 {
					t.Fatal("quota failed", e, calls)
				}
				again, e := s.QueryAggregates(ctx, storage.AggregateQuery{})
				if e != nil || again.Records[0].RetrievalID != p.Records[0].RetrievalID {
					t.Fatal("failure changed acceptance")
				}
			} else {
				if len(p.Records) != 0 {
					t.Fatal("partial acceptance")
				}
				if mode == "invalid" && (a.Records[0].Outcome != "rejected" || a.Records[0].ErrorCode != "aggregate_decimal" || len(a.Records[0].Evidence) != 1 || !strings.Contains(a.Records[0].Message, "placeholder")) {
					t.Fatal(a.Records[0])
				}
			}
			expectedCalls := 1
			if strings.HasPrefix(mode, "wrong") {
				expectedCalls = 0
				if a.Records[0].ErrorCode != "aggregate_request" || a.Records[0].Outcome != "failed" {
					t.Fatal(a.Records[0])
				}
				status, err := s.QueryStatus(ctx)
				if err != nil || status[0].Jobs[0].Checkpoint != nil || status[0].WindowCount != 0 {
					t.Fatal("wrong selectors advanced progress", err)
				}
			}
			if calls != expectedCalls {
				t.Fatal("inline retry", calls)
			}
		})
	}
}

func TestSevenDateAggregateCollection(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			ctx := context.Background()
			s, e := storage.Open(ctx, t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-PLANT", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-PLANT"}}})
			if e != nil {
				t.Fatal(e)
			}
			start, end, e := storage.AggregateDateBounds("2026-01-12", "2026-01-18")
			if e != nil {
				t.Fatal(e)
			}
			jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(start), End: storage.At(end), ProviderDate: "2026-01-12", NextDue: storage.At(time.Now())}})
			if e != nil {
				t.Fatal(e)
			}
			calls := 0
			client := growatt.NewClient("synthetic", aggregateTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				q := r.URL.Query()
				if q.Get("start_date") != "2026-01-12" || q.Get("end_date") != "2026-01-18" {
					t.Fatal("window narrowed", q)
				}
				rows := []map[string]any{}
				for n := 0; n < 7; n++ {
					var value any = "1.234567890123456789"
					if invalid && n == 6 {
						value = nil
					}
					rows = append(rows, map[string]any{"date": start.AddDate(0, 0, n).Format("2006-01-02"), "energy": value})
				}
				b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": 7, "time_unit": "day", "energys": rows}})
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
			}))
			e = CollectGrowattAggregateUnit(ctx, s, client, jobs[0], "SYN-PLANT")
			if (e != nil) != invalid || calls != 1 {
				t.Fatal(e, calls)
			}
			page, e := s.QueryAggregates(ctx, storage.AggregateQuery{})
			if e != nil {
				t.Fatal(e)
			}
			status, e := s.QueryStatus(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if invalid {
				if len(page.Records) != 0 || status[0].Jobs[0].Checkpoint != nil || status[0].LastAttempt.Outcome != "rejected" {
					t.Fatal("partial acceptance")
				}
			} else {
				if len(page.Records) != 7 || page.Records[0].Period != "2026-01-12" || page.Records[6].Period != "2026-01-18" || status[0].Jobs[0].Checkpoint == nil || *status[0].Jobs[0].Checkpoint != storage.At(end) {
					t.Fatal(page, status)
				}
			}
		})
	}
}

func TestAggregateSchedulerBudgetOverrideUsesSharedReservations(t *testing.T) {
	ctx := context.Background()
	s, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-EXPERIMENT", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-EXPERIMENT"}}})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	for n := 0; n < 10; n++ {
		due, e := s.ReserveAggregateCall(ctx, storage.At(now.Add(-time.Hour+time.Duration(n)*5*time.Minute)))
		if e != nil || !due.IsZero() {
			t.Fatal(n, due, e)
		}
	}
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(from), End: storage.At(from.AddDate(0, 0, 1)), ProviderDate: "2026-01-15", NextDue: storage.At(now)}})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	client := growatt.NewClient("synthetic", aggregateTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error_code":0,"data":{"count":0,"time_unit":"day","energys":[]}}`))}, nil
	}))
	if e = CollectGrowattAggregateUnit(ctx, aggregateBudgetStore{s, false}, client, jobs[0], "SYN-EXPERIMENT"); e != nil || calls != 1 {
		t.Fatal("11th transmission not allowed", e, calls)
	}
	if e = CollectGrowattAggregateUnit(ctx, aggregateBudgetStore{s, false}, client, jobs[0], "SYN-EXPERIMENT"); e == nil || calls != 1 {
		t.Fatal("spacing bypass", e, calls)
	}
	if e = CollectGrowattAggregateUnit(ctx, aggregateBudgetStore{s, true}, client, jobs[0], "SYN-EXPERIMENT"); e == nil || calls != 1 {
		t.Fatal("restored cap bypass", e, calls)
	}
}
