package testsupport

import (
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

// SeedAggregates is synthetic and never used by the application binary.
func SeedAggregates(ctx context.Context, s *storage.Store) error {
	now := storage.At(time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC))
	ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-PLANT-A", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-PLANT-A"}}, {Provider: "growatt", Stream: "generation", StableKey: "SYN-PLANT-B", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-PLANT-B"}}})
	if e != nil {
		return e
	}
	for _, id := range ids {
		for n := 0; n < 3; n++ {
			day := time.Date(2026, 1, 15+n, 0, 0, 0, 0, time.UTC)
			date := day.Format("2006-01-02")
			jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: id.ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(day), End: storage.At(day.AddDate(0, 0, 1)), ProviderDate: date, NextDue: now}})
			if e != nil {
				return e
			}
			attempt, e := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: now})
			if e != nil {
				return e
			}
			value := "1.234567890123456789"
			if n == 1 {
				value = "0"
			}
			e = s.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: attempt, Finished: now, NextDue: now, Evidence: []storage.Evidence{{Content: json.RawMessage(`{"date":"` + date + `","energy":"` + value + `"}`), Retrieved: now, Complete: true}}, Aggregates: []storage.Aggregate{{PlantID: id.Metadata.PlantID, Metric: "generation", Resolution: "day", Period: date, Value: value, Unit: "kWh", Calendar: storage.AggregateCalendar, Quality: storage.AggregateQuality(value)}}, Coverage: storage.Coverage{Valid: 1, Qualified: 1}})
			if e != nil {
				return e
			}
			if n == 2 {
				attempt, e = s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: now})
				if e != nil {
					return e
				}
				if e = s.RecordRejectedOrFailedAttempt(ctx, storage.FailedAttempt{AttemptID: attempt, Finished: now, NextDue: now, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "aggregate_decimal", Evidence: []storage.Evidence{{Content: json.RawMessage(`{"energy":null}`), Retrieved: now}}}); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
