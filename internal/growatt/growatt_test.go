package growatt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(s string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(s))}
}
func decode(s string) map[string]any {
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	var v map[string]any
	_ = d.Decode(&v)
	return v
}
func TestDestinationsAndRedirect(t *testing.T) {
	for _, raw := range []string{"http://openapi.growatt.com/v1/plant/list", "https://evil.test/v1/plant/list", "https://openapi.growatt.com:443/v1/plant/list", "https://openapi.growatt.com/v1/device/mix/mix_data_info", "https://openapi.growatt.com/v1/plant/list#x", "https://openapi.growatt.com/v1/plant/list?q=x"} {
		if destination(raw, "GET", url.Values{"page": {"1"}, "perpage": {"100"}}) {
			t.Fatalf("allowed unsafe destination %s", raw)
		}
	}
	calls := 0
	c := NewClient("synthetic-token", roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "openapi.growatt.com" || r.Header.Get("token") != "synthetic-token" {
			t.Fatal("destination/auth")
		}
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://evil.test/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	_, e := c.Discover(context.Background(), "", "")
	if e == nil || calls != 1 {
		t.Fatal("redirect followed")
	}
}
func TestDiscoverySelection(t *testing.T) {
	c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "plant/list") {
			return reply(`{"error_code":0,"data":{"count":1,"plants":[{"plant_id":123}]}}`), nil
		}
		return reply(`{"error_code":0,"data":{"count":3,"devices":[{"device_sn":"SYN-A","type":5},{"device_sn":"SYN-B","type":"5"},{"device_sn":"SYN-C","type":2}]}}`), nil
	}))
	d, e := c.Discover(context.Background(), "", "")
	if e != nil || d.State != "selection_required" {
		t.Fatal(d, e)
	}
	d, e = c.Discover(context.Background(), "123", "SYN-B")
	if e != nil || d.Selected == nil || d.Selected.Serial != "SYN-B" {
		t.Fatal(d, e)
	}
	d, e = c.Discover(context.Background(), "123", "SYN-C")
	if e != nil || d.Selected != nil {
		t.Fatal(d, e)
	}
}
func TestNormalizationExactOptionalAndTime(t *testing.T) {
	row := decode(`{"serialNum":"SYN-A","time":"2026-10-25 01:30:00","soc":0,"bmsSOC":7,"ppv":1.234567890123456789e2,"pcharge1":-1,"unknown":{"sensor":99}}`)
	s, e := normalize(row, "2026-10-25", "SYN-A", 0)
	if e != nil {
		t.Fatal(e)
	}
	if s.Instant != nil || s.ResolvedKey != nil || s.Quality != "unresolved_time" {
		t.Fatal("guessed identity/time")
	}
	if *s.Values["soc"].Decimal != "0" || s.Values["bmsSOC"].Unit != nil || *s.Values["ppv"].Decimal != "123.4567890123456789" || s.Values["pdischarge1"].Decimal != nil {
		t.Fatal(s.Values)
	}
	for _, date := range []string{"2026-03-29 01:30:00", "2026-10-25 01:30:00"} {
		x, e := normalize(decode(`{"time":"`+date+`"}`), "", "SYN-A", 0)
		if e != nil || x.Instant != nil {
			t.Fatal("DST clock guessed")
		}
	}
	for _, bad := range []string{`{"soc":101}`, `{"bmsSOC":-1}`, `{"ppv":-1}`, `{"ppv":2,"ppvText":"3 W"}`, `{"soc":"NaN"}`, `{"time":"2026-01-01 00:00:00"}`, `{"serialNum":"OTHER"}`} {
		if _, e := normalize(decode(bad), "2026-10-25", "SYN-A", 0); e == nil {
			t.Fatal("accepted invalid unit", bad)
		}
	}
}
func TestWholeDayAssemblyAndFailures(t *testing.T) {
	for _, scenario := range []string{"valid", "count", "early", "repeat", "invalid", "provider", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if e := r.ParseForm(); e != nil {
					t.Fatal(e)
				}
				if r.Form.Get("start_date") != r.Form.Get("end_date") {
					t.Fatal("unbounded")
				}
				if calls == 1 {
					rows := make([]map[string]any, 100)
					for i := range rows {
						rows[i] = map[string]any{"time": "2026-10-25 01:30:00", "soc": i % 100, "unknown": i}
					}
					b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": 101, "datas": rows}})
					return reply(string(b)), nil
				}
				switch scenario {
				case "count":
					return reply(`{"error_code":0,"data":{"count":102,"datas":[]}}`), nil
				case "early":
					return reply(`{"error_code":0,"data":{"count":101,"datas":[]}}`), nil
				case "repeat":
					calls = 0
					rows := make([]map[string]any, 100)
					for i := range rows {
						rows[i] = map[string]any{"time": "2026-10-25 01:30:00", "soc": i % 100, "unknown": i}
					}
					b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": 101, "datas": rows}})
					return reply(string(b)), nil
				case "invalid":
					return reply(`{"error_code":0,"data":{"count":101,"datas":[{"soc":101}]}}`), nil
				case "provider":
					return reply(`{"error_code":10011,"error_msg":"synthetic-secret"}`), nil
				case "missing":
					return reply(`{"error_code":0,"data":{"count":101}}`), nil
				}
				return reply(`{"error_code":0,"data":{"count":101,"datas":[{"soc":0,"time":"2026-10-25 01:30:00","calendar":{"timeInMillis":123}}]}}`), nil
			}))
			w, e := c.FetchDay(context.Background(), Device{"123", "SYN-A", "5"}, "2026-10-25", "Europe/London")
			if scenario == "valid" {
				if e != nil || len(w.Samples) != 101 || !w.Evidence[0].Complete || w.Samples[100].Instant != nil {
					t.Fatal(len(w.Samples), e)
				}
			} else {
				if e == nil || len(w.Evidence) == 0 {
					t.Fatal("failed unit accepted")
				}
			}
		})
	}
}
func TestRetryOwnershipAndCancellation(t *testing.T) {
	calls := 0
	c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private-error"))}, nil
	}))
	c.wait = func(context.Context, time.Duration) error { return nil }
	c.jitter = func() time.Duration { return 0 }
	w, e := c.FetchLatest(context.Background(), Device{"123", "SYN-A", "5"})
	if e == nil || calls != 3 || len(w.Retries) != 3 {
		t.Fatal(calls, w, e)
	}
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"120"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	w, e = c.FetchLatest(context.Background(), Device{"123", "SYN-A", "5"})
	if e.(*Error).DeferredUntil.IsZero() || len(w.Retries) != 1 {
		t.Fatal("long wait not deferred")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = c.FetchLatest(ctx, Device{"123", "SYN-A", "5"})
	if e.(*Error).Code != "cancelled" {
		t.Fatal(e)
	}
}
func TestEvidencePrivacyAndEmpty(t *testing.T) {
	c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
		return reply(`{"error_code":0,"data":{"soc":0,"token":"secret","address":7,"unknown":{"voltage":1,"headers":{"token":"secret"}}}}`), nil
	}))
	w, e := c.FetchLatest(context.Background(), Device{"123", "SYN-A", "5"})
	if e != nil || strings.Contains(string(w.Evidence[0].Content), "secret") || !strings.Contains(string(w.Evidence[0].Content), "voltage") {
		t.Fatal(w, e)
	}
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return reply(`{"error_code":0,"data":{"count":0,"datas":[]}}`), nil
	})
	w, e = c.FetchDay(context.Background(), Device{"123", "SYN-A", "5"}, "2026-01-01", "Europe/London")
	if e != nil || len(w.Samples) != 0 || !w.Evidence[0].Complete {
		t.Fatal(w, e)
	}
}

func TestBudgetsAndActiveCancellation(t *testing.T) {
	for _, body := range []string{`{"error_code":0,"data":{"count":10001,"datas":[]}}`, `{"error_code":0,"data":{"datas":[]}}`, strings.Repeat(" ", MaxBody+1)} {
		c := NewClient("synthetic", roundTrip(func(*http.Request) (*http.Response, error) { return reply(body), nil }))
		if _, e := c.FetchDay(context.Background(), Device{"123", "SYN-A", "5"}, "2026-01-01", "Europe/London"); e == nil {
			t.Fatal("budget/missing structure accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	}))
	done := make(chan error, 1)
	go func() { _, e := c.FetchLatest(ctx, Device{"123", "SYN-A", "5"}); done <- e }()
	<-entered
	cancel()
	select {
	case e := <-done:
		if e.(*Error).Code != "cancelled" {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
}
