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
	due, e := s.ReserveAggregateCall(ctx, testStart+Instant(time.Hour/time.Microsecond))
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
