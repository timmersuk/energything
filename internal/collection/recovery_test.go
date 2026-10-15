package collection

import (
	"context"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryLongDeferralSurvivesRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	db, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	due := storage.At(now.Add(48 * time.Hour))
	failure := &storage.DiscoveryFailure{Class: "rate_limit", Code: "limited", DeferredUntil: &due}
	_, e = db.RecordOctopusDiscovery(ctx, []storage.DiscoveryState{{Provider: "octopus", Stream: "import", State: "failed", Occurred: storage.At(now), Failure: failure}, {Provider: "octopus", Stream: "export", State: "failed", Occurred: storage.At(now), Failure: failure}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s := NewScheduler(db, config.Config{Timezone: time.UTC, Octopus: config.Provider{State: "valid"}})
	calls := 0
	s.Now = func() time.Time { return now }
	s.Octopus = octopus.NewClient("synthetic", transport(func(*http.Request) (*http.Response, error) { calls++; return nil, context.Canceled }))
	s.Wait = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
	s.lane(ctx, "octopus")
	if calls != 0 {
		t.Fatal("restart ignored deferred discovery", calls)
	}
}

func TestTransientTelemetryDiscoveryRecoversWhileGenerationRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	now := time.Now().UTC()
	deviceCalls, generationCalls, latestCalls := 0, 0, 0
	s := NewScheduler(db, config.Config{Timezone: time.UTC, Growatt: config.Provider{State: "valid", PlantID: "P", LatestInterval: 5 * time.Minute, HistoryInterval: time.Hour}})
	s.Now = func() time.Time { return now }
	s.Growatt = growatt.NewClient("synthetic", transport(func(r *http.Request) (*http.Response, error) {
		code := 200
		body := ""
		switch r.URL.Path {
		case "/v1/plant/list":
			body = `{"error_code":0,"data":{"count":1,"plants":[{"plant_id":"P"}]}}`
		case "/v1/device/list":
			deviceCalls++
			body = `{"error_code":0,"data":{}}`
			if deviceCalls > 1 {
				body = `{"error_code":0,"data":{"count":1,"devices":[{"device_sn":"S","type":5}]}}`
			}
		case "/v1/plant/energy":
			generationCalls++
			code = 401
		case "/v1/device/mix/mix_last_data":
			latestCalls++
			body = `{"error_code":0,"data":{"serialNum":"S","time":"` + time.Now().UTC().Format("2006-01-02 15:04:05") + `","soc":0}}`
		default:
			code = 401
		}
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	waits := 0
	s.Wait = func(context.Context, time.Duration) error {
		waits++
		now = now.Add(15 * time.Minute)
		if latestCalls > 0 || waits > 8 {
			cancel()
			return context.Canceled
		}
		return nil
	}
	s.lane(ctx, "growatt")
	if deviceCalls != 2 || generationCalls == 0 || latestCalls != 1 {
		t.Fatal("stream recovery stalled", deviceCalls, generationCalls, latestCalls)
	}
	plans, e := db.QuerySchedules(context.Background())
	if e != nil || len(plans) != 7 {
		t.Fatal("independent bootstrap missing", len(plans), e)
	}
}
