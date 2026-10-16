package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func aggregateFixture(t *testing.T) (*Store, string, Job) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	s, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	ids, e := s.UpsertIdentities(ctx, []Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-PLANT", Selected: true, Metadata: DiscoveryMetadata{PlantID: "SYN-PLANT"}}})
	if e != nil {
		t.Fatal(e)
	}
	jobs, e := s.EnsureJobs(ctx, []Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: testStart, End: testStart + 48*halfHour, ProviderDate: "2026-10-01", NextDue: testStart}})
	if e != nil {
		t.Fatal(e)
	}
	return s, dir, jobs[0]
}
func aggregateInput(id int64, value string) AcceptedWindow {
	return AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 5*halfHour, Evidence: []Evidence{{Content: json.RawMessage(`{"energy":"` + value + `"}`), Retrieved: testStart + 4*halfHour, Complete: true}}, Aggregates: []Aggregate{{PlantID: "SYN-PLANT", Metric: "generation", Resolution: "day", Period: "2026-10-01", Value: value, Unit: "kWh", Calendar: AggregateCalendar, Quality: AggregateQuality(value)}}}
}
func TestAggregateReplayCorrectionAtomicReopen(t *testing.T) {
	s, dir, j := aggregateFixture(t)
	ctx := context.Background()
	for _, v := range []string{"1.000000000000000001", "1.000000000000000001", "2"} {
		if e := s.CommitAcceptedWindow(ctx, aggregateInput(begin(t, s, j), v)); e != nil {
			t.Fatal(e)
		}
	}
	if count(t, s, "energy_aggregates") != 1 || count(t, s, "aggregate_versions") != 3 || count(t, s, "samples") != 0 || count(t, s, "intervals") != 0 {
		t.Fatal("series mixed")
	}
	id := begin(t, s, j)
	in := aggregateInput(id, "3")
	in.Aggregates = append(in.Aggregates, in.Aggregates[0])
	in.Aggregates[1].Value = "4"
	if e := s.CommitAcceptedWindow(ctx, in); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if e := s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: in.Finished, NextDue: in.NextDue, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "conflict", Evidence: in.Evidence}); e != nil {
		t.Fatal(e)
	}
	id = begin(t, s, j)
	in = aggregateInput(id, "0")
	in.Aggregates = nil
	if e := s.CommitAcceptedWindow(ctx, in); e != nil {
		t.Fatal(e)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, e := s.QueryAggregates(ctx, AggregateQuery{PlantID: "SYN-PLANT"})
	if e != nil || len(p.Records) != 1 || p.Records[0].Value != "2" {
		t.Fatal(p, e)
	}
	a, e := s.QueryAttempt(ctx, id)
	if e != nil || a.Outcome != "empty" {
		t.Fatal(a, e)
	}
}
func TestAggregateQuotaDurable(t *testing.T) {
	s, dir, _ := aggregateFixture(t)
	ctx := context.Background()
	for n := 0; n < 10; n++ {
		due, e := s.ReserveAggregateCall(ctx, testStart+Instant(n)*Instant(5*time.Minute/time.Microsecond))
		if e != nil || !due.IsZero() {
			t.Fatal(due, e)
		}
	}
	s.Close()
	s, e := Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	due, e := s.ReserveAggregateCallWithDailyCap(ctx, testStart+Instant(time.Hour/time.Microsecond), true)
	if e != nil || due.UnixMicro() != int64(testStart+Instant(24*time.Hour/time.Microsecond)) {
		t.Fatal(due, e)
	}
}
func TestAggregateFaultPreservesAccepted(t *testing.T) {
	s, _, j := aggregateFixture(t)
	ctx := context.Background()
	if e := s.CommitAcceptedWindow(ctx, aggregateInput(begin(t, s, j), "1")); e != nil {
		t.Fatal(e)
	}
	id := begin(t, s, j)
	s.fault = func(stage string) error {
		if stage == "after_observations" {
			return errors.New("synthetic fault")
		}
		return nil
	}
	if e := s.CommitAcceptedWindow(ctx, aggregateInput(id, "2")); e == nil {
		t.Fatal("fault ignored")
	}
	s.fault = nil
	p, e := s.QueryAggregates(ctx, AggregateQuery{})
	if e != nil || p.Records[0].Value != "1" || count(t, s, "aggregate_versions") != 1 {
		t.Fatal(p, e)
	}
}

