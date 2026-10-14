// Package testsupport supplies synthetic task 03 fixtures through typed storage
// writes. It is never imported by the application binary.
package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/timmersuk/energything/internal/storage"
)

func pointer[T any](x T) *T { return &x }

func SeedBrowsing(ctx context.Context, s *storage.Store) error {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		return err
	}
	retrieved := storage.At(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	ids, err := s.UpsertIdentities(ctx, []storage.Identity{
		{Provider: "octopus", Stream: "import", StableKey: "synthetic-private-import", Selected: true},
		{Provider: "octopus", Stream: "export", StableKey: "synthetic-private-export", Selected: true},
		{Provider: "growatt", Stream: "device", StableKey: "synthetic-private-device", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "synthetic-plant", Serial: "synthetic-inverter"}},
		{Provider: "octopus", Stream: "import", StableKey: "synthetic-private-retired", Selected: true},
	})
	if err != nil {
		return err
	}
	accept := func(job storage.Job, intervals []storage.Interval, samples []storage.Sample) error {
		jobs, err := s.EnsureJobs(ctx, []storage.Job{job})
		if err != nil {
			return err
		}
		id, err := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: retrieved})
		if err != nil {
			return err
		}
		return s.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: retrieved, NextDue: retrieved + 3600000000, Evidence: []storage.Evidence{{Content: json.RawMessage(`{"synthetic":true,"telemetry":1}`), Retrieved: retrieved, Complete: true}}, Intervals: intervals, Samples: samples, TimestampQuality: len(samples) > 0})
	}
	for _, date := range []string{"2026-03-29", "2026-10-25"} {
		day, _ := time.ParseInLocation("2006-01-02", date, loc)
		from, to := storage.At(day), storage.At(day.AddDate(0, 0, 1))
		for _, identity := range ids[:2] {
			rows := []storage.Interval{}
			for t := from; t < to; t += 1800000000 {
				rows = append(rows, storage.Interval{Start: t, End: t + 1800000000, KWh: "0.100000000000000001"})
			}
			rows[1].KWh = "0.000"
			if err := accept(storage.Job{IdentityID: identity.ID, Kind: "history", Policy: "backfill", Start: from, End: to, NextDue: retrieved}, rows, nil); err != nil {
				return err
			}
		}
		a := from + 5400000000
		b := a + 3600000000
		samples := []storage.Sample{
			{ResolvedKey: pointer(date + "-a"), Instant: &a, ProviderDate: date, OriginalTime: map[string]string{"clock": "01:30 <script>window.fixtureExecuted=true</script>"}, Quality: "qualified", Values: map[string]storage.Value{"soc": {Decimal: nil, Unit: pointer("%"), Quality: "missing"}, "power": {Decimal: pointer("0"), Unit: pointer("W"), Quality: "valid"}}},
			{ResolvedKey: pointer(date + "-b"), Instant: &b, ProviderDate: date, OriginalTime: map[string]string{"clock": "01:30"}, Quality: "valid", Values: map[string]storage.Value{"soc": {Decimal: pointer("0"), Unit: pointer("%"), Quality: "valid"}, "counter": {Decimal: nil, Unit: nil, Quality: "unresolved"}}},
			{ProviderDate: date, OriginalTime: map[string]string{"clock": date + " 01:30"}, Quality: "unresolved", Values: map[string]storage.Value{}},
			{ProviderDate: date, OriginalTime: map[string]string{"clock": date + " 01:30"}, Quality: "unresolved", Values: map[string]storage.Value{}},
		}
		if err := accept(storage.Job{IdentityID: ids[2].ID, Kind: "history", Policy: "backfill", ProviderDate: date, Start: from, End: to, NextDue: retrieved}, nil, samples); err != nil {
			return fmt.Errorf("samples: %w", err)
		}
	}
	// A historical deselected identity retains partial readings and a sanitized
	// failed attempt; absence does not become fabricated zero energy.
	start := storage.At(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	job := storage.Job{IdentityID: ids[3].ID, Kind: "history", Policy: "backfill", Start: start, End: start + 86400000000, NextDue: retrieved}
	if err = accept(job, []storage.Interval{{Start: start, End: start + 1800000000, KWh: "0.250"}}, nil); err != nil {
		return err
	}
	jobs, err := s.EnsureJobs(ctx, []storage.Job{job})
	if err != nil {
		return err
	}
	aid, err := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: retrieved + 1000000})
	if err != nil {
		return err
	}
	if err = s.RecordRejectedOrFailedAttempt(ctx, storage.FailedAttempt{AttemptID: aid, Finished: retrieved + 2000000, NextDue: retrieved + 3600000000, Outcome: "failed", ErrorClass: "network", ErrorCode: "timeout", Paused: true}); err != nil {
		return err
	}
	ids[3].Selected = false
	if _, err = s.UpsertIdentities(ctx, []storage.Identity{ids[3]}); err != nil {
		return err
	}
	// Pending backfill and a running latest attempt expose stored operational states.
	jobs, err = s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[2].ID, Kind: "latest", Policy: "routine", Start: retrieved, End: retrieved + 1000000, NextDue: retrieved}})
	if err != nil {
		return err
	}
	_, err = s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: retrieved})
	return err
}
