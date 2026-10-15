package collection

import (
	"context"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"testing"
	"time"
)

func TestIndependentBootstrapAndCoalescedRoutine(t *testing.T) {
	ctx := context.Background()
	db, e := storage.Open(ctx, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.UpsertIdentities(ctx, []storage.Identity{
		{Provider: "octopus", Stream: "import", StableKey: "import", Selected: true},
		{Provider: "octopus", Stream: "export", StableKey: "export", Selected: true},
		{Provider: "growatt", Stream: "telemetry", StableKey: "telemetry", Selected: true},
		{Provider: "growatt", Stream: "generation", StableKey: "plant", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "plant"}},
	})
	if e != nil {
		t.Fatal(e)
	}
	c := config.Config{Timezone: time.UTC, Octopus: config.Provider{State: "valid", RefreshInterval: time.Hour}, Growatt: config.Provider{State: "valid", LatestInterval: 5 * time.Minute, HistoryInterval: time.Hour}}
	s := NewScheduler(db, c)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	for _, p := range []string{"octopus", "growatt"} {
		if e = s.prepare(ctx, p, "", false); e != nil {
			t.Fatal(e)
		}
	}
	plans, e := db.QuerySchedules(ctx)
	if e != nil || len(plans) != 11 {
		t.Fatal(len(plans), e)
	}
	ids, e := s.Refresh(ctx, "octopus", "import")
	if e != nil || len(ids) != 1 {
		t.Fatal(ids, e)
	}
	again, e := s.Refresh(ctx, "octopus", "import")
	if e != nil || len(again) != 1 || again[0] != ids[0] {
		t.Fatal("refresh duplicated", again, e)
	}
	jobs, e := db.ScheduledJobs(ctx, storage.At(now))
	if e != nil || len(jobs) != 11 {
		t.Fatal(len(jobs), e)
	}
	seenHistorical := false
	for _, j := range jobs {
		if j.Policy == "backfill" {
			seenHistorical = true
		} else if seenHistorical {
			t.Fatal("routine starved by history")
		}
		if j.End <= j.Start {
			t.Fatal(j)
		}
		if j.Kind == "aggregate" && j.End-j.Start != storage.Instant(7*24*time.Hour/time.Microsecond) {
			t.Fatal("aggregate not seven dates", j)
		}
	}
}

func TestBoundedWindowsAndCancelledWait(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	i := storage.Identity{Provider: "growatt", Stream: "telemetry"}
	c := config.Config{Timezone: time.UTC}
	p := plans(i, c, now)
	history := p[len(p)-1]
	units := 0
	for history.Frontier > history.SearchLimit {
		j := plannedWindow(history, i, now, time.UTC)
		if j.Start < history.SearchLimit || j.End-j.Start != storage.Instant(24*time.Hour/time.Microsecond) {
			t.Fatal(j)
		}
		history.Frontier = j.Start
		units++
	}
	if units != SearchDays+1 {
		t.Fatal(units)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := wait(ctx, time.Hour); e == nil {
		t.Fatal("wait ignored cancellation")
	}
	job := storage.Job{RetryCount: 20}
	deferUntil := now.Add(48 * time.Hour)
	if !retryDue(job, now, deferUntil).Equal(deferUntil) {
		t.Fatal("long Retry-After ignored")
	}
	if got := retryDue(job, now, time.Time{}); got.Before(now.Add(time.Hour)) || got.After(now.Add(time.Hour+time.Second)) {
		t.Fatal(got)
	}
}
