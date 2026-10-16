package testsupport

import (
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

// SeedAudit supplies only synthetic identities and selected telemetry. Equal
// occurrence times deliberately exercise the audit ID ordering tie-break.
func SeedAudit(ctx context.Context, s *storage.Store) error {
	ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "audit-synthetic", Selected: true, Metadata: storage.DiscoveryMetadata{MPAN: "SYN-MPAN", Serial: "SYN-METER"}}})
	if e != nil {
		return e
	}
	start := storage.At(time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC))
	now := start + 48*3600000000
	jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: start, End: start + 1800000000, NextDue: now}})
	if e != nil {
		return e
	}
	for _, outcome := range []string{"success", "rejected", "failed", "failed", "failed", "success", "empty", "cancelled", "limited"} {
		id, e := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: now})
		if e != nil {
			return e
		}
		evidence := []storage.Evidence{{Content: json.RawMessage(`{"results":[{"interval_start":"2026-10-25T00:00:00Z","interval_end":"2026-10-25T00:30:00Z","consumption":0.5,"unknown_telemetry":"<script>window.auditInjected=true</script>"}]}`), Retrieved: now, Complete: true}}
		if outcome == "success" || outcome == "empty" {
			rows := []storage.Interval{}
			if outcome == "success" {
				rows = append(rows, storage.Interval{Start: start, End: start + 1800000000, KWh: "0.5"})
			} else {
				evidence[0].Content = json.RawMessage(`{"results":[]}`)
			}
			e = s.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: now, NextDue: now, Evidence: evidence, Intervals: rows})
		} else {
			class, code, terminal := "network", "timeout", outcome
			if outcome == "rejected" {
				class = "validation"
				code = "invalid_duration"
			}
			if outcome == "cancelled" {
				code = "cancelled"
			}
			if outcome == "limited" {
				class = "rate_limit"
				code = "limited"
				terminal = "failed"
				evidence = nil
			}
			page := 1
			e = s.RecordRejectedOrFailedAttempt(ctx, storage.FailedAttempt{AttemptID: id, Finished: now, NextDue: now + 3600000000, Outcome: terminal, ErrorClass: class, ErrorCode: code, Page: &page, Evidence: evidence})
		}
		if e != nil {
			return e
		}
	}
	return nil
}

// SeedGenerationProgress represents retained January rows with an unfinished
// August frontier deferred by the aggregate budget. Bounds are bookkeeping only.
func SeedGenerationProgress(ctx context.Context, s *storage.Store) error {
	identities, e := s.Identities(ctx)
	if e != nil {
		return e
	}
	for _, identity := range identities {
		if identity.Provider != "growatt" || identity.Stream != "generation" {
			continue
		}
		now := storage.At(time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC))
		frontier := storage.At(time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC))
		boundary := storage.At(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))
		job, e := s.PrepareSchedule(ctx, storage.Schedule{IdentityID: identity.ID, Kind: "aggregate", Policy: "backfill", SearchPolicy: "bounded", Frontier: frontier, SearchLimit: boundary, Interval: 24 * time.Hour, Provenance: "Synthetic configured inclusive history boundary"}, now, false, func(storage.Schedule) storage.Job {
			return storage.Job{Start: frontier - 7*86400000000, End: frontier, ProviderDate: "2026-08-08"}
		})
		if e != nil {
			return e
		}
		if job == nil {
			continue
		}
		id, e := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: job.ID, Started: now})
		if e != nil {
			return e
		}
		return s.RecordRejectedOrFailedAttempt(ctx, storage.FailedAttempt{AttemptID: id, Finished: now, NextDue: now + 20*3600000000, Outcome: "failed", ErrorClass: "rate_limit", ErrorCode: "local_budget"})
	}
	return nil
}
