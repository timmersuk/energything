package storage

import (
	"context"
	"encoding/json"
	"testing"
)

func TestProviderDaysNewestFirstIndependentOfRetrieval(t *testing.T) {
	s, _, _, job := fixture(t, "growatt")
	ctx := context.Background()
	samples := []Sample{}
	for _, clock := range []string{`"2026-10-01 01:00:00"`, `"2026-10-01 23:59:00"`, `"2026-10-01 23:58:00"`, `"2026-10-01 12:00:00"`, `"2026-10-01 00:01:00"`} {
		samples = append(samples, Sample{ProviderDate: "2026-10-01", OriginalTime: map[string]string{"time": clock}, Quality: "unresolved", Values: map[string]Value{}})
	}
	if e := s.CommitAcceptedWindow(ctx, AcceptedWindow{AttemptID: begin(t, s, job), Finished: testStart + 4*halfHour, NextDue: testStart + 5*halfHour, Evidence: []Evidence{{Content: json.RawMessage(`{"synthetic":true}`), Retrieved: testStart + 4*halfHour, Complete: true}}, Samples: samples, Coverage: Coverage{Valid: len(samples), Qualified: len(samples)}}); e != nil {
		t.Fatal(e)
	}
	q := SampleQuery{RecordQuery: RecordQuery{Limit: 1}, ProviderView: true, ProviderFrom: "2026-10-01", ProviderTo: "2026-10-01"}
	seen := map[int64]bool{}
	for _, want := range []int64{2, 3, 4, 1, 5} {
		p, e := s.QuerySamples(ctx, q)
		if e != nil || len(p.Records) != 1 || p.Records[0].ID != want || seen[want] || p.Records[0].Instant != nil {
			t.Fatal(want, p, e)
		}
		seen[want] = true
		if q.Cursor == "" {
			bad := q
			bad.Cursor = p.NextCursor
			bad.ProviderTo = "2026-10-02"
			if _, e := s.QuerySamples(ctx, bad); e != ErrInvalid {
				t.Fatal("cursor filter mismatch", e)
			}
		}
		q.Cursor = p.NextCursor
	}
	if q.Cursor != "" {
		t.Fatal("extra page")
	}
}
