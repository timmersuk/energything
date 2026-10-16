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

func TestGrowingGrowattDayPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	ids, err := db.RecordGrowattDiscovery(ctx, storage.DiscoveryState{Provider: "growatt", Stream: "telemetry", State: "selected", Occurred: storage.At(time.Now())}, []storage.Identity{{Provider: "growatt", Stream: "telemetry", StableKey: "synthetic-growth", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "synthetic", Serial: "SYN-GROW", DeviceType: "5"}}})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	jobs, err := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "routine", Start: storage.At(from), End: storage.At(from.Add(24 * time.Hour)), ProviderDate: "2026-10-16", NextDue: storage.At(time.Now())}})
	if err != nil {
		t.Fatal(err)
	}
	shrink := false
	soc := 50
	c := growatt.NewClient("synthetic", growattTransport(func(r *http.Request) (*http.Response, error) {
		r.ParseForm()
		page := r.Form.Get("page")
		count, size, offset := 101, 100, 0
		if page == "2" {
			count, size, offset = 102, 2, 100
			if shrink {
				count, size = 100, 0
			}
		}
		rows := make([]map[string]any, size)
		for i := range rows {
			rows[i] = map[string]any{"time": fmt.Sprintf("2026-10-16 %02d:%02d:00", (offset+i)/60, (offset+i)%60), "soc": soc}
		}
		b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": count, "datas": rows}})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	}))
	d := growatt.Device{PlantID: "synthetic", Serial: "SYN-GROW", Family: "5"}
	if err = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); err != nil {
		t.Fatal("growing day discarded", err)
	}
	// Replay and correction must both use the periods in the real provider
	// response, without caller-supplied synthetic reading keys.
	for _, value := range []int{50, 51} {
		soc = value
		if err = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); err != nil {
			t.Fatal(err)
		}
	}
	shrink = true
	if err = CollectGrowattUnit(ctx, db, c, jobs[0], d, "Europe/London"); err == nil {
		t.Fatal("shrinking day accepted")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	status, err := db.QueryStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status[0].SampleCount != 102 || status[0].LastAcceptedAttempt.Outcome != "success" || status[0].LastAttempt.ErrorCode != "count_changed" {
		t.Fatal("accepted history lost after failure/restart", status)
	}
	audits, err := db.QueryAudit(ctx, storage.AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(audits.Records) != 4 || audits.Records[1].AcceptedRows != 102 || len(audits.Records[1].Evidence) != 2 || !audits.Records[1].Evidence[0].Complete {
		t.Fatal("growth audit/evidence missing", audits)
	}
	samples, err := db.QuerySamples(ctx, storage.SampleQuery{QualityView: true, RecordQuery: storage.RecordQuery{Limit: 200}})
	if err != nil || len(samples.Records) != 102 {
		t.Fatal("replay multiplied canonical readings", err)
	}
	for _, sample := range samples.Records {
		if sample.Instant != nil || sample.Values["soc"].Decimal == nil || *sample.Values["soc"].Decimal != "51" {
			t.Fatal("correction not canonical")
		}
	}
}
