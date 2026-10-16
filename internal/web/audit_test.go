package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"io"
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

type auditTransport func(*http.Request) (*http.Response, error)

func (f auditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOctopusCountDiagnosticPersistsThroughAuditAPI(t *testing.T) {
	for _, mode := range []string{"count_changed", "count_mismatch"} {
		t.Run(mode, func(t *testing.T) {
			s, h := browsingFixture(t, false)
			ctx := context.Background()
			ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "synthetic-count", Selected: true, Metadata: storage.DiscoveryMetadata{MPAN: "SYN-MPAN", Serial: "SYN-METER"}}})
			if e != nil {
				t.Fatal(e)
			}
			from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(from.Add(time.Hour)), NextDue: storage.At(time.Now())}})
			if e != nil {
				t.Fatal(e)
			}
			client := octopus.NewClient("synthetic-key", auditTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"count":2,"next":null,"results":[]}`
				if mode == "count_changed" {
					next := *r.URL
					q := next.Query()
					q.Set("page", "2")
					next.RawQuery = q.Encode()
					body = fmt.Sprintf(`{"count":2,"next":%q,"results":[{"consumption":1,"interval_start":"2026-01-01T00:00:00Z","interval_end":"2026-01-01T00:30:00Z"}]}`, next.String())
					if r.URL.Query().Get("page") == "2" {
						body = `{"count":1,"next":null,"results":[]}`
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			if e = collection.CollectOctopusWindow(ctx, s, client, jobs[0], octopus.Meter{MPAN: "SYN-MPAN", Serial: "SYN-METER", Direction: "import"}); e == nil {
				t.Fatal("count error accepted")
			}
			w := request(h, "/api/v1/audit", "synthetic")
			var v auditView
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Records) != 1 {
				t.Fatal(w.Code, w.Body)
			}
			a := v.Records[0]
			observed := 0
			if mode == "count_changed" {
				observed = 1
			}
			if a.ErrorCode != mode || a.Diagnostic.Expected == nil || *a.Diagnostic.Expected != 2 || a.Diagnostic.Observed == nil || *a.Diagnostic.Observed != observed || len(a.Evidence) == 0 || a.Page == nil {
				t.Fatal(a)
			}
		})
	}
}

func TestGrowingOctopusWindowReachesRecordsAndAudit(t *testing.T) {
	s, h := browsingFixture(t, false)
	ctx := context.Background()
	ids, err := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "synthetic-growth", Selected: true, Metadata: storage.DiscoveryMetadata{MPAN: "SYN-MPAN", Serial: "SYN-METER"}}})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	jobs, err := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(from.Add(90 * time.Minute)), NextDue: storage.At(time.Now())}})
	if err != nil {
		t.Fatal(err)
	}
	client := octopus.NewClient("synthetic-key", auditTransport(func(r *http.Request) (*http.Response, error) {
		interval := func(n int) string {
			return fmt.Sprintf(`{"consumption":1,"interval_start":%q,"interval_end":%q}`, from.Add(time.Duration(n)*30*time.Minute).Format(time.RFC3339), from.Add(time.Duration(n+1)*30*time.Minute).Format(time.RFC3339))
		}
		next := *r.URL
		q := next.Query()
		q.Set("page", "2")
		next.RawQuery = q.Encode()
		body := fmt.Sprintf(`{"count":2,"next":%q,"results":[%s]}`, next.String(), interval(0))
		if r.URL.Query().Get("page") == "2" {
			body = fmt.Sprintf(`{"count":3,"next":null,"results":[%s,%s]}`, interval(1), interval(2))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	if err = collection.CollectOctopusWindow(ctx, s, client, jobs[0], octopus.Meter{MPAN: "SYN-MPAN", Serial: "SYN-METER", Direction: "import"}); err != nil {
		t.Fatal(err)
	}
	w := request(h, "/api/v1/audit", "synthetic")
	var audit auditView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &audit) != nil || len(audit.Records) != 1 || audit.Records[0].Outcome != "success" || audit.Records[0].AcceptedRows != 3 || len(audit.Records[0].Evidence) != 2 {
		t.Fatal(w.Code, w.Body)
	}
	w = request(h, "/api/v1/octopus/intervals?from=2026-01-01&to=2026-01-01", "synthetic")
	var records struct {
		Records []json.RawMessage `json:"records"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &records) != nil || len(records.Records) != 3 {
		t.Fatal(w.Code, w.Body)
	}
}

func Test429HintsPersistWithoutResponseSecrets(t *testing.T) {
	s, h := browsingFixture(t, false)
	ctx := context.Background()
	ids, e := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: "SYN-HINT", Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: "SYN-HINT"}}})
	if e != nil {
		t.Fatal(e)
	}
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	jobs, e := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(from), End: storage.At(from.AddDate(0, 0, 1)), ProviderDate: "2026-01-15", NextDue: storage.At(time.Now())}})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	client := growatt.NewClient("synthetic", auditTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"60"}, "Set-Cookie": {"private-secret"}}, Body: io.NopCloser(strings.NewReader(`{"error_code":10012,"retry_after_seconds":900,"token":"private-secret","message":"private-secret"}`))}, nil
	}))
	if e = collection.CollectGrowattAggregateUnit(ctx, s, client, jobs[0], "SYN-HINT"); e == nil || calls != 1 {
		t.Fatal(e, calls)
	}
	w := request(h, "/api/v1/audit?error_class=rate_limit", "synthetic")
	var v auditView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Records) != 1 {
		t.Fatal(w.Code, w.Body)
	}
	a := v.Records[0]
	if a.Diagnostic.HTTPStatus != 429 || a.Diagnostic.RetryHintSource != "retry_after_body" || a.Diagnostic.RateLimitBody != "recognized" || a.RetryHintAt == nil || a.NextRetry == nil || len(a.Evidence) != 0 || a.Diagnostic.ProviderCode == nil || *a.Diagnostic.ProviderCode != 10012 {
		t.Fatal(a)
	}
	hint, e := time.Parse(time.RFC3339Nano, *a.RetryHintAt)
	if e != nil {
		t.Fatal(e)
	}
	next, e := time.Parse(time.RFC3339Nano, *a.NextRetry)
	if e != nil || next.Before(hint) {
		t.Fatal("provider deadline shortened", e)
	}
	if strings.Contains(w.Body.String(), "private-secret") || strings.Contains(w.Body.String(), "Set-Cookie") {
		t.Fatal("raw response leaked")
	}
	html := request(h, a.DetailURL, "synthetic")
	if html.Code != 200 || !strings.Contains(html.Body.String(), "retry_after_body") || !strings.Contains(html.Body.String(), "Provider retry instruction UTC") {
		t.Fatal(html.Code, html.Body)
	}

}
