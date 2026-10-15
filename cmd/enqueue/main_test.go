package main

import (
	"context"
	"fmt"
	"github.com/timmersuk/energything/internal/storage"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestQueueProcessHelper(t *testing.T) {
	if os.Getenv("ENERGYTHING_TEST_QUEUE_HELPER") != "1" {
		return
	}
	if e := run(os.Args[len(os.Args)-10:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	os.Exit(0)
}
func TestCrossProcessEnqueuePreservesServerAttempt(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ids, e := db.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "synthetic", Selected: true}})
	if e != nil {
		t.Fatal(e)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "routine", Start: storage.At(start), End: storage.At(start.Add(time.Hour)), NextDue: storage.At(start)}})
	if e != nil {
		t.Fatal(e)
	}
	attempt, e := db.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: storage.At(time.Now())})
	if e != nil {
		t.Fatal(e)
	}
	enqueue := func(last string) error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestQueueProcessHelper$", "--", "-data-dir", dir, "-identity", fmt.Sprint(ids[0].ID), "-from", "2026-01-01", "-to", last, "-operation-id", "synthetic-repair")
		cmd.Env = append(os.Environ(), "ENERGYTHING_TEST_QUEUE_HELPER=1")
		out, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("helper: %w %s", e, out)
		}
		return nil
	}
	if e = enqueue("2026-01-02"); e != nil {
		t.Fatal(e)
	}
	if e = enqueue("2026-01-02"); e != nil {
		t.Fatal(e)
	}
	if e = enqueue("2026-01-03"); e == nil {
		t.Fatal("operation rebound")
	}
	a, e := db.QueryAttempt(ctx, attempt)
	if e != nil || a.Outcome != "running" {
		t.Fatal("live attempt recovered by CLI", a, e)
	}
	due, e := db.ScheduledJobs(ctx, storage.At(time.Now()))
	if e != nil || len(due) != 1 || due[0].Policy != "range" {
		t.Fatal(due, e)
	}
}
