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

type growattTransport func(*http.Request) (*http.Response, error)

func (f growattTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGrowattAcceptedRejectedEmptyReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := db.RecordGrowattDiscovery(ctx, storage.DiscoveryState{Provider: "growatt", Stream: "telemetry", State: "selected", Occurred: storage.At(time.Now())}, []storage.Identity{{Provider: "growatt", Stream: "telemetry", StableKey: `["123","SYN-A"]`, Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "123", Serial: "SYN-A", DeviceType: "5"}}})
	if e != nil {
		t.Fatal(e)
	}
	from := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(from.Add(24 * time.Hour)), ProviderDate: "2026-03-29", NextDue: storage.At(time.Now())}})
	if e != nil {
		t.Fatal(e)
	}
	body := `{"error_code":0,"data":{"count":2,"datas":[{"time":"2026-03-29 01:30:00","soc":0,"bmsSOC":8,"unknown":99},{"time":"2026-03-29 01:31:00","soc":2}]}}`
	c := growatt.NewClient("synthetic", growattTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	d := growatt.Device{PlantID: "123", Serial: "SYN-A", Family: "5"}
	if e = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); e != nil {
		t.Fatal(e)
	}
	status, _ := db.QueryStatus(ctx)
	checkpoint := *status[0].Jobs[0].Checkpoint
	body = `{"error_code":0,"data":{"count":2,"datas":[{"time":"2026-03-29 01:30:00","soc":10},{"time":"2026-03-29 01:31:00","soc":101}]}}`
	if e = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); e == nil {
		t.Fatal("invalid day accepted")
	}
	status, _ = db.QueryStatus(ctx)
	if status[0].SampleCount != 2 || *status[0].Jobs[0].Checkpoint != checkpoint || status[0].LastAttempt.Outcome != "rejected" || status[0].LastAttempt.ErrorCode != "soc_range" {
		t.Fatal(status)
	}
	body = `{"error_code":0,"data":{"count":2,"datas":[{"time":"2026-03-29 01:30:00","soc":10},{"time":"2026-03-29 01:30:00","soc":20}]}}`
	if e = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); e == nil {
		t.Fatal("conflicting source period accepted")
	}
	status, _ = db.QueryStatus(ctx)
	if status[0].SampleCount != 2 || status[0].LastAttempt.Outcome != "rejected" || status[0].LastAttempt.ErrorCode != "conflict" || status[0].LastAttempt.NextRetry == nil || *status[0].LastAttempt.NextRetry <= *status[0].LastAttempt.Finished {
		t.Fatal("conflict changed accepted rows or lost backoff", status)
	}
	body = `{"error_code":0,"data":{"count":0,"datas":[]}}`
	if e = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	status, _ = db.QueryStatus(ctx)
	if status[0].SampleCount != 2 || status[0].LastAttempt.Outcome != "empty" || status[0].UnresolvedCount != 2 {
		t.Fatal(status)
	}
	audits, e := db.QueryAudit(ctx, storage.AuditQuery{})
	if e != nil || len(audits.Records) != 4 || len(audits.Records[1].Evidence) != 1 {
		t.Fatal(audits, e)
	}
}

func TestGrowattOverrunAndDiscoveryFailureAudit(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ids, e := db.RecordGrowattDiscovery(ctx, storage.DiscoveryState{Provider: "growatt", Stream: "telemetry", State: "selected", Occurred: storage.At(time.Now())}, []storage.Identity{{Provider: "growatt", Stream: "telemetry", StableKey: "synthetic", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "123", Serial: "SYN-A"}}})
	if e != nil {
		t.Fatal(e)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", ProviderDate: "2026-01-01", Start: storage.At(from), End: storage.At(from.Add(24 * time.Hour)), NextDue: storage.At(time.Now())}})
	if e != nil {
		t.Fatal(e)
	}
	body := `{"error_code":0,"data":{"count":10000,"datas":[` + strings.Repeat(`{"soc":0},`, 10000) + `{"soc":0}]}}`
	c := growatt.NewClient("synthetic", growattTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	if e = CollectGrowattUnit(ctx, db, c, jobs[0], growatt.Device{PlantID: "123", Serial: "SYN-A", Family: "5"}, "Europe/London"); e == nil {
		t.Fatal("oversized unit accepted")
	}
	status, _ := db.QueryStatus(ctx)
	a := status[0].LastAttempt
	if a.Outcome != "failed" || a.Diagnostic.Observed == nil || *a.Diagnostic.Observed != 10001 || status[0].Jobs[0].RunningAttempt != nil || status[0].Jobs[0].Checkpoint != nil {
		t.Fatal("overrun audit lost", status)
	}
	c = growatt.NewClient("synthetic", growattTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private-secret"))}, nil
	}))
	if _, _, e = DiscoverGrowatt(ctx, db, c, "", ""); e == nil {
		t.Fatal("failed discovery accepted")
	}
	states, e := db.QueryDiscovery(ctx)
	if e != nil || len(states) != 1 || states[0].Failure == nil || states[0].Failure.Diagnostic.HTTPStatus != 404 || strings.Contains(states[0].Failure.Message, "secret") {
		t.Fatal("discovery context unavailable", states, e)
	}
}
