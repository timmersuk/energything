package storage

import (
	"context"
	"testing"
	"time"
)

func TestMigrationResumesLegacyHistoryWithoutResettingProgress(t *testing.T) {
	ctx := context.Background()
	s, dir, base := aggregateFixture(t)
	if e := s.CommitAcceptedWindow(ctx, aggregateInput(begin(t, s, base), "1.25")); e != nil {
		t.Fatal(e)
	}
	oldFrontier := At(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	restoreSchemaSeven(t, s)
	// Reconstruct the already deployed schema006 and its stopped120-day plan.
	if _, e := s.db.Exec(`ALTER TABLE schedules DROP COLUMN search_policy; DELETE FROM schema_migrations WHERE version=7; INSERT INTO schedules(identity_id,kind,policy,frontier,search_limit,interval_us,next_due,state,provenance) VALUES(?,'aggregate','backfill',?,?,3600000000,?,'search_limit_boundary_unresolved','old120-day budget')`, base.IdentityID, oldFrontier, oldFrontier, testStart); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ReserveAggregateCall(ctx, testStart); e != nil {
		t.Fatal(e)
	}
	s.Close()
	var e error
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	plans, e := s.QuerySchedules(ctx)
	if e != nil || len(plans) != 1 {
		t.Fatal(plans, e)
	}
	p := plans[0]
	if p.Frontier != oldFrontier || p.SearchLimit != At(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || p.SearchPolicy != "calendar" || p.State != "active" {
		t.Fatal("migration lost frontier or remained stopped", p)
	}
	if count(t, s, "energy_aggregates") != 1 || count(t, s, "aggregate_calls") != 1 {
		t.Fatal("migration changed observations/evidence/quota")
	}
	jobs, e := s.EnsureJobs(ctx, []Job{base})
	if e != nil || jobs[0].Checkpoint == nil || *jobs[0].Checkpoint != base.End {
		t.Fatal("accepted checkpoint changed", jobs, e)
	}
	next, e := s.PrepareSchedule(ctx, p, testStart, false, func(saved Schedule) Job {
		start := time.UnixMicro(int64(saved.Frontier)).UTC().AddDate(0, 0, -7)
		return Job{Start: At(start), End: saved.Frontier, ProviderDate: start.Format("2006-01-02")}
	})
	if e != nil || next == nil || next.End != oldFrontier {
		t.Fatal("legacy plan did not resume from old frontier", next, e)
	}
}

func TestMigrationKeepsUnfinishedUnitAndRetryPause(t *testing.T) {
	ctx := context.Background()
	s, dir, base := aggregateFixture(t)
	frontier := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	start := frontier.AddDate(0, 0, -7)
	jobs, e := s.EnsureJobs(ctx, []Job{{IdentityID: base.IdentityID, Kind: "aggregate", Policy: "backfill", Start: At(start), End: At(frontier), ProviderDate: start.Format("2006-01-02"), NextDue: testStart}})
	if e != nil {
		t.Fatal(e)
	}
	job := jobs[0]
	if e = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: begin(t, s, job), Finished: testStart + 4*halfHour, NextDue: testStart + 8*halfHour, Outcome: "failed", ErrorClass: "authentication", ErrorCode: "unauthorized", Paused: true}); e != nil {
		t.Fatal(e)
	}
	restoreSchemaSeven(t, s)
	if _, e = s.db.Exec(`ALTER TABLE schedules DROP COLUMN search_policy; DELETE FROM schema_migrations WHERE version=7; INSERT INTO schedules(identity_id,kind,policy,frontier,search_limit,interval_us,next_due,state,provenance,current_job) VALUES(?,'aggregate','backfill',?,?,3600000000,?,'paused','old120-day budget',?)`, base.IdentityID, At(frontier), At(start.AddDate(0, 0, -120)), testStart+8*halfHour, job.ID); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, e := s.QuerySchedules(ctx)
	if e != nil || p[0].CurrentJob == nil || *p[0].CurrentJob != job.ID || p[0].Frontier != At(frontier) || p[0].State != "paused" || p[0].NextDue != testStart+8*halfHour {
		t.Fatal("unfinished unit changed", p, e)
	}
	jobs, e = s.EnsureJobs(ctx, []Job{job})
	if e != nil || jobs[0].ID != job.ID || jobs[0].RetryCount != 1 || !jobs[0].Paused || jobs[0].Checkpoint != nil || jobs[0].Start != At(start) || jobs[0].End != At(frontier) {
		t.Fatal("retry/pause/window changed", jobs, e)
	}
}

