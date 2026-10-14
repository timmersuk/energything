package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRealisticIndexedQueries(t *testing.T) {
	ctx := context.Background()
	s, _, identity, j := fixture(t, "octopus")
	a := accepted(t, s, j, "0.100")
	// Real rows, real SQLite planner. Bulk fixture SQL stays inside storage tests.
	_, err := s.db.Exec(`WITH RECURSIVE seq(n) AS (VALUES(2) UNION ALL SELECT n+1 FROM seq WHERE n<20001)
 INSERT INTO intervals(identity_id,start,end,kwh,retrieved,retrieval_id)
 SELECT ?,?+n*1800000000,?+(n+1)*1800000000,'0.001',?,1 FROM seq`, identity.ID, testStart, testStart, testStart+4*halfHour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`WITH RECURSIVE seq(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM seq WHERE n<20000)
 INSERT INTO samples(identity_id,resolved_key,attempt_id,ordinal,instant,provider_date,original_time,quality,values_json,retrieved,retrieval_id)
 SELECT ?,'synthetic/'||n,?,n,?+n*1000000,'2026-10-01','{}','valid','{}',?,1 FROM seq`, identity.ID, a, testStart, testStart+4*halfHour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`WITH RECURSIVE seq(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM seq WHERE n<20000)
 INSERT INTO attempts(job_id,identity_id,kind,start,end,provider_date,started,occurred,finished,outcome,retry_count)
 SELECT ?,?,'history',?,?,'',?,?,?,'failed',0 FROM seq`, j.ID, identity.ID, j.Start, j.End, testStart, testStart+4*halfHour, testStart+4*halfHour)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		sql, index string
		args       []any
	}{
		{`SELECT * FROM intervals WHERE identity_id=? AND start>=? AND start<? ORDER BY start,identity_id LIMIT 100`, "sqlite_autoindex_intervals", []any{identity.ID, testStart, testStart + 20*halfHour}},
		{`SELECT * FROM intervals WHERE start>=? ORDER BY start,identity_id LIMIT 100`, "intervals_time", []any{testStart}},
		{`SELECT * FROM samples WHERE identity_id=? AND instant>=? AND instant<? ORDER BY instant,id LIMIT 100`, "samples_source_time", []any{identity.ID, testStart, testStart + halfHour}},
		{`SELECT * FROM samples WHERE instant>=? ORDER BY instant,id LIMIT 100`, "samples_time", []any{testStart}},
		{`SELECT * FROM samples WHERE quality=? AND provider_date=? ORDER BY retrieved,id LIMIT 100`, "samples_quality_date", []any{"valid", "2026-10-01"}},
		{`SELECT * FROM attempts WHERE identity_id=? AND occurred>=? ORDER BY occurred DESC,id DESC LIMIT 100`, "attempts_source_time", []any{identity.ID, testStart}},
		{`SELECT * FROM attempts WHERE outcome=? AND occurred>=? ORDER BY occurred DESC,id DESC LIMIT 100`, "attempts_outcome_time", []any{"failed", testStart}},
	} {
		rows, err := s.db.Query("EXPLAIN QUERY PLAN "+tc.sql, tc.args...)
		if err != nil {
			t.Fatal(err)
		}
		plan := ""
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan += detail + "\n"
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan, tc.index) || strings.Contains(plan, "USE TEMP B-TREE") {
			t.Fatalf("query not indexed as expected: %s", plan)
		}
		t.Log(strings.TrimSpace(plan))
	}
	p, err := s.QueryIntervals(ctx, RecordQuery{IdentityID: identity.ID, From: ptr(testStart + 100*halfHour), To: ptr(testStart + 200*halfHour), Limit: 50})
	if err != nil || len(p.Records) != 50 || p.NextCursor == "" {
		t.Fatal("bounded indexed intervals", err)
	}
	samples, err := s.QuerySamples(ctx, SampleQuery{RecordQuery: RecordQuery{IdentityID: identity.ID, Limit: 100}})
	if err != nil || len(samples.Records) != 100 || samples.NextCursor == "" {
		t.Fatal("bounded samples", err)
	}
	next, err := s.QuerySamples(ctx, SampleQuery{RecordQuery: RecordQuery{IdentityID: identity.ID, Limit: 100, Cursor: samples.NextCursor}})
	if err != nil || next.Records[0].ID <= samples.Records[99].ID {
		t.Fatal("sample cursor", err)
	}
	audit, err := s.QueryAudit(ctx, AuditQuery{IdentityID: identity.ID, Outcome: "failed", Limit: 100})
	if err != nil || len(audit.Records) != 100 || audit.Counts.Failed != 20000 {
		t.Fatal("bounded audit aggregates", err)
	}
}

func TestCancelledInFlightTransactionAndPoolWait(t *testing.T) {
	ctx := context.Background()
	s, _, _, j := fixture(t, "octopus")
	id := begin(t, s, j)
	cancelled, cancel := context.WithCancel(ctx)
	defer cancel()
	s.fault = func(boundary string) error {
		if boundary == "after_observations" {
			cancel()
		}
		return nil
	}
	err := s.CommitAcceptedWindow(cancelled, AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, Evidence: []Evidence{{json.RawMessage(`{"reading":1}`), testStart + 4*halfHour, true}}, Intervals: []Interval{{Start: testStart, End: testStart + halfHour, KWh: "0.100"}}})
	s.fault = nil
	if !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight cancellation ignored", err)
	}
	if count(t, s, "intervals") != 0 || count(t, s, "evidence") != 0 || count(t, s, "completions") != 0 {
		t.Fatal("cancelled transaction persisted")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waiting, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	if _, err = s.QueryIntervals(waiting, RecordQuery{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting reader ignored cancellation", err)
	}
}

func TestResolvedSampleCorrectionsAndFinishedBackfill(t *testing.T) {
	ctx := context.Background()
	s, _, _, j := fixture(t, "growatt")
	for _, value := range []string{"0", "0", "1"} {
		err := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: begin(t, s, j), Finished: testStart + 4*halfHour, TimestampQuality: true,
			Evidence: []Evidence{{json.RawMessage(`{"soc":"` + value + `"}`), testStart + 4*halfHour, true}},
			Samples:  []Sample{{ResolvedKey: ptr("synthetic-provider-key"), Instant: ptr(testStart), ProviderDate: j.ProviderDate, OriginalTime: map[string]string{"local": "synthetic"}, Quality: "qualified", Values: map[string]Value{"soc": {ptr(value), ptr("%"), "valid"}}}},
			Coverage: Coverage{Expected: 1, Valid: 1, Qualified: 1}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count(t, s, "samples") != 1 || count(t, s, "sample_versions") != 3 || count(t, s, "evidence") != 2 {
		t.Fatal("resolved correction/replay lost provenance")
	}
	due, err := s.DueJobs(ctx, testStart+9*halfHour, 100)
	if err != nil || len(due) != 0 {
		t.Fatal("completed backfill reruns", err)
	}
	p, err := s.QuerySamples(ctx, SampleQuery{})
	if err != nil || *p.Records[0].Values["soc"].Decimal != "1" {
		t.Fatal("current sample", err)
	}
	a, err := s.QueryAudit(ctx, AuditQuery{ErrorClass: "timestamp_quality"})
	if err != nil || len(a.Records) != 3 || a.Records[0].Outcome != "success" {
		t.Fatal("timestamp quality described as failed fetch", err)
	}
}
