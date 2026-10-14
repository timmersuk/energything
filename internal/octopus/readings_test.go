package octopus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func stamp(v string) time.Time {
	t, e := time.Parse(time.RFC3339, v)
	if e != nil {
		panic(e)
	}
	return t
}

var from = stamp("2026-01-01T00:00:00Z")
var meter = Meter{"synthetic-mpan", "synthetic-meter", "import"}

func row(t time.Time, value string) string {
	return fmt.Sprintf(`{"consumption":%s,"interval_start":%q,"interval_end":%q,"unknown_telemetry":{"quality":"synthetic"}}`, value, t.Format(time.RFC3339), t.Add(Slot).Format(time.RFC3339))
}
func page(rows []string, next string, count int) string {
	n := "null"
	if next != "" {
		b, _ := json.Marshal(next)
		n = string(b)
	}
	return fmt.Sprintf(`{"count":%d,"results":[%s],"next":%s}`, count, strings.Join(rows, ","), n)
}
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func client(fn transport) *Client {
	c := NewClient("synthetic-secret", fn)
	c.now = func() time.Time { return from.Add(48 * time.Hour) }
	c.jitter = func() time.Duration { return 0 }
	c.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	return c
}
func nextURL(r *http.Request) string {
	u := *r.URL
	q := u.Query()
	q.Set("page", "2")
	u.RawQuery = q.Encode()
	return u.String()
}
func TestPaginationAtomicAndExact(t *testing.T) {
	calls := 0
	c := client(func(r *http.Request) (*http.Response, error) {
		calls++
		key, password, ok := r.BasicAuth()
		if !ok || key != "synthetic-secret" || password != "" {
			t.Fatal("auth")
		}
		if r.URL.Query().Get("period_to") != "2026-01-01T01:59:59Z" {
			t.Fatal("boundary")
		}
		if calls == 1 {
			return response(200, page([]string{row(from.Add(Slot), "1e-18"), row(from, "0.100")}, nextURL(r), 4)), nil
		}
		return response(200, page([]string{row(from, "0.1"), row(from.Add(2*Slot), "0")}, "", 4)), nil
	})
	w, err := c.Fetch(context.Background(), meter, from, from.Add(4*Slot))
	if err != nil || len(w.Intervals) != 3 || w.Duplicates != 1 || w.Missing != 1 || w.Intervals[1].KWh != "0.000000000000000001" || calls != 2 {
		t.Fatal(w, err, calls)
	}
	if !strings.Contains(string(w.Evidence[0].Content), "unknown_telemetry") {
		t.Fatal("evidence lost")
	}
}
func TestHostPathAndQueryBeforeAuth(t *testing.T) {
	for _, change := range []func(*url.URL){func(u *url.URL) { u.Host = "evil.example" }, func(u *url.URL) { u.Scheme = "http" }, func(u *url.URL) { u.User = url.User("secret") }, func(u *url.URL) { u.Path = strings.Replace(u.Path, "synthetic-meter", "other", 1) }, func(u *url.URL) { q := u.Query(); q.Set("period_to", "2027-01-01T00:00:00Z"); u.RawQuery = q.Encode() }, func(u *url.URL) { q := u.Query(); q.Add("period_from", "other"); u.RawQuery = q.Encode() }, func(u *url.URL) { u.Fragment = "x" }, func(u *url.URL) { u.Host = "api.octopus.energy:443" }} {
		calls := 0
		c := client(func(r *http.Request) (*http.Response, error) {
			calls++
			u, _ := url.Parse(nextURL(r))
			change(u)
			return response(200, page([]string{row(from, "1")}, u.String(), 2)), nil
		})
		w, err := c.Fetch(context.Background(), meter, from, from.Add(2*Slot))
		if err == nil || calls != 1 || len(w.Intervals) != 0 {
			t.Fatal("unsafe pagination", err, calls)
		}
	}
	c := client(func(r *http.Request) (*http.Response, error) {
		res := response(302, "")
		res.Header.Set("Location", "https://evil.example")
		return res, nil
	})
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err == nil {
		t.Fatal("redirect accepted")
	}
}
func TestWholeWindowRejectionAndNeighbors(t *testing.T) {
	for _, rows := range [][]string{{row(from, "1"), row(from, "2")}, {row(from, "1"), row(from.Add(Slot), "-1")}, {row(from.Add(15*time.Minute), "1")}, {strings.Replace(row(from, "1"), "00:30:00", "01:00:00", 1)}, {row(from, "null")}, {row(from, `"1"`)}, {row(from, "1e9999")}, {row(from.Add(-15*time.Minute), "1")}} {
		c := client(func(*http.Request) (*http.Response, error) { return response(200, page(rows, "", len(rows))), nil })
		w, err := c.Fetch(context.Background(), meter, from, from.Add(2*Slot))
		var e *Error
		if !errors.As(err, &e) || e.Class != "validation" || len(w.Intervals) != 0 || len(w.Evidence) != 1 {
			t.Fatal(w, err)
		}
	}
	c := client(func(*http.Request) (*http.Response, error) {
		return response(200, page([]string{row(from.Add(-Slot), "1"), row(from, "0"), row(from.Add(Slot), "2")}, "", 3)), nil
	})
	w, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
	if err != nil || w.Outside != 2 || len(w.Intervals) != 1 {
		t.Fatal(w, err)
	}
}
func TestProtocolAndBudgets(t *testing.T) {
	for _, body := range []string{`{"count":0,"next":null}`, `{"count":0,"results":null,"next":null}`, `{"count":1,"results":[],"next":null}`, `{"count":0,"results":[]}`, `{"count":0,"results":[],"next":null} trailing`, strings.Repeat("x", MaxBody+1), page([]string{row(from, "1")}, "", MaxRecords+1)} {
		c := client(func(*http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err == nil {
			t.Fatal("protocol accepted")
		}
	}
	c := client(func(*http.Request) (*http.Response, error) { return response(200, page(nil, "", 0)), nil })
	w, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
	if err != nil || w.Missing != 1 || len(w.Evidence) != 1 {
		t.Fatal(w, err)
	}
	calls := 0
	c = client(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(200, page([]string{row(from, "1")}, nextURL(r), 2)), nil
		}
		return response(200, page([]string{row(from, "1")}, nextURL(r), 2)), nil
	})
	if _, err = c.Fetch(context.Background(), meter, from, from.Add(2*Slot)); err == nil || calls != 2 {
		t.Fatal("repeated page", err, calls)
	}
}
func TestRetryClassificationCancellation(t *testing.T) {
	for _, tc := range []struct {
		status, calls int
		class         string
	}{{429, 3, "rate_limit"}, {500, 3, "network"}, {408, 3, "network"}, {401, 1, "authentication"}, {403, 1, "authentication"}, {400, 1, "protocol"}} {
		calls := 0
		c := client(func(*http.Request) (*http.Response, error) {
			calls++
			res := response(tc.status, "PRIVATE BODY")
			res.Header.Set("Retry-After", "0")
			return res, nil
		})
		w, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
		var e *Error
		if !errors.As(err, &e) || e.Class != tc.class || calls != tc.calls || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal(w, err, calls)
		}
	}
	for _, after := range []string{"31", from.Add(48*time.Hour + time.Minute).Format(http.TimeFormat)} {
		calls := 0
		c := client(func(*http.Request) (*http.Response, error) {
			calls++
			r := response(429, "")
			r.Header.Set("Retry-After", after)
			return r, nil
		})
		_, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
		var e *Error
		if !errors.As(err, &e) || e.DeferredUntil.IsZero() || calls != 1 {
			t.Fatal(err, calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := client(func(r *http.Request) (*http.Response, error) {
		cancel()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	_, err := c.Fetch(ctx, meter, from, from.Add(Slot))
	var e *Error
	if !errors.As(err, &e) || e.Code != "cancelled" {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	c = client(func(*http.Request) (*http.Response, error) { return response(500, ""), nil })
	c.wait = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	if _, err = c.Fetch(ctx, meter, from, from.Add(Slot)); err == nil {
		t.Fatal("cancel wait")
	}
}
func TestLondonCoverageAndPartial(t *testing.T) {
	london, _ := time.LoadLocation("Europe/London")
	for _, tc := range []struct {
		date  string
		slots int
	}{{"2026-03-29", 46}, {"2026-10-25", 50}} {
		start, _ := time.ParseInLocation("2006-01-02", tc.date, london)
		end := start.AddDate(0, 0, 1)
		w := Window{}
		if err := Validate(&w, start, end, end); err != nil || w.Expected != tc.slots || w.Missing != tc.slots {
			t.Fatal(w, err)
		}
	}
	w := Window{}
	if err := Validate(&w, from, from.Add(24*time.Hour), from.Add(75*time.Minute)); err != nil || w.Expected != 2 {
		t.Fatal(w, err)
	}
}
func TestWindowPageRecordAndNetworkBudgets(t *testing.T) {
	calls := 0
	c := client(func(r *http.Request) (*http.Response, error) {
		calls++
		u := *r.URL
		q := u.Query()
		q.Set("page", fmt.Sprint(calls+1))
		u.RawQuery = q.Encode()
		return response(200, page([]string{row(from.Add(time.Duration(calls-1)*Slot), "1")}, u.String(), MaxRecords)), nil
	})
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(MaxWindow)); err == nil || calls != MaxPages {
		t.Fatal("page budget", err, calls)
	}
	rows := make([]string, MaxRecords+1)
	for i := range rows {
		rows[i] = row(from, "1")
	}
	c = client(func(*http.Request) (*http.Response, error) { return response(200, page(rows, "", MaxRecords)), nil })
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err == nil {
		t.Fatal("record budget")
	}
	calls = 0
	c = client(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("PRIVATE URL AND KEY") })
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err == nil || strings.Contains(err.Error(), "PRIVATE") || calls != 3 {
		t.Fatal(err, calls)
	}
	calls = 0
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(MaxWindow+Slot)); err == nil || calls != 0 {
		t.Fatal("window budget")
	}
}
func TestEvidenceSelectionAndAdjacentWindows(t *testing.T) {
	c := client(func(r *http.Request) (*http.Response, error) {
		start, _ := time.Parse(time.RFC3339, r.URL.Query().Get("period_from"))
		body := page([]string{row(start, "1"), row(start.Add(Slot), "2")}, "", 2)
		body = strings.Replace(body, `"unknown_telemetry":`, `"password":"private","account":"private","headers":{"secret":"private"},"unknown_telemetry":`, 1)
		return response(200, body), nil
	})
	for _, start := range []time.Time{from, from.Add(Slot)} {
		w, err := c.Fetch(context.Background(), meter, start, start.Add(Slot))
		if err != nil || len(w.Intervals) != 1 || !w.Intervals[0].Start.Equal(start) || w.Outside != 1 {
			t.Fatal(w, err)
		}
		b := string(w.Evidence[0].Content)
		if strings.Contains(b, "private") || !strings.Contains(b, "unknown_telemetry") || !strings.Contains(b, `"boundary_neighbors_excluded":1`) {
			t.Fatal("evidence selection", b)
		}
	}
}
func TestAlignedFractionalTimestampAndCancelledBody(t *testing.T) {
	c := client(func(*http.Request) (*http.Response, error) {
		body := strings.ReplaceAll(page([]string{row(from, "1")}, "", 1), ":00Z", ":00.000000Z")
		return response(200, body), nil
	})
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c = client(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: cancelBody{cancel}}, nil
	})
	_, err := c.Fetch(ctx, meter, from, from.Add(Slot))
	var e *Error
	if !errors.As(err, &e) || e.Code != "cancelled" {
		t.Fatal(err)
	}
}

