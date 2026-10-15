package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
)

func browsingFixture(t *testing.T, seed bool) (*storage.Store, http.Handler) {
	t.Helper()
	s, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if seed {
		if err = testsupport.SeedBrowsing(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	loc, _ := time.LoadLocation("Europe/London")
	h, err := NewRouter(config.Config{Username: "synthetic", Password: "synthetic", Timezone: loc, Octopus: config.Provider{State: "valid", APIKey: "private-secret"}, Growatt: config.Provider{State: "disabled", Message: "Unconfigured"}}, "task03", s)
	if err != nil {
		t.Fatal(err)
	}
	return s, h
}
func request(h http.Handler, path, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	if auth != "" {
		r.SetBasicAuth("synthetic", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestBrowsingAuthEmptyAndErrors(t *testing.T) {
	s, h := browsingFixture(t, false)
	paths := []string{"/", "/records/octopus", "/records/growatt", "/api/v1/status", "/api/v1/octopus/intervals", "/api/v1/growatt/samples", "/static/style.css", "/static/growatt.js"}
	for _, path := range paths {
		for _, auth := range []string{"", "wrong"} {
			w := request(h, path, auth)
			if w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" || strings.Contains(w.Body.String(), "providers") {
				t.Fatalf("auth %s: %d %s", path, w.Code, w.Body)
			}
		}
		if w := request(h, path, "synthetic"); w.Code != 200 {
			t.Fatalf("empty %s: %d %s", path, w.Code, w.Body)
		}
		r := httptest.NewRequest("POST", path, nil)
		r.SetBasicAuth("synthetic", "synthetic")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 405 {
			t.Fatal(w.Code)
		}
	}
	for _, path := range []string{"/audit", "/api/v1/audit", "/api/v1/refresh"} {
		if request(h, path, "synthetic").Code != 404 {
			t.Fatal("out of scope route")
		}
	}
	for _, query := range []string{"from=2026-03-29", "from=2026-03-30&to=2026-03-29", "from=2026-03-29&to=2026-03-30T00:00:00Z", "from=2026-03-29T00:00:00&to=2026-03-30T00:00:00", "limit=0", "limit=501", "limit=no", "cursor=invalid", "limit=2&limit=3", "identity=-1", "identity=999", "direction=wrong", "unknown=secret", "from=9999-12-31&to=9999-12-31", "bad=%zz", "limit=1;secret=bad"} {
		for _, path := range []string{"/api/v1/octopus/intervals", "/records/octopus"} {
			w := request(h, path+"?"+query, "synthetic")
			if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("invalid %s: %d %s", query, w.Code, w.Body)
			}
		}
	}
	s.Close()
	w := request(h, "/api/v1/status", "synthetic")
	if w.Code != 503 || strings.Contains(w.Body.String(), "sql") {
		t.Fatal(w.Code, w.Body)
	}
}

func TestCalendarBounds(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/London")
	for _, tc := range []struct {
		day   string
		slots int
	}{{"2026-03-29", 46}, {"2026-10-25", 50}} {
		q, err := parseQuery(url.Values{"from": {tc.day}, "to": {tc.day}}, false, loc, time.Now())
		if err != nil || int(*q.To-*q.From)/1800000000 != tc.slots {
			t.Fatal(q, err)
		}
	}
	now := time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC)
	q, err := parseQuery(url.Values{}, false, loc, now)
	if err != nil || q.FromText != "2026-10-25" {
		t.Fatal(q, err)
	}
	for _, query := range []string{"view=wrong", "provider_date=2026-10-25", "view=quality&provider_date=bad", "quality=unresolved", "view=quality&from=2026-10-26"} {
		v, _ := url.ParseQuery(query)
		if _, err := parseQuery(v, true, loc, now); err == nil {
			t.Fatal(query)
		}
	}
	for _, from := range []string{"2026-10-25T00:00:00+24:00", "2026-10-25T00:00:00+00:60", "2026-10-25T00:00:00,123Z", "2026-10-25T00:00:00.1234560001Z"} {
		if _, err := parseQuery(url.Values{"from": {from}, "to": {"2026-10-27T00:00:00Z"}}, false, loc, now); err == nil {
			t.Errorf("accepted malformed or imprecise instant %s", from)
		}
	}
}

func TestStoredValuesPagingAndTimes(t *testing.T) {
	s, h := browsingFixture(t, true)
	base := "/api/v1/octopus/intervals?from=2026-10-25&to=2026-10-25&limit=3"
	seen := map[string]bool{}
	path := base
	for {
		w := request(h, path, "synthetic")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var p struct {
			Records []intervalView `json:"records"`
			Next    string         `json:"next_cursor"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		for _, r := range p.Records {
			key := r.Identity + r.MeasuredAt
			if seen[key] {
				t.Fatal("duplicate")
			}
			seen[key] = true
			if r.KWh != "0.100000000000000001" && r.KWh != "0.000" {
				t.Fatal("decimal changed")
			}
			if r.RetrievedAt == r.MeasuredAt || r.Unit != "kWh" {
				t.Fatal(r)
			}
		}
		if p.Next == "" {
			break
		}
		path = base + "&cursor=" + url.QueryEscape(p.Next)
		if len(seen) == 3 {
			if request(h, path+"&direction=import", "synthetic").Code != 400 {
				t.Fatal("cursor not filter-bound")
			}
			// A new earlier arrival must not shift previously paged rows forward.
			ctx := context.Background()
			ids, err := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "new-earlier-source", Selected: true}})
			if err != nil {
				t.Fatal(err)
			}
			start := storage.At(time.Date(2026, 10, 24, 23, 0, 0, 0, time.UTC))
			jobs, err := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: start, End: start + 1800000000, NextDue: start}})
			if err != nil {
				t.Fatal(err)
			}
			aid, err := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: start + 1800000000})
			if err != nil {
				t.Fatal(err)
			}
			if err = s.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: aid, Finished: start + 1800000000, NextDue: start + 3600000000, Evidence: []storage.Evidence{{Content: json.RawMessage(`{"synthetic":true}`), Retrieved: start + 1800000000, Complete: true}}, Intervals: []storage.Interval{{Start: start, End: start + 1800000000, KWh: "0.100000000000000001"}}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(seen) != 100 {
		t.Fatal("omission", len(seen))
	}
	w := request(h, base+"&direction=export", "synthetic")
	if !strings.Contains(w.Body.String(), `"direction":"export"`) {
		t.Fatal(w.Body)
	}
	w = request(h, "/api/v1/growatt/samples?from=2026-03-29&to=2026-03-29", "synthetic")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"value":null`) || !strings.Contains(w.Body.String(), `"value":"0"`) || !strings.Contains(w.Body.String(), `"unit":null`) || !strings.Contains(w.Body.String(), `"omitted_unresolved":4`) {
		t.Fatal(w.Code, w.Body)
	}
	w = request(h, "/records/growatt?view=quality&provider_date=2026-10-25", "synthetic")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Unresolved") || strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
		t.Fatal(w.Code, w.Body)
	}
	w = request(h, "/api/v1/growatt/samples?view=quality&provider_date=2026-10-25&limit=2", "synthetic")
	var p struct {
		Records []sampleView `json:"records"`
		Next    string       `json:"next_cursor"`
	}
	json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Records) != 2 || p.Next == "" {
		t.Fatal(w.Body)
	}
	w = request(h, "/api/v1/growatt/samples?view=quality&provider_date=2026-10-25&limit=2&cursor="+url.QueryEscape(p.Next), "synthetic")
	p.Next = ""
	json.Unmarshal(w.Body.Bytes(), &p)
	if len(p.Records) != 2 || p.Records[0].MeasuredAt != nil || p.Next != "" {
		t.Fatal(w.Body)
	}
	w = request(h, "/api/v1/status", "synthetic")
	if strings.Contains(w.Body.String(), "synthetic-private") || strings.Contains(w.Body.String(), "private-secret") || !strings.Contains(w.Body.String(), `"running":true`) || strings.Contains(w.Body.String(), `"newest_measured_at":"2026-10-26T12:00:00Z"`) {
		t.Fatal(w.Body)
	}
	var status statusView
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	retired := status.Providers[0].Sources[2]
	if retired.LastAttempt.Outcome != "failed" || retired.LastAccepted.Outcome != "success" {
		t.Fatal("latest failure overwritten by accepted attempt", retired)
	}
	// Browsing leaves durable work and attempts unchanged.
	before, _ := s.QueryAudit(context.Background(), storage.AuditQuery{})
	request(h, "/", "synthetic")
	after, _ := s.QueryAudit(context.Background(), storage.AuditQuery{})
	if before.Counts != after.Counts {
		t.Fatal("read changed attempts")
	}
}

type rejectTransport struct{ calls int }

func (t *rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, errors.New("provider transport must never be called")
}
func TestBrowsingNeverCallsTransport(t *testing.T) {
	transport := &rejectTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	_, h := browsingFixture(t, true)
	for _, path := range []string{"/", "/records/octopus", "/records/growatt", "/api/v1/status", "/api/v1/octopus/intervals", "/api/v1/growatt/samples"} {
		if request(h, path, "synthetic").Code != 200 {
			t.Fatal(path)
		}
	}
	if transport.calls != 0 {
		t.Fatal("outbound calls", transport.calls)
	}
}

type failStore struct{}

func (failStore) QueryStatus(context.Context) ([]storage.Status, error) {
	return nil, errors.New("secret provider body")
}
func (failStore) QueryIntervals(context.Context, storage.RecordQuery) (storage.IntervalPage, error) {
	return storage.IntervalPage{}, errors.New("secret provider body")
}
func (failStore) QuerySamples(context.Context, storage.SampleQuery) (storage.SamplePage, error) {
	return storage.SamplePage{}, errors.New("secret provider body")
}
func TestSafeStorageErrors(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/London")
	h, _ := NewRouter(config.Config{Username: "synthetic", Password: "synthetic", Timezone: loc}, "test", failStore{})
	for _, p := range []string{"/", "/records/octopus", "/records/growatt", "/api/v1/status", "/api/v1/octopus/intervals", "/api/v1/growatt/samples"} {
		w := request(h, p, "synthetic")
		if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w.Code, w.Body)
		}
	}
}

