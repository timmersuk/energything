package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAuditStorageBackedRoutes(t *testing.T) {
	s, h := browsingFixture(t, false)
	if e := testsupport.SeedAudit(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	get := func(path string) auditView {
		t.Helper()
		w := request(h, path, "synthetic")
		var v auditView
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cache")
		}
		return v
	}
	all := get("/api/v1/audit?limit=2")
	if all.Counts.Total != 9 || all.Counts.Failed != 4 || all.Counts.Rejected != 1 {
		t.Fatal(all.Counts)
	}
	seen := map[int64]bool{}
	last := int64(9999)
	cursor := ""
	for {
		v := get("/api/v1/audit?limit=2&cursor=" + url.QueryEscape(cursor))
		if v.Counts != all.Counts {
			t.Fatal("page-local counts")
		}
		for _, a := range v.Records {
			if seen[a.ID] || a.ID >= last {
				t.Fatal("unstable paging")
			}
			seen[a.ID] = true
			last = a.ID
		}
		cursor = v.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 9 {
		t.Fatal(seen)
	}
	repeated := get("/api/v1/audit?source=octopus&stream=import&outcome=failed&error_class=network&limit=1")
	if repeated.Counts.Total != 3 || repeated.Counts.Failed != 3 {
		t.Fatal(repeated.Counts)
	}
	if get("/api/v1/audit?outcome=empty").Counts.Total != 1 || get("/api/v1/audit?outcome=success").Counts.Total != 2 {
		t.Fatal("empty/recovery lost")
	}
	for _, query := range []string{"source=invalid", "stream=unknown", "outcome=deferred", "error_class=no", "identity=999", "limit=", "limit=501", "limit=1&limit=2", "unknown=secret", "from=2026-10-25", "from=2026-10-26&to=2026-10-25", "cursor=bad", "source=growatt&limit=2&cursor=" + url.QueryEscape(all.NextCursor)} {
		for _, path := range []string{"/audit?", "/api/v1/audit?"} {
			w := request(h, path+query, "synthetic")
			if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
				t.Fatal(query, w.Code, w.Body)
			}
		}
	}
	rejected := get("/api/v1/audit?outcome=rejected").Records[0]
	detail := get(fmt.Sprintf("/api/v1/audit/%d", rejected.ID)).Records[0]
	if detail.ProviderIdentity.MPAN != "SYN-MPAN" || detail.ProviderIdentity.Serial != "SYN-METER" || !strings.Contains(detail.Message, "30 minutes") || len(detail.Evidence) != 1 {
		t.Fatal(detail)
	}
	evidence := detail.Evidence[0].URL
	for _, path := range []string{"/audit", "/api/v1/audit", detail.DetailURL, fmt.Sprintf("/api/v1/audit/%d", detail.ID), evidence, evidence + "?download=1", "/static/audit.js"} {
		for _, auth := range []string{"", "wrong"} {
			if w := request(h, path, auth); w.Code != 401 {
				t.Fatal(path, w.Code)
			}
		}
		if w := request(h, path, "synthetic"); w.Code != 200 {
			t.Fatal(path, w.Code, w.Body)
		}
		r := httptest.NewRequest("POST", path, nil)
		r.SetBasicAuth("synthetic", "synthetic")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 405 {
			t.Fatal(path, w.Code)
		}
	}
	w := request(h, evidence, "synthetic")
	if !strings.Contains(w.Body.String(), "unknown_telemetry") || strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "private-secret") {
		t.Fatal(w.Body)
	}
	if request(h, evidence+"?download=1", "synthetic").Header().Get("Content-Disposition") == "" {
		t.Fatal("no attachment")
	}
	other := get("/api/v1/audit?outcome=success").Records[0]
	for _, path := range []string{fmt.Sprintf("/api/v1/audit/%d/evidence/%d", other.ID, detail.Evidence[0].RetrievalID), "/api/v1/audit/999", "/api/v1/audit/1/evidence/999", "/audit/1/2", "/api/v1/audit/01"} {
		if w := request(h, path, "synthetic"); w.Code != 404 {
			t.Fatal(path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/audit?limit=2", "/audit?outcome=rejected", detail.DetailURL} {
		if w := request(h, path, "synthetic"); w.Code != 200 || strings.Contains(w.Body.String(), "<script>window.auditInjected") {
			t.Fatal(path, w.Code, w.Body)
		}
	}
	// All reads leave accepted data, attempts and the live job state intact.
	intervals, e := s.QueryIntervals(context.Background(), storage.RecordQuery{})
	if e != nil || len(intervals.Records) != 1 {
		t.Fatal(intervals, e)
	}
	if get("/api/v1/audit").Counts != all.Counts {
		t.Fatal("browsing changed attempts")
	}
}

func TestAuditRestartInterruptedAndReadOnly(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = testsupport.SeedBrowsing(ctx, s); e != nil {
		t.Fatal(e)
	}
	loc, _ := time.LoadLocation("Europe/London")
	router := func() http.Handler {
		h, e := NewRouter(config.Config{Username: "synthetic", Password: "synthetic", Timezone: loc}, "audit-test", s)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	before := request(router(), "/api/v1/audit?outcome=running", "synthetic")
	if !strings.Contains(before.Body.String(), `"outcome":"running"`) {
		t.Fatal(before.Body)
	}
	s.Close()
	s, e = storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w := request(router(), "/api/v1/audit?outcome=interrupted", "synthetic")
	var v auditView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Records) != 1 || v.Records[0].Outcome != "interrupted" {
		t.Fatal(w.Code, w.Body)
	}
}

func TestGenerationUnfinishedProgress(t *testing.T) {
	s, h := browsingFixture(t, false)
	ctx := context.Background()
	if e := testsupport.SeedAggregates(ctx, s); e != nil {
		t.Fatal(e)
	}
	if e := testsupport.SeedGenerationProgress(ctx, s); e != nil {
		t.Fatal(e)
	}
	w := request(h, "/api/v1/growatt/aggregates?plant_id=SYN-PLANT-A&from=2026-01-15&to=2026-01-15", "synthetic")
	var v aggregatesView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Progress) != 1 || len(v.Progress[0].Jobs) != 1 {
		t.Fatal(w.Code, w.Body)
	}
	p := v.Progress[0]
	if p.Frontier != "2026-08-15T00:00:00Z" || p.SearchLimit != "2022-01-01T00:00:00Z" || p.LastAttempt.ErrorCode != "local_budget" || p.LastAttempt.NextRetry == nil || p.Jobs[0].Retries != 1 {
		t.Fatal(p)
	}
	html := request(h, "/records/growatt/aggregates?plant_id=SYN-PLANT-A", "synthetic")
	if html.Code != 200 || !strings.Contains(html.Body.String(), "not fetched yet") || !strings.Contains(html.Body.String(), "2026-08-15") || !strings.Contains(html.Body.String(), "Local aggregate request budget") {
		t.Fatal(html.Code, html.Body)
	}
}