type cancelBody struct{ cancel context.CancelFunc }

func (b cancelBody) Read([]byte) (int, error) { b.cancel(); return 0, context.Canceled }
func (cancelBody) Close() error               { return nil }
func TestNullCountAndCanonicalRepeatedPages(t *testing.T) {
	c := client(func(*http.Request) (*http.Response, error) {
		return response(200, `{"count":null,"results":[],"next":null}`), nil
	})
	if _, err := c.Fetch(context.Background(), meter, from, from.Add(Slot)); err == nil {
		t.Fatal("null count accepted")
	}
	calls := 0
	c = client(func(r *http.Request) (*http.Response, error) {
		calls++
		rows := []string{row(from, "1"), row(from.Add(Slot), "2")}
		if calls == 2 {
			rows[0], rows[1] = rows[1], rows[0]
		}
		body := page(rows, "", 4)
		if calls == 1 {
			body = page(rows, nextURL(r), 4)
		} else {
			var v any
			json.Unmarshal([]byte(body), &v)
			b, _ := json.MarshalIndent(v, "", " ")
			body = string(b)
		}
		return response(200, body), nil
	})
	_, err := c.Fetch(context.Background(), meter, from, from.Add(2*Slot))
	var e *Error
	if !errors.As(err, &e) || e.Class != "pagination" || calls != 2 {
		t.Fatal(err, calls)
	}
}