func (failStore) QueryDiscovery(context.Context) ([]storage.DiscoveryState, error) {
	return nil, errors.New("secret")
}
func TestOctopusShowsFullMeterIdentity(t *testing.T) {
	s, h := browsingFixture(t, false)
	ctx := context.Background()
	ids, err := s.UpsertIdentities(ctx, []storage.Identity{{Provider: "octopus", Stream: "import", StableKey: "synthetic-hidden-key", Selected: true, Metadata: storage.DiscoveryMetadata{MPAN: "1234567890123", Serial: "SYNTH-METER"}}})
	if err != nil {
		t.Fatal(err)
	}
	start := storage.At(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	jobs, err := s.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "history", Policy: "backfill", Start: start, End: start + 1800000000, NextDue: storage.At(time.Now())}})
	if err != nil {
		t.Fatal(err)
	}
	began := storage.At(time.Now())
	id, err := s.BeginAttempt(ctx, storage.AttemptRequest{JobID: jobs[0].ID, Started: began})
	if err != nil {
		t.Fatal(err)
	}
	err = s.CommitAcceptedWindow(ctx, storage.AcceptedWindow{AttemptID: id, Finished: storage.At(time.Now()), NextDue: began, Evidence: []storage.Evidence{{Content: json.RawMessage(`{"synthetic":true}`), Retrieved: began, Complete: true}}, Intervals: []storage.Interval{{Start: start, End: start + 1800000000, KWh: "0.100", Evidence: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/api/v1/status", "/records/octopus?from=2026-01-01&to=2026-01-01", "/api/v1/octopus/intervals?from=2026-01-01&to=2026-01-01"} {
		w := request(h, path, "synthetic")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "1234567890123") || !strings.Contains(w.Body.String(), "SYNTH-METER") || strings.Contains(w.Body.String(), "(masked)") || strings.Contains(w.Body.String(), "synthetic-hidden-key") {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if strings.HasPrefix(path, "/api/") && !strings.Contains(w.Body.String(), `"mpan":"1234567890123"`) {
			t.Fatal("explicit MPAN field missing", w.Body.String())
		}
		w = request(h, path, "")
		if w.Code != 401 || strings.Contains(w.Body.String(), "1234567890123") {
			t.Fatal("unauthenticated identity exposed", w.Body.String())
		}
	}
}
func TestGrowattIdentifiersVisibleWithoutStableKey(t *testing.T) {
	_, h := browsingFixture(t, true)
	for _, path := range []string{"/", "/api/v1/status", "/records/growatt?from=2026-03-29&to=2026-03-29", "/api/v1/growatt/samples?from=2026-03-29&to=2026-03-29"} {
		w := request(h, path, "synthetic")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "synthetic-plant") || !strings.Contains(w.Body.String(), "synthetic-inverter") || strings.Contains(w.Body.String(), "(masked)") || strings.Contains(w.Body.String(), "synthetic-private-device") {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if strings.HasPrefix(path, "/api/") && !strings.Contains(w.Body.String(), `"plant_id":"synthetic-plant"`) {
			t.Fatal("plant ID field missing")
		}
		if request(h, path, "").Code != 401 {
			t.Fatal("unauthenticated route")
		}
	}
}
