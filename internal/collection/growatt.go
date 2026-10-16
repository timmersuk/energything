package collection

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

type growattStore interface {
	RecordGrowattDiscovery(context.Context, storage.DiscoveryState, []storage.Identity) ([]storage.Identity, error)
	EnsureJobs(context.Context, []storage.Job) ([]storage.Job, error)
	ResumeJob(context.Context, int64, storage.Instant) error
	BeginAttempt(context.Context, storage.AttemptRequest) (int64, error)
	CommitAcceptedWindow(context.Context, storage.AcceptedWindow) error
	RecordRejectedOrFailedAttempt(context.Context, storage.FailedAttempt) error
}

func DiscoverGrowatt(ctx context.Context, db growattStore, client *growatt.Client, plant, serial string) (growatt.Discovery, []storage.Identity, error) {
	state := storage.DiscoveryState{Provider: "growatt", Stream: "telemetry", State: "discovering", Occurred: storage.At(time.Now())}
	if _, e := db.RecordGrowattDiscovery(ctx, state, nil); e != nil {
		return growatt.Discovery{}, nil, errors.New("storage: Growatt discovery start unavailable")
	}
	d, e := client.Discover(ctx, plant, serial)
	state.State = d.State
	state.Occurred = storage.At(time.Now())
	if e != nil {
		state.State = "failed"
		var pe *growatt.Error
		if errors.As(e, &pe) {
			state.Failure = &storage.DiscoveryFailure{Class: pe.Class, Code: pe.Code, Page: pe.Page, Diagnostic: pe.Diagnostic}
			if !pe.DeferredUntil.IsZero() {
				due := storage.At(pe.DeferredUntil)
				state.Failure.DeferredUntil = &due
			}
		}
	}
	rows := []storage.Identity{}
	if e == nil {
		for _, v := range d.Candidates {
			rows = append(rows, storage.Identity{Provider: "growatt", Stream: "telemetry", StableKey: v.Key(), Selected: d.Selected != nil && d.Selected.Key() == v.Key(), Metadata: storage.DiscoveryMetadata{PlantID: v.PlantID, Serial: v.Serial, DeviceType: v.Family}})
		}
	}
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	identities, se := db.RecordGrowattDiscovery(auditCtx, state, rows)
	if se != nil {
		return d, nil, errors.New("storage: Growatt discovery result unavailable")
	}
	return d, identities, e
}

// CollectGrowattUnit runs one explicitly supplied latest or provider-day job.
// Task 06 owns automatic work creation and scheduling; this method has no timer.
func CollectGrowattUnit(ctx context.Context, db growattStore, client *growatt.Client, job storage.Job, device growatt.Device, zone string) error {
	id, e := db.BeginAttempt(ctx, storage.AttemptRequest{JobID: job.ID, Started: storage.At(time.Now())})
	if e != nil {
		return errors.New("storage: Growatt attempt not started")
	}
	var w growatt.Window
	var fetchErr error
	if job.Kind == "latest" {
		w, fetchErr = client.FetchLatest(ctx, device)
	} else {
		w, fetchErr = client.FetchDay(ctx, device, job.ProviderDate, zone)
	}
	retries := []storage.RetryDetail{}
	for _, r := range w.Retries {
		retries = append(retries, storage.RetryDetail{Page: r.Page, Retry: r.Attempt, HTTPStatus: r.Status, Code: r.Code, WaitMicroseconds: r.Wait.Microseconds()})
	}
	finished := storage.At(time.Now())
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if fetchErr == nil {
		for n := range w.Samples {
			growatt.QualifyClock(&w.Samples[n], zone)
		}
		e = db.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: finished, NextDue: finished, Evidence: w.Evidence, Samples: w.Samples, Retries: retries, TimestampQuality: len(w.Samples) > 0, Coverage: storage.Coverage{Valid: len(w.Samples), Qualified: len(w.Samples)}})
		if e == nil {
			return nil
		}
		code, outcome, class := "io", "failed", "storage"
		if errors.Is(e, storage.ErrInvalid) {
			code, outcome, class = "conflict", "rejected", "validation"
		}
		if ctx.Err() != nil {
			code, outcome = "cancelled", "cancelled"
		}
		if db.RecordRejectedOrFailedAttempt(auditCtx, storage.FailedAttempt{AttemptID: id, Finished: storage.At(time.Now()), NextDue: finished, Outcome: outcome, ErrorClass: class, ErrorCode: code, Evidence: w.Evidence, Retries: retries}) != nil {
			return errors.New("storage: Growatt commit and failure audit unavailable; no completion claimed")
		}
		return errors.New("storage: Growatt accepted commit failed; failure recorded")
	}
	var pe *growatt.Error
	if !errors.As(fetchErr, &pe) {
		pe = &growatt.Error{Class: "protocol", Code: "invalid"}
	}
	outcome := "failed"
	if pe.Class == "validation" {
		outcome = "rejected"
	}
	if pe.Code == "cancelled" {
		outcome = "cancelled"
	}
	due := retryDue(job, time.Now(), time.Time{})
	if !pe.DeferredUntil.IsZero() {
		due = retryDue(job, time.Now(), pe.DeferredUntil)
	}
	page := pe.Page
	if db.RecordRejectedOrFailedAttempt(auditCtx, storage.FailedAttempt{AttemptID: id, Finished: finished, NextDue: storage.At(due), Outcome: outcome, ErrorClass: pe.Class, ErrorCode: pe.Code, Page: &page, Diagnostic: pe.Diagnostic, Paused: pe.Class == "authentication" || pe.Class == "configuration", Evidence: w.Evidence, Retries: retries}) != nil {
		return errors.New("storage: Growatt failed audit unavailable; no completion claimed")
	}
	return fetchErr
}
