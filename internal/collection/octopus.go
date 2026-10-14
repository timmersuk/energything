// Package collection coordinates one-shot provider units through typed storage.
package collection

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

type octopusStore interface {
	RecordOctopusDiscovery(context.Context, []storage.DiscoveryState, []storage.Identity) ([]storage.Identity, error)
	EnsureJobs(context.Context, []storage.Job) ([]storage.Job, error)
	ResumeJob(context.Context, int64, storage.Instant) error
	BeginAttempt(context.Context, storage.AttemptRequest) (int64, error)
	CommitAcceptedWindow(context.Context, storage.AcceptedWindow) error
	RecordRejectedOrFailedAttempt(context.Context, storage.FailedAttempt) error
}

// OctopusOnce discovers once, then fetches one bounded unit per selected stream.
// No timer, scheduler retry or browser-triggered provider call exists here.
func OctopusOnce(ctx context.Context, db octopusStore, client *octopus.Client, p config.Provider, from, to time.Time) error {
	states := func(d octopus.Discovery) []storage.DiscoveryState {
		return []storage.DiscoveryState{{Provider: "octopus", Stream: "import", State: d.Import.State, Occurred: storage.At(time.Now())}, {Provider: "octopus", Stream: "export", State: d.Export.State, Occurred: storage.At(time.Now())}}
	}
	if _, err := db.RecordOctopusDiscovery(ctx, states(octopus.Discovery{Import: octopus.Stream{State: "discovering"}, Export: octopus.Stream{State: "discovering"}}), nil); err != nil {
		return errors.New("storage: discovery start not recorded")
	}
	account, err := client.Account(ctx, p.Account)
	if err != nil {
		// Account/address data and raw provider errors are never persisted.
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, storageErr := db.RecordOctopusDiscovery(auditCtx, states(octopus.Discovery{Import: octopus.Stream{State: "failed"}, Export: octopus.Stream{State: "failed"}}), nil); storageErr != nil {
			return errors.New("storage: discovery failure not recorded")
		}
		return err
	}
	d := octopus.Select(account, octopus.Choice{MPAN: p.ImportMPAN, Serial: p.ImportSerial}, octopus.Choice{MPAN: p.ExportMPAN, Serial: p.ExportSerial})
	identities := []storage.Identity{}
	meters := map[string]octopus.Meter{}
	for _, stream := range []octopus.Stream{d.Import, d.Export} {
		for _, m := range stream.Candidates {
			selected := stream.Meter != nil && stream.Meter.Key() == m.Key()
			identities = append(identities, storage.Identity{Provider: "octopus", Stream: m.Direction, StableKey: m.Key(), Selected: selected, Metadata: storage.DiscoveryMetadata{MPAN: m.MPAN, Serial: m.Serial}})
			meters[m.Key()] = m
		}
	}
	stored, err := db.RecordOctopusDiscovery(ctx, states(d), identities)
	if err != nil {
		return errors.New("storage: discovery not recorded")
	}
	var result error
	for _, identity := range stored {
		if !identity.Selected {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		jobs, err := db.EnsureJobs(ctx, []storage.Job{{IdentityID: identity.ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(to), NextDue: storage.At(time.Now())}})
		if err != nil {
			return errors.New("storage: one-shot work not recorded")
		}
		job := jobs[0]
		if err = db.ResumeJob(ctx, job.ID, storage.At(time.Now())); err != nil {
			return errors.New("storage: one-shot work not resumed")
		}
		// Explicit one-shot invocation permits a replay/correction of the same unit.
		if err = CollectOctopusWindow(ctx, db, client, job, meters[identity.StableKey]); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}
func CollectOctopusWindow(ctx context.Context, db octopusStore, client *octopus.Client, job storage.Job, m octopus.Meter) error {
	id, err := db.BeginAttempt(ctx, storage.AttemptRequest{JobID: job.ID, Started: storage.At(time.Now())})
	if err != nil {
		return errors.New("storage: attempt not started")
	}
	w, fetchErr := client.Fetch(ctx, m, time.UnixMicro(int64(job.Start)), time.UnixMicro(int64(job.End)))
	evidence := []storage.Evidence{}
	for _, e := range w.Evidence {
		evidence = append(evidence, storage.Evidence{Content: e.Content, Retrieved: storage.At(e.Retrieved), Complete: e.Complete})
	}
	retries := []storage.RetryDetail{}
	for _, r := range w.Retries {
		retries = append(retries, storage.RetryDetail{Page: r.Page, Retry: r.Attempt, HTTPStatus: r.Status, Code: r.Code, WaitMicroseconds: r.Wait.Microseconds()})
	}
	finished := storage.At(time.Now())
	// A separate bounded cleanup context can record cancellation after fetch stops.
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if fetchErr == nil {
		rows := []storage.Interval{}
		for _, r := range w.Intervals {
			rows = append(rows, storage.Interval{Start: storage.At(r.Start), End: storage.At(r.End), KWh: r.KWh, Evidence: r.Evidence})
		}
		if err = db.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: finished, NextDue: finished, Evidence: evidence, Intervals: rows, Retries: retries}); err == nil {
			return nil
		}
		outcome, code := "failed", "io"
		if ctx.Err() != nil {
			outcome, code = "cancelled", "cancelled"
		}
		if db.RecordRejectedOrFailedAttempt(auditCtx, storage.FailedAttempt{AttemptID: id, Finished: storage.At(time.Now()), NextDue: finished, Outcome: outcome, ErrorClass: "storage", ErrorCode: code, Evidence: evidence, Retries: retries}) != nil {
			return errors.New("storage: commit and failure audit unavailable; no completion claimed")
		}
		return errors.New("storage: accepted commit failed; failure recorded")
	}
	var providerErr *octopus.Error
	if !errors.As(fetchErr, &providerErr) {
		providerErr = &octopus.Error{Class: "protocol", Code: "invalid"}
	}
	outcome := "failed"
	if providerErr.Class == "validation" {
		outcome = "rejected"
	}
	if providerErr.Code == "cancelled" {
		outcome = "cancelled"
	}
	due := time.Now().Add(time.Minute)
	if !providerErr.DeferredUntil.IsZero() {
		due = providerErr.DeferredUntil
	}
	page := providerErr.Page
	err = db.RecordRejectedOrFailedAttempt(auditCtx, storage.FailedAttempt{AttemptID: id, Finished: finished, NextDue: storage.At(due), Outcome: outcome, ErrorClass: providerErr.Class, ErrorCode: providerErr.Code, Page: &page, Paused: providerErr.Class == "authentication" || providerErr.Class == "configuration", Evidence: evidence, Retries: retries})
	if err != nil {
		return errors.New("storage: failed attempt audit unavailable; no completion claimed")
	}
	return fetchErr
}

// StartupWindow makes the explicit one-shot path finite. Default: previous local
// midnight through the latest elapsed half hour; configured start: first <=14 days.
func StartupWindow(p config.Provider, location *time.Location, now time.Time) (time.Time, time.Time) {
	local := now.In(location)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	from := midnight.AddDate(0, 0, -1).UTC()
	if p.BackfillStart != nil {
		from = p.BackfillStart.UTC()
	}
	to := now.UTC().Truncate(octopus.Slot)
	if to.Sub(from) > octopus.MaxWindow {
		to = from.Add(octopus.MaxWindow)
	}
	return from, to
}