type bodyTimeout struct{}

func (bodyTimeout) Error() string            { return "PRIVATE TIMEOUT" }
func (bodyTimeout) Timeout() bool            { return true }
func (bodyTimeout) Read([]byte) (int, error) { return 0, bodyTimeout{} }
func (bodyTimeout) Close() error             { return nil }
func TestBodyTimeoutUsesOneRetryBudget(t *testing.T) {
	for _, succeed := range []bool{true, false} {
		calls := 0
		c := client(func(*http.Request) (*http.Response, error) {
			calls++
			if succeed && calls == 2 {
				return response(200, page(nil, "", 0)), nil
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: bodyTimeout{}}, nil
		})
		w, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
		if succeed {
			if err != nil || calls != 2 || len(w.Retries) != 1 || w.Retries[0].Code != "timeout" {
				t.Fatal(w, err, calls)
			}
		} else {
			var e *Error
			if !errors.As(err, &e) || e.Code != "timeout" || calls != 3 || len(w.Evidence) != 0 {
				t.Fatal(w, err, calls)
			}
		}
	}
}
func TestValidationReasonsRemainSpecific(t *testing.T) {
	cases := []struct {
		rows []string
		code string
	}{{[]string{row(from, "-1")}, "invalid_decimal"}, {[]string{strings.Replace(row(from, "1"), "00:30:00", "01:00:00", 1)}, "invalid_duration"}, {[]string{row(from.Add(15*time.Minute), "1")}, "unaligned_interval"}, {[]string{row(from, "1"), row(from, "2")}, "conflicting_interval"}}
	for _, tc := range cases {
		c := client(func(*http.Request) (*http.Response, error) {
			return response(200, page(tc.rows, "", len(tc.rows))), nil
		})
		_, err := c.Fetch(context.Background(), meter, from, from.Add(2*Slot))
		var e *Error
		if !errors.As(err, &e) || e.Code != tc.code {
			t.Fatal(err, tc.code)
		}
	}
}
func TestCountFailuresHaveSpecificReasons(t *testing.T) {
	for _, tc := range []struct{ body, code string }{{`{"results":[],"next":null}`, "missing_count"}, {`{"count":null,"results":[],"next":null}`, "invalid_count"}, {`{"count":2049,"results":[],"next":null}`, "count_budget"}} {
		c := client(func(*http.Request) (*http.Response, error) { return response(200, tc.body), nil })
		_, err := c.Fetch(context.Background(), meter, from, from.Add(Slot))
		var e *Error
		if !errors.As(err, &e) || e.Code != tc.code {
			t.Fatal(err, tc.code)
		}
	}
	calls := 0
	c := client(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(200, page([]string{row(from, "1")}, nextURL(r), 2)), nil
		}
		return response(200, page([]string{row(from.Add(Slot), "1")}, "", 3)), nil
	})
	_, err := c.Fetch(context.Background(), meter, from, from.Add(2*Slot))
	var e *Error
	if !errors.As(err, &e) || e.Code != "count_changed" {
		t.Fatal(err)
	}
}
