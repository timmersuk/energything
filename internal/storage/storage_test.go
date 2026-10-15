package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testStart = At(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))

const halfHour = Instant(1800 * 1e6)

func ptr[T any](x T) *T { return &x }
func fixture(t *testing.T, provider string) (*Store, string, Identity, Job) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	stream := "import"
	date := ""
	if provider == "growatt" {
		stream = "device"
		date = "2026-10-01"
	}
	ids, err := s.UpsertIdentities(context.Background(), []Identity{{Provider: provider, Stream: stream, StableKey: "synthetic/source", Selected: true}})
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := s.EnsureJobs(context.Background(), []Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: testStart, End: testStart + 2*halfHour, ProviderDate: date, NextDue: testStart}})
	if err != nil {
		t.Fatal(err)
	}
	return s, dir, ids[0], jobs[0]
}
func begin(t *testing.T, s *Store, j Job) int64 {
	t.Helper()
	id, err := s.BeginAttempt(context.Background(), AttemptRequest{j.ID, testStart + 3*halfHour})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func accepted(t *testing.T, s *Store, j Job, kwh string) int64 {
	t.Helper()
	id := begin(t, s, j)
	err := s.CommitAcceptedWindow(context.Background(), AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 5*halfHour,
		Evidence:  []Evidence{{json.RawMessage(fmt.Sprintf(`{"reading":"%s","unknown_telemetry":7}`, kwh)), testStart + 4*halfHour, true}},
		Intervals: []Interval{{Start: testStart, End: testStart + halfHour, KWh: kwh}, {Start: testStart + halfHour, End: testStart + 2*halfHour, KWh: "0.000"}}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func count(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestAcceptedReplayCorrectionReopen(t *testing.T) {
	ctx := context.Background()
	s, dir, _, j := fixture(t, "octopus")
	first := accepted(t, s, j, "0.100")
	second := accepted(t, s, j, "0.100")
	third := accepted(t, s, j, "0.120")
	if count(t, s, "intervals") != 2 || count(t, s, "evidence") != 2 || count(t, s, "retrievals") != 3 || count(t, s, "interval_versions") != 6 {
		t.Fatal("replay/correction provenance lost")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	p, err := reopened.QueryIntervals(ctx, RecordQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Records) != 2 || p.Records[0].KWh != "0.120" || p.Records[1].KWh != "0.000" {
		t.Fatalf("records %+v", p)
	}
	a, err := reopened.QueryAudit(ctx, AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Records) != 3 || a.Records[0].ID != third || a.Records[1].ID != second || a.Records[2].ID != first || len(a.Records[2].Evidence) != 1 {
		t.Fatalf("audit %+v", a)
	}
	status, err := reopened.QueryStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	x := status[0]
	if *x.Jobs[0].Checkpoint != j.End || !x.Windows[0].Coverage.MeasurementComplete || *x.NewestMeasurement != j.End || *x.LastRetrieval != testStart+4*halfHour {
		t.Fatalf("status %+v", x)
	}
}
func TestRejectedAndTransactionFaults(t *testing.T) {
	ctx := context.Background()
	s, _, _, j := fixture(t, "octopus")
	accepted(t, s, j, "0.100")
	id := begin(t, s, j)
	bad := AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, Evidence: []Evidence{{json.RawMessage(`{"suspect":true}`), testStart + 4*halfHour, true}}, Intervals: []Interval{{Start: testStart, End: testStart + halfHour, KWh: "0.120"}, {Start: testStart + halfHour, End: j.End, KWh: "-1"}}}
	if err := s.CommitAcceptedWindow(ctx, bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid window: %v", err)
	}
	bad.Intervals[1] = Interval{Start: testStart, End: testStart + halfHour, KWh: "0.999"}
	if err := s.CommitAcceptedWindow(ctx, bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("conflict: %v", err)
	}
	if err := s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: bad.Finished, NextDue: bad.Finished + halfHour, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "conflict", Evidence: bad.Evidence}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.QueryIntervals(ctx, RecordQuery{})
	if p.Records[0].KWh != "0.100" || count(t, s, "completions") != 1 {
		t.Fatal("rejection changed accepted state")
	}
	for _, boundary := range []string{"after_evidence", "after_observations", "before_commit"} {
		t.Run(boundary, func(t *testing.T) {
			id := begin(t, s, j)
			beforeEvidence := count(t, s, "evidence")
			beforeVersions := count(t, s, "interval_versions")
			s.fault = func(b string) error {
				if b == boundary {
					return errors.New("synthetic transaction failure")
				}
				return nil
			}
			bad.AttemptID = id
			bad.Evidence[0].Content = json.RawMessage(`{"different":9}`)
			bad.Intervals = bad.Intervals[:1]
			if err := s.CommitAcceptedWindow(ctx, bad); err == nil {
				t.Fatal("fault ignored")
			}
			s.fault = nil
			if count(t, s, "evidence") != beforeEvidence || count(t, s, "interval_versions") != beforeVersions {
				t.Fatal("partial transaction persisted")
			}
			var outcome string
			if err := s.db.QueryRow(`SELECT outcome FROM attempts WHERE id=?`, id).Scan(&outcome); err != nil || outcome != "running" {
				t.Fatal("false terminal result", outcome, err)
			}
			if _, err := s.RecoverInterruptedAttempts(ctx, bad.Finished); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestMissingSlotsAndDeselection(t *testing.T) {
	ctx := context.Background()
	s, _, identity, j := fixture(t, "octopus")
	accepted(t, s, j, "0.100")
	id := begin(t, s, j)
	err := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, Evidence: []Evidence{{json.RawMessage(`{"slot":1}`), testStart + 4*halfHour, true}}, Intervals: []Interval{{Start: testStart, End: testStart + halfHour, KWh: "0.120"}}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := s.QueryStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status[0].IntervalCount != 2 || status[0].Windows[0].Coverage.Missing != 1 || status[0].Windows[0].Coverage.MeasurementComplete {
		t.Fatal("absence deleted history or fabricated coverage")
	}
	identity.Selected = false
	if _, err = s.UpsertIdentities(ctx, []Identity{identity}); err != nil {
		t.Fatal(err)
	}
	due, err := s.DueJobs(ctx, testStart+9*halfHour, 100)
	if err != nil || len(due) != 0 {
		t.Fatal("deselected job eligible", err)
	}
	if count(t, s, "intervals") != 2 {
		t.Fatal("deselection deleted history")
	}
}
func TestGrowattNullRepeatedTimeAndEmpty(t *testing.T) {
	ctx := context.Background()
	s, dir, _, j := fixture(t, "growatt")
	id := begin(t, s, j)
	values := map[string]Value{"soc": {nil, ptr("%"), "missing"}, "unknown_power": {nil, nil, "missing"}}
	samples := []Sample{{OriginalTime: map[string]string{"local": "2026-10-25 01:30"}, Quality: "unresolved", Values: values, ProviderDate: j.ProviderDate},
		{OriginalTime: map[string]string{"local": "2026-10-25 01:30"}, Quality: "unresolved", Values: map[string]Value{"soc": {ptr("0"), ptr("%"), "valid"}}, ProviderDate: j.ProviderDate}}
	in := AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, Evidence: []Evidence{{json.RawMessage(`{"local":"2026-10-25 01:30","telemetry":[null,0]}`), testStart + 4*halfHour, true}}, Samples: samples}
	if err := s.CommitAcceptedWindow(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.AttemptID = begin(t, s, j)
	if err := s.CommitAcceptedWindow(ctx, in); err != nil {
		t.Fatal(err)
	}
	if count(t, s, "samples") != 4 || count(t, s, "evidence") != 1 || count(t, s, "retrievals") != 2 {
		t.Fatal("unresolved occurrences overwritten")
	}
	// Empty expired history is fetched, not measured, and never deletes local data.
	if err := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: begin(t, s, j), Finished: in.Finished, Evidence: []Evidence{{json.RawMessage(`{"history":[]}`), in.Finished, true}}}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.QuerySamples(ctx, SampleQuery{QualityView: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Records) != 4 || p.Records[0].Instant != nil || p.Records[0].ResolvedKey != nil || p.Records[0].Values["soc"].Decimal != nil || *p.Records[1].Values["soc"].Decimal != "0" {
		t.Fatalf("samples %+v", p)
	}
	encoded, _ := json.Marshal(p)
	if p.Records[0].Values["unknown_power"].Unit != nil || !strings.Contains(string(encoded), `"unit":null`) {
		t.Fatal("missing unit became a fabricated unit")
	}
	if !strings.Contains(string(encoded), `"value":null`) || !strings.Contains(string(encoded), `"value":"0"`) {
		t.Fatal("JSON lost null/zero", string(encoded))
	}
	absolute, err := s.QuerySamples(ctx, SampleQuery{})
	if err != nil || len(absolute.Records) != 0 || absolute.OmittedUnresolved != 4 {
		t.Fatal("absolute query guessed UTC", absolute, err)
	}
	status, err := s.QueryStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status[0].LastAttempt.Outcome != "empty" || status[0].Windows[0].Coverage.MeasurementComplete || status[0].SampleCount != 4 {
		t.Fatal("empty erased history or measured zero")
	}
}
func TestInterruptedRecoveryAndNonoverlap(t *testing.T) {
	ctx := context.Background()
	s, dir, _, j := fixture(t, "octopus")
	id := begin(t, s, j)
	if _, err := s.BeginAttempt(ctx, AttemptRequest{j.ID, testStart}); !errors.Is(err, ErrConflict) {
		t.Fatal("overlap allowed", err)
	}
	s.Close()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.QueryAudit(ctx, AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Records[0].ID != id || a.Records[0].Outcome != "interrupted" {
		t.Fatal("not recovered")
	}
	due, err := s.DueJobs(ctx, At(time.Now().Add(time.Minute)), 100)
	if err != nil || len(due) != 1 || due[0].Checkpoint != nil || due[0].Start != j.Start || due[0].End != j.End {
		t.Fatal("recovery advanced unfinished work", due, err)
	}
	if n, err := s.RecoverInterruptedAttempts(ctx, testStart); err != nil || n != 0 {
		t.Fatal("recovery not idempotent", err)
	}
}
func TestPaginationFiltersSanitizationAndRangeIDs(t *testing.T) {
	ctx := context.Background()
	s, _, _, j := fixture(t, "octopus")
	accepted(t, s, j, "0.100")
	for i := 0; i < 4; i++ {
		id := begin(t, s, j)
		if err := s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: testStart + 4*halfHour, Outcome: "failed", ErrorClass: "network", ErrorCode: "incomplete", Evidence: []Evidence{{json.RawMessage(`{"page":1}`), testStart + 4*halfHour, false}}, Retries: []RetryDetail{{Page: 1, Retry: 1, HTTPStatus: 500, Code: "unavailable", WaitMicroseconds: 1e6}}}); err != nil {
			t.Fatal(err)
		}
	}
	q := AuditQuery{Provider: "octopus", Outcome: "failed", From: ptr(testStart), To: ptr(testStart + 9*halfHour), Limit: 2}
	a, err := s.QueryAudit(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Records) != 2 || a.Counts.Total != 4 || a.Counts.Failed != 4 || a.NextCursor == "" || a.Records[0].Evidence[0].Complete {
		t.Fatal("audit filters/completeness", a)
	}
	q.Cursor = a.NextCursor
	b, err := s.QueryAudit(ctx, q)
	if err != nil || len(b.Records) != 2 || b.Records[0].ID >= a.Records[1].ID || b.NextCursor != "" {
		t.Fatal("unstable audit paging", b, err)
	}
	q.Provider = "growatt"
	if _, err = s.QueryAudit(ctx, q); !errors.Is(err, ErrInvalid) {
		t.Fatal("cursor filter mismatch accepted")
	}
	p, err := s.QueryIntervals(ctx, RecordQuery{Limit: 1})
	if err != nil || p.NextCursor == "" {
		t.Fatal("interval cursor", err)
	}
	p2, err := s.QueryIntervals(ctx, RecordQuery{Limit: 1, Cursor: p.NextCursor})
	if err != nil || len(p2.Records) != 1 || p2.Records[0].Start <= p.Records[0].Start {
		t.Fatal("interval paging", err)
	}
	if _, _, err = canonicalEvidence(json.RawMessage(`{"nested":{"api_token":"synthetic-secret"}}`)); err == nil {
		t.Fatal("secret field accepted")
	}
	id := begin(t, s, j)
	err = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: testStart + 4*halfHour, Outcome: "failed", ErrorClass: "synthetic-secret", ErrorCode: "invalid"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("free error class accepted")
	}
	s.RecoverInterruptedAttempts(ctx, testStart+4*halfHour)
	raw, _ := json.Marshal(a)
	detail, err := s.QueryAttempt(ctx, a.Records[0].ID)
	if err != nil || len(detail.Evidence) != 1 || detail.Message != errorMessages["network"] {
		t.Fatal("audit detail", err)
	}
	if _, err = s.QueryAttempt(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown audit ID", err)
	}
	if _, err = s.QueryAudit(ctx, AuditQuery{Outcome: "running|success"}); !errors.Is(err, ErrInvalid) {
		t.Fatal("malformed outcome accepted", err)
	}
	if strings.Contains(string(raw), "synthetic/source") || strings.Contains(string(raw), "page\":1") {
		t.Fatal("audit leaked identity/content")
	}
	rangeJob := j
	rangeJob.Policy = "range"
	one, err := s.EnsureRange(ctx, "synthetic-range-1", []Job{rangeJob})
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.EnsureRange(ctx, "synthetic-range-1", []Job{rangeJob})
	if err != nil || same[0].ID != one[0].ID {
		t.Fatal("range not idempotent", err)
	}
	rangeJob.End += halfHour
	if _, err = s.EnsureRange(ctx, "synthetic-range-1", []Job{rangeJob}); !errors.Is(err, ErrConflict) {
		t.Fatal("changed range ID allowed", err)
	}
	rangeJob.End = j.End
	two, err := s.EnsureRange(ctx, "synthetic-range-2", []Job{rangeJob})
	if err != nil || two[0].ID == one[0].ID {
		t.Fatal("new range ID did not repeat", err)
	}
}
func TestMigrationRollbackNewerAndReopen(t *testing.T) {
	ctx := context.Background()
	s, dir, _, _ := fixture(t, "octopus")
	if err := migrate(ctx, s.db, []string{"unused", "unused", "unused", "unused", "unused", "unused", "unused", `CREATE TABLE synthetic_upgrade(value TEXT); INSERT INTO synthetic_upgrade VALUES ('x'); invalid SQL;`}); err == nil {
		t.Fatal("migration error ignored")
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='synthetic_upgrade'`).Scan(&n)
	if n != 0 {
		t.Fatal("migration data did not rollback")
	}
	s.db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&n)
	if n != schemaVersion {
		t.Fatal("migration version advanced")
	}
	if _, err := s.db.Exec(`INSERT INTO schema_migrations VALUES(8)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if reopened, err := Open(ctx, dir); err == nil {
		reopened.Close()
		t.Fatal("newer schema accepted")
	} else if !strings.Contains(err.Error(), "newer") {
		t.Fatal("unactionable newer schema", err)
	}
}
func TestLockDiskFailureAndCancellation(t *testing.T) {
	ctx := context.Background()
	s, dir, _, j := fixture(t, "octopus")
	id := begin(t, s, j)
	other, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// Open performs restart recovery: use a fresh attempt after opening second handle.
	id = begin(t, s, j)
	if _, err := other.db.Exec(`BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = s.UpsertIdentities(ctx, []Identity{{Provider: "octopus", Stream: "export", StableKey: "synthetic/locked", Selected: true}})
	if err == nil || time.Since(started) > 8*time.Second {
		t.Fatal("lock did not fail within busy budget", err, time.Since(started))
	}
	other.db.Exec(`ROLLBACK`)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.QueryIntervals(cancelled, RecordQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatal("query cancellation", err)
	}
	if _, err = s.BeginAttempt(cancelled, AttemptRequest{j.ID, testStart}); !errors.Is(err, context.Canceled) {
		t.Fatal("write cancellation", err)
	}
	// Actual SQLite write denial; retain running audit and no false checkpoint.
	if _, err = s.db.Exec(`PRAGMA query_only=ON`); err != nil {
		t.Fatal(err)
	}
	err = s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: id, Finished: testStart + 4*halfHour, Evidence: []Evidence{{json.RawMessage(`{"history":[]}`), testStart + 4*halfHour, true}}})
	if err == nil {
		t.Fatal("read-only SQLite write succeeded")
	}
	s.db.Exec(`PRAGMA query_only=OFF`)
	if count(t, s, "completions") != 0 {
		t.Fatal("write failure created completion")
	}
	var outcome string
	s.db.QueryRow(`SELECT outcome FROM attempts WHERE id=?`, id).Scan(&outcome)
	if outcome != "running" {
		t.Fatal("write failure created terminal success")
	}
	// Force SQLITE_FULL using a real page-count ceiling, not a mock.
	var pages int
	if err = s.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(fmt.Sprintf(`PRAGMA max_page_count=%d`, pages)); err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"synthetic":"` + strings.Repeat("x", 1024*1024) + `"}`)
	err = s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: testStart + 4*halfHour, Outcome: "failed", ErrorClass: "network", ErrorCode: "incomplete", Evidence: []Evidence{{payload, testStart + 4*halfHour, false}}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "full") {
		t.Fatal("disk-full failure not exercised", err)
	}
	if count(t, s, "evidence") != 0 {
		t.Fatal("disk-full left partial evidence")
	}
	s.db.QueryRow(`SELECT outcome FROM attempts WHERE id=?`, id).Scan(&outcome)
	if outcome != "running" {
		t.Fatal("disk-full recorded false terminal audit")
	}
	t.Log("actual SQLite busy timeout, query-only write denial, SQLITE_FULL and cancellation passed", filepath.Base(dir))
}
func TestAuditIdentityAndSpecificDiagnostic(t *testing.T) {
	s, _, identity, j := fixture(t, "octopus")
	ctx := context.Background()
	identity.Metadata = DiscoveryMetadata{MPAN: "synthetic-mpan", Serial: "synthetic-serial"}
	if _, err := s.UpsertIdentities(ctx, []Identity{identity}); err != nil {
		t.Fatal(err)
	}
	id := begin(t, s, j)
	page := 2
	if err := s.RecordRejectedOrFailedAttempt(ctx, FailedAttempt{AttemptID: id, Finished: testStart + 4*halfHour, NextDue: testStart + 5*halfHour, Outcome: "rejected", ErrorClass: "validation", ErrorCode: "invalid_duration", Page: &page}); err != nil {
		t.Fatal(err)
	}
	a, err := s.QueryAttempt(ctx, id)
	if err != nil || !strings.Contains(a.DisplayIdentity, "synthetic-mpan") || !strings.Contains(a.DisplayIdentity, "synthetic-serial") || strings.Contains(a.DisplayIdentity, "masked") || a.Message != "Reading interval duration is not 30 minutes" || a.Page == nil || *a.Page != 2 {
		t.Fatal(a, err)
	}
}
