package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAttemptEvidenceOwnershipAndExclusion(t *testing.T) {
	s, _, _, job := fixture(t, "octopus")
	ctx := context.Background()
	id := accepted(t, s, job, "0.100")
	a, e := s.QueryAttempt(ctx, id)
	if e != nil || len(a.Evidence) != 1 {
		t.Fatal(a, e)
	}
	ref := a.Evidence[0]
	if content, e := s.QueryAttemptEvidence(ctx, id, ref.RetrievalID); e != nil || !json.Valid(content) {
		t.Fatal(e)
	}
	other := accepted(t, s, job, "0.100")
	if _, e = s.QueryAttemptEvidence(ctx, other, ref.RetrievalID); !errors.Is(e, ErrNotFound) {
		t.Fatal("dedup bypass", e)
	}
	// Historical unsafe/corrupted content must fail closed at read time too. These
	// writes intentionally bypass persistence validation only in this storage test.
	for _, content := range []string{`{"api_key":"private-secret"}`, `{"nested":{"headers":{"Authorization":"private-secret"}}}`, `{"account":{"address":"private"}}`, `{"exception":"private-secret"}`, `{"stack_trace":"private-secret"}`, `{"url":"https://private.test/?token=secret"}`, `invalid`, `{"x":"` + strings.Repeat("x", 4*1024*1024) + `"}`} {
		if _, e = s.db.Exec(`UPDATE evidence SET content=? WHERE id=?`, content, ref.EvidenceID); e != nil {
			t.Fatal(e)
		}
		if _, e = s.QueryAttemptEvidence(ctx, id, ref.RetrievalID); !errors.Is(e, ErrNotFound) {
			t.Fatal("unsafe evidence exposed", e)
		}
	}
}

func TestRetryHintDiagnosticSurvivesRestart(t *testing.T) {
	s, dir, _, job := fixture(t, "octopus")
	ctx := context.Background()
	id := begin(t, s, job)
	hint := testStart + 10*halfHour
	if e := s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: hint, Outcome: "failed", ErrorClass: "rate_limit", ErrorCode: "limited", Diagnostic: Diagnostic{HTTPStatus: 429, RetryHintSource: "retry_after_header", RetryHintAt: &hint, RateLimitBody: "invalid_json"}}); e != nil {
		t.Fatal(e)
	}
	s.Close()
	var e error
	s, e = Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a, e := s.QueryAttempt(ctx, id)
	if e != nil || a.Diagnostic.RetryHintAt == nil || *a.Diagnostic.RetryHintAt != hint || a.NextRetry == nil || *a.NextRetry != hint || a.Diagnostic.RetryHintSource != "retry_after_header" {
		t.Fatal(a, e)
	}
	if validateDiagnostic(Diagnostic{RetryHintSource: "private-secret"}) == nil || validateDiagnostic(Diagnostic{RateLimitBody: "private-secret"}) == nil {
		t.Fatal("unchecked text accepted")
	}
}
