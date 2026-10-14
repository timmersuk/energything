package main

import (
	"context"
	"fmt"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func main() {
	ctx := context.Background()
	root, e := os.MkdirTemp(".cache", "task05-browser-")
	must(e)
	db, e := storage.Open(ctx, root)
	must(e)
	must(testsupport.SeedBrowsing(ctx, db))
	ids, e := db.RecordGrowattDiscovery(ctx, storage.DiscoveryState{Provider: "growatt", Stream: "telemetry", State: "selected", Occurred: storage.At(time.Now())}, []storage.Identity{{Provider: "growatt", Stream: "telemetry", StableKey: `["123","SYN-A"]`, Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "123", Serial: "SYN-A", DeviceType: "5"}}})
	must(e)
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(from.Add(24 * time.Hour)), ProviderDate: "2026-10-13", NextDue: storage.At(time.Now())}})
	must(e)
	body := `{"error_code":0,"data":{"count":1,"datas":[{"time":"2026-10-13 01:30:00","soc":0,"bmsSOC":12,"ppv":123.4567890123456789,"pcharge1":0,"unknown":42}]}}`
	status := 200
	client := growatt.NewClient("synthetic", transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	device := growatt.Device{PlantID: "123", Serial: "SYN-A", Family: "5"}
	must(collection.CollectGrowattUnit(ctx, db, client, jobs[0], device, "Europe/London"))
	body = `{"error_code":0,"data":{"count":1,"datas":[{"soc":101}]}}`
	if collection.CollectGrowattUnit(ctx, db, client, jobs[0], device, "Europe/London") == nil {
		panic("invalid accepted")
	}
	status = 403
	if collection.CollectGrowattUnit(ctx, db, client, jobs[0], device, "Europe/London") == nil {
		panic("outage accepted")
	}
	must(db.Close())
	abs, _ := filepath.Abs(root)
	fmt.Println(abs)
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
