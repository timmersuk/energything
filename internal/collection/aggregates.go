package collection

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

type aggregateStore interface {
	BeginAttempt(context.Context, storage.AttemptRequest) (int64, error)
	CommitAcceptedWindow(context.Context, storage.AcceptedWindow) error
	RecordRejectedOrFailedAttempt(context.Context, storage.FailedAttempt) error
	ReserveAggregateCall(context.Context, storage.Instant) (time.Time, error)
}

// CollectGrowattAggregateUnit accepts only explicit daily plant jobs. No scheduler.
func CollectGrowattAggregateUnit(ctx context.Context, db aggregateStore, client *growatt.Client, job storage.Job, plant string) error {
	if job.Kind != "aggregate" {
		return storage.ErrInvalid
	}
	id, e := db.BeginAttempt(ctx, storage.AttemptRequest{JobID: job.ID, Started: storage.At(time.Now())})
	if e != nil {
		return errors.New("storage: aggregate attempt unavailable")
	}
	w, fetchErr := client.FetchAggregateDay(ctx, plant, job.ProviderDate, func(ctx context.Context) (time.Time, error) {
		return db.ReserveAggregateCall(ctx, storage.At(time.Now()))
	})
	retries := []storage.RetryDetail{}
	for _, r := range w.Retries {
		retries = append(retries, storage.RetryDetail{Page: r.Page, Retry: r.Attempt, HTTPStatus: r.Status, Code: r.Code, WaitMicroseconds: r.Wait.Microseconds()})
	}
	finished := storage.At(time.Now())
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if fetchErr == nil {
		e = db.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: finished, NextDue: storage.At(time.Now().Add(5 * time.Minute)), Evidence: w.Evidence, Aggregates: w.Aggregates, Retries: retries, Coverage: storage.Coverage{Valid: len(w.Aggregates), Qualified: len(w.Aggregates)}})
		if e == nil {
			return nil
		}
		fetchErr = &growatt.Error{Class: "storage", Code: "io"}
	}
	var pe *growatt.Error
	if !errors.As(fetchErr, &pe) {
		pe = &growatt.Error{Class: "protocol", Code: "invalid"}
	}
	outcome := "failed"
	if pe.Class == "validation" {
		outcome = "rejected"
	}
	if ctx.Err() != nil {
		outcome = "cancelled"
		pe = &growatt.Error{Class: "network", Code: "cancelled"}
	}
	due := time.Now().Add(5 * time.Minute)
	if pe.DeferredUntil.After(due) {
		due = pe.DeferredUntil
	}
	page := pe.Page
	if e = db.RecordRejectedOrFailedAttempt(auditCtx, storage.FailedAttempt{AttemptID: id, Finished: storage.At(time.Now()), NextDue: storage.At(due), Outcome: outcome, ErrorClass: pe.Class, ErrorCode: pe.Code, Page: &page, Diagnostic: pe.Diagnostic, Paused: pe.Class == "configuration" || pe.Class == "authentication", Evidence: w.Evidence, Retries: retries}); e != nil {
		return errors.New("storage: aggregate failure audit unavailable; no completion claimed")
	}
	return fetchErr
}
