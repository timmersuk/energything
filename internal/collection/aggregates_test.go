package collection

import (
	"context"
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
	for _, mode := range []string{"accepted", "invalid", "auth", "cancelled", "wrongplant"} {
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
			if mode == "wrongplant" {
				plant = "SYN-OTHER"
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
			if calls != 1 {
				t.Fatal("inline retry", calls)
			}
		})
	}
}