func TestCalendarHistoryContinuesAcrossYearsEmptyGapsAndRestart(t *testing.T) {
	ctx := context.Background()
	s, dir, base := aggregateFixture(t)
	frontier := At(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC))
	target := At(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	seed := Schedule{IdentityID: base.IdentityID, Kind: "aggregate", Policy: "backfill", SearchPolicy: "calendar", Frontier: frontier, SearchLimit: target, Interval: time.Hour}
	planner := func(p Schedule) Job {
		end := time.UnixMicro(int64(p.Frontier)).UTC()
		start := end.AddDate(0, 0, -7)
		if At(start) < p.SearchLimit {
			start = time.UnixMicro(int64(p.SearchLimit)).UTC()
		}
		return Job{Start: At(start), End: p.Frontier, ProviderDate: start.Format("2006-01-02")}
	}
	foundOlder := false
	priorStart := frontier
	for unit := 0; unit < 56; unit++ {
		j, e := s.PrepareSchedule(ctx, seed, testStart, false, planner)
		if e != nil || j == nil {
			t.Fatalf("stopped archive at unit %d: %v", unit, e)
		}
		if j.End != priorStart || j.Start >= j.End || j.End-j.Start > Instant(7*24*time.Hour/time.Microsecond) {
			t.Fatal("gap, replay or unbounded unit", j)
		}
		id := begin(t, s, *j)
		in := AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 4*halfHour, Evidence: []Evidence{{Content: []byte(`{"energys":[]}`), Retrieved: testStart + 4*halfHour, Complete: true}}}
		if unit == 1 {
			if e = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: in.Finished, NextDue: testStart + 8*halfHour, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "conflict", Evidence: in.Evidence}); e != nil {
				t.Fatal(e)
			}
			retry, e := s.PrepareSchedule(ctx, seed, testStart, false, planner)
			if e != nil || retry == nil || retry.ID != j.ID || retry.Checkpoint != nil || retry.NextDue != testStart+8*halfHour {
				t.Fatal("rejected year-boundary unit advanced", retry, e)
			}
			in.AttemptID = begin(t, s, *retry)
		}
		// A year of accepted empty/zero units must not hide older generation.
		if j.ProviderDate < "2025-01-01" {
			in.Aggregates = []Aggregate{{PlantID: "SYN-PLANT", Metric: "generation", Resolution: "day", Period: j.ProviderDate, Value: "1.25", Unit: "kWh", Calendar: AggregateCalendar, Quality: AggregateQuality("1.25")}}
			foundOlder = true
		} else if unit%2 == 0 {
			in.Aggregates = []Aggregate{{PlantID: "SYN-PLANT", Metric: "generation", Resolution: "day", Period: j.ProviderDate, Value: "0", Unit: "kWh", Calendar: AggregateCalendar, Quality: AggregateQuality("0")}}
		}
		if e = s.CommitAcceptedWindow(ctx, in); e != nil {
			t.Fatal(e)
		}
		priorStart = j.Start
		if unit == 25 {
			s.Close()
			s, e = Open(ctx, dir)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	defer s.Close()
	if !foundOlder {
		t.Fatal("older data after empty year was missed")
	}
	p, e := s.QuerySchedules(ctx)
	if e != nil || len(p) != 1 || p[0].SearchLimit >= target || p[0].SearchPolicy != "calendar" {
		t.Fatal(p, e)
	}
}

func TestCalendarScanClampsAtRepresentableDateWithoutYearZero(t *testing.T) {
	ctx := context.Background()
	s, _, base := aggregateFixture(t)
	start := At(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))
	end := At(time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC))
	seed := Schedule{IdentityID: base.IdentityID, Kind: "aggregate", Policy: "backfill", SearchPolicy: "calendar", Frontier: end, SearchLimit: start, Interval: time.Hour}
	plan := func(p Schedule) Job { return Job{Start: p.SearchLimit, End: p.Frontier, ProviderDate: "0001-01-01"} }
	j, e := s.PrepareSchedule(ctx, seed, testStart, false, plan)
	if e != nil || j == nil {
		t.Fatal(j, e)
	}
	if e = s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: begin(t, s, *j), Finished: testStart + 4*halfHour, NextDue: testStart + 4*halfHour, Evidence: []Evidence{{Content: []byte(`{"energys":[]}`), Retrieved: testStart + 4*halfHour, Complete: true}}}); e != nil {
		t.Fatal(e)
	}
	j, e = s.PrepareSchedule(ctx, seed, testStart, false, plan)
	if e != nil || j != nil {
		t.Fatal("calendar underflow", j, e)
	}
	p, e := s.QuerySchedules(ctx)
	if e != nil || p[0].State != "calendar_domain_scanned" || p[0].Frontier != start {
		t.Fatal(p, e)
	}
}
