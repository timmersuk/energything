package octopus

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOldestProbeExplicitDomainBoundedAndValidated(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	calls := 0
	mode := "valid"
	c := NewClient("synthetic", transport(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if q.Get("page_size") != "1" || q.Get("order_by") != "period" || q.Get("period_from") != "0001-01-01T00:00:00Z" {
			t.Fatal("implicit/unbounded probe", q)
		}
		body := `{"count":500000,"next":"https://api.octopus.energy/ignored","results":[{"consumption":0,"interval_start":"2020-01-01T00:00:00Z","interval_end":"2020-01-01T00:30:00Z"}]}`
		if mode == "empty" {
			body = `{"count":0,"next":null,"results":[]}`
		}
		if mode == "invalid" {
			body = strings.Replace(body, `"consumption":0`, `"consumption":-1`, 1)
		}
		if mode == "too_many" {
			body = `{"count":2,"next":null,"results":[]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	m := Meter{MPAN: "SYN-MPAN", Serial: "SYN-SERIAL", Direction: "import"}
	oldest, evidence, e := c.Oldest(context.Background(), m, now)
	if e != nil || oldest == nil || oldest.Year() != 2020 || len(evidence) == 0 || calls != 1 {
		t.Fatal(oldest, e, calls)
	}
	mode = "empty"
	oldest, _, e = c.Oldest(context.Background(), m, now)
	if e != nil || oldest != nil {
		t.Fatal(oldest, e)
	}
	for _, v := range []string{"invalid", "too_many"} {
		mode = v
		if _, _, e = c.Oldest(context.Background(), m, now); e == nil {
			t.Fatal("bad metadata accepted", v)
		}
	}
}
