package storage

import (
	"context"
	"testing"
	"time"
)

func TestQueueOpenDoesNotInterruptLiveAttempt(t *testing.T) {
	s, dir, _, j := fixture(t, "octopus")
	id := begin(t, s, j)
	queue, e := OpenQueue(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	defer queue.Close()
	jobs, e := queue.EnsureRange(context.Background(), "repair", []Job{{IdentityID: j.IdentityID, Kind: j.Kind, Policy: "range", Start: j.Start, End: j.End, NextDue: testStart}})
	if e != nil || len(jobs) != 1 {
		t.Fatal(e)
	}
	a, e := s.QueryAttempt(context.Background(), id)
	if e != nil || a.Outcome != "running" {
		t.Fatal("queue stole server attempt", a, e)
	}
	jobs[0].Start -= halfHour
	if _, e = queue.EnsureRange(context.Background(), "repair", jobs); e == nil {
		t.Fatal("range rebound")
	}
}

func TestHistoricalEmptyGapProgressAndRestart(t *testing.T) {
	ctx := context.Background()
	s, dir, i, _ := fixture(t, "octopus")
	seed := Schedule{IdentityID: i.ID, Kind: "history", Policy: "backfill", Frontier: testStart + 2*halfHour, SearchLimit: testStart - 2*halfHour, Interval: time.Hour, Provenance: "synthetic bounded domain"}
	planner := func(p Schedule) Job { return Job{Start: p.Frontier - 2*halfHour, End: p.Frontier} }
	prepare := func() *Job {
		j, e := s.PrepareSchedule(ctx, seed, testStart+3*halfHour, false, planner)
		if e != nil {
			t.Fatal(e)
		}
		return j
	}
	j := prepare()
	id := begin(t, s, *j)
	if e := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 4*halfHour, Evidence: []Evidence{{Content: []byte(`{"results":[]}`), Retrieved: testStart + 4*halfHour, Complete: true}}}); e != nil {
		t.Fatal(e)
	}
	s.Close()
	var e error
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	older := prepare()
	if older == nil || older.Start != testStart-2*halfHour {
		t.Fatal("empty stopped search or replayed", older)
	}
	olderID := begin(t, s, *older)
	if e := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: olderID, Finished: testStart + 4*halfHour, NextDue: testStart + 4*halfHour, Evidence: []Evidence{{Content: []byte(`{"results":[1]}`), Retrieved: testStart + 4*halfHour, Complete: true}}, Intervals: []Interval{{Start: older.Start, End: older.Start + halfHour, KWh: "0.1"}}}); e != nil {
		t.Fatal(e)
	}
	if j := prepare(); j != nil {
		t.Fatal("unbounded search", j)
	}
	p, e := s.QuerySchedules(ctx)
	if e != nil || len(p) != 1 || p[0].State != "search_limit_boundary_unresolved" {
		t.Fatal(p, e)
	}
	// A new oldest-reading observation extends a previously limited frontier,
	// without resetting accepted newer units.
	lower := testStart - 4*halfHour
	if e = s.RecordHistoryBoundary(ctx, HistoryBoundary{IdentityID: i.ID, Oldest: &lower, Observed: testStart + 4*halfHour, NextDue: testStart + 20*halfHour, State: "observed", Provenance: "synthetic explicit domain", Evidence: []byte(`{"probe":"oldest","results":[]}`)}); e != nil {
		t.Fatal(e)
	}
	continued := prepare()
	if continued == nil || continued.Start != lower || continued.End != older.Start {
		t.Fatal("new older boundary restarted history or did not extend", continued)
	}
	id = begin(t, s, *continued)
	if e = s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 4*halfHour, Evidence: []Evidence{{Content: []byte(`{"results":[]}`), Retrieved: testStart + 4*halfHour, Complete: true}}}); e != nil {
		t.Fatal(e)
	}
	if j := prepare(); j != nil {
		t.Fatal("past verified boundary", j)
	}
	p, e = s.QuerySchedules(ctx)
	if e != nil || p[0].State != "complete_at_observed_boundary" {
		t.Fatal(p, e)
	}
}

func TestRoutineCoalescesPrioritizesAndRepeatsAfterDue(t *testing.T) {
	ctx := context.Background()
	s, _, i, history := fixture(t, "octopus")
	seed := Schedule{IdentityID: i.ID, Kind: "history", Policy: "routine", Frontier: history.End, SearchLimit: history.Start - halfHour, Interval: time.Hour}
	plan := func(Schedule) Job { return Job{Start: history.Start, End: history.End} }
	one, e := s.PrepareSchedule(ctx, seed, testStart, true, plan)
	if e != nil {
		t.Fatal(e)
	}
	two, e := s.PrepareSchedule(ctx, seed, testStart, true, plan)
	if e != nil || one.ID != two.ID {
		t.Fatal("not coalesced", e)
	}
	back := seed
	back.Policy = "backfill"
	_, e = s.PrepareSchedule(ctx, back, testStart, false, plan)
	if e != nil {
		t.Fatal(e)
	}
	due, e := s.ScheduledJobs(ctx, testStart)
	if e != nil || len(due) != 2 || due[0].Policy != "routine" {
		t.Fatal(due, e)
	}
	accepted(t, s, *one, "0.1")
	if j, e := s.PrepareSchedule(ctx, seed, testStart+5*halfHour, false, plan); e != nil || j != nil {
		t.Fatal("early repeat", j, e)
	}
	three, e := s.PrepareSchedule(ctx, seed, testStart+7*halfHour, false, plan)
	if e != nil || three == nil || three.ID != one.ID {
		t.Fatal("routine never repeats", three, e)
	}
	due, e = s.ScheduledJobs(ctx, testStart+7*halfHour)
	if e != nil || len(due) < 1 || due[0].ID != one.ID {
		t.Fatal("accepted routine excluded", due, e)
	}
}