func TestAggregateRangeBoundsAndAtomicAcceptance(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		valid    bool
	}{{"2026-01-12", "2026-01-18", true}, {"2026-01-12", "2026-01-12", true}, {"2024-02-27", "2024-03-04", true}, {"2026-12-29", "2027-01-04", true}, {"2026-01-12", "2026-01-19", false}, {"2026-01-18", "2026-01-12", false}, {"2026-02-30", "2026-03-01", false}, {"0000-01-01", "0000-01-01", false}} {
		_, _, e := AggregateDateBounds(tc.from, tc.to)
		if (e == nil) != tc.valid {
			t.Fatal(tc, e)
		}
	}
	ctx := context.Background()
	s, dir, j := aggregateFixture(t)
	j.ID = 0
	j.End = j.Start + 7*48*halfHour
	jobs, e := s.EnsureJobs(ctx, []Job{j})
	if e != nil {
		t.Fatal(e)
	}
	j = jobs[0]
	in := aggregateInput(begin(t, s, j), "1")
	for n := 1; n < 7; n++ {
		x := in.Aggregates[0]
		x.Period = time.UnixMicro(int64(j.Start)).AddDate(0, 0, n).Format("2006-01-02")
		in.Aggregates = append(in.Aggregates, x)
	}
	if e = s.CommitAcceptedWindow(ctx, in); e != nil {
		t.Fatal(e)
	}
	in = aggregateInput(begin(t, s, j), "2")
	bad := in.Aggregates[0]
	bad.Period = "2026-10-08"
	in.Aggregates = append(in.Aggregates, bad)
	if e = s.CommitAcceptedWindow(ctx, in); !errors.Is(e, ErrInvalid) {
		t.Fatal("outside row accepted", e)
	}
	if count(t, s, "energy_aggregates") != 7 || count(t, s, "aggregate_versions") != 7 {
		t.Fatal("partial correction")
	}
	if e = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: in.AttemptID, Finished: in.Finished, NextDue: in.NextDue, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "aggregate_date", Evidence: in.Evidence}); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, e := s.QueryAggregates(ctx, AggregateQuery{})
	if e != nil || len(p.Records) != 7 || p.Records[0].Value != "1" || p.Records[6].Period != "2026-10-07" {
		t.Fatal(p, e)
	}
	j.End += 48 * halfHour
	if _, e = s.EnsureJobs(ctx, []Job{j}); !errors.Is(e, ErrInvalid) {
		t.Fatal("eight days accepted", e)
	}
}

func TestAggregateConfigurableCapKeepsSpacingAndLedger(t *testing.T) {
	s, dir, _ := aggregateFixture(t)
	ctx := context.Background()
	step := Instant(5 * time.Minute / time.Microsecond)
	for n := 0; n < 10; n++ {
		due, e := s.ReserveAggregateCall(ctx, testStart+Instant(n)*step)
		if e != nil || !due.IsZero() {
			t.Fatal(n, due, e)
		}
	}
	eleventh := testStart + 10*step
	if due, e := s.ReserveAggregateCallWithDailyCap(ctx, eleventh, true); e != nil || due.IsZero() {
		t.Fatal("default cap lost", due, e)
	}
	if due, e := s.ReserveAggregateCallWithDailyCap(ctx, eleventh, false); e != nil || !due.IsZero() {
		t.Fatal("11th reservation blocked", due, e)
	}
	if due, e := s.ReserveAggregateCallWithDailyCap(ctx, eleventh+1, false); e != nil || At(due) != eleventh+step {
		t.Fatal("spacing lost", due, e)
	}
	s.Close()
	var e error
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if due, e := s.ReserveAggregateCallWithDailyCap(ctx, eleventh+1, false); e != nil || At(due) != eleventh+step {
		t.Fatal("restart lost spacing", due, e)
	}
	if due, e := s.ReserveAggregateCallWithDailyCap(ctx, eleventh+step, true); e != nil || At(due) != testStart+step+Instant(24*time.Hour/time.Microsecond) {
		t.Fatal("restoring cap lost extra reservation", due, e)
	}
	if count(t, s, "aggregate_calls") != 11 {
		t.Fatal("ledger reset")
	}

}

func TestChangedAggregateBudgetOnlyReconsidersLocalDeferrals(t *testing.T) {
	s, _, original := aggregateFixture(t)
	ctx := context.Background()
	now := testStart + 4*halfHour
	deadline := now + 48*halfHour
	for n := 0; n < 10; n++ {
		if _, e := s.ReserveAggregateCall(ctx, testStart+Instant(n)*Instant(5*time.Minute/time.Microsecond)); e != nil {
			t.Fatal(e)
		}
	}
	for _, code := range []string{"local_budget", "limited"} {
		seed := Schedule{IdentityID: original.IdentityID, Kind: "aggregate", Policy: "backfill", Slot: 0, Frontier: original.End, SearchLimit: original.Start, Interval: time.Hour, Provenance: "synthetic quota experiment"}
		if code == "limited" {
			seed.Slot = 1
			seed.Frontier += 48 * halfHour
			seed.SearchLimit += 48 * halfHour
		}
		job, e := s.PrepareSchedule(ctx, seed, now, false, func(p Schedule) Job {
			return Job{Start: p.SearchLimit, End: p.Frontier, ProviderDate: time.UnixMicro(int64(p.SearchLimit)).UTC().Format("2006-01-02")}
		})
		if e != nil || job == nil {
			t.Fatal(e)
		}
		aid, e := s.BeginAttempt(ctx, AttemptRequest{JobID: job.ID, Started: now})
		if e != nil {
			t.Fatal(e)
		}
		if e = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: aid, Finished: now, NextDue: deadline, Outcome: "failed", ErrorClass: "rate_limit", ErrorCode: code}); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.ReconsiderAggregateDeferrals(ctx, now, false); e != nil {
		t.Fatal(e)
	}
	rows, e := s.QueryStatus(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, j := range rows[0].Jobs {
		a, e := s.QueryAudit(ctx, AuditQuery{IdentityID: j.IdentityID})
		if e != nil {
			t.Fatal(e)
		}
		for _, attempt := range a.Records {
			if attempt.JobID != j.ID {
				continue
			}
			want := deadline
			if attempt.ErrorCode == "local_budget" {
				want = now
			}
			if j.NextDue != want || j.RetryCount != 1 || j.Checkpoint != nil {
				t.Fatal("retry/progress changed", j, want)
			}
		}
	}
	if count(t, s, "aggregate_calls") != 10 {
		t.Fatal("reservations changed")
	}
}
