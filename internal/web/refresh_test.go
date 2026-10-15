package web

import (
	"context"
	"github.com/timmersuk/energything/internal/config"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRefreshAuthCSRFOriginMethodAndBody(t *testing.T) {
	db, _ := browsingFixture(t, false)
	calls := 0
	h, e := NewRouterWithRefresh(config.Config{Username: "synthetic", Password: "synthetic", Timezone: time.UTC}, "scheduler", db, func(context.Context, string, string) ([]int64, error) { calls++; return []int64{7}, nil })
	if e != nil {
		t.Fatal(e)
	}
	token := request(h, "/api/v1/status", "synthetic").Header().Get("X-CSRF-Token")
	if len(token) < 32 {
		t.Fatal("missing token")
	}
	cases := []struct {
		method, body, csrf, origin, auth string
		code                             int
	}{
		{"POST", `{"source":"octopus"}`, token, "", "", 401},
		{"GET", "", token, "", "synthetic", 405},
		{"POST", `{"source":"octopus"}`, "", "", "synthetic", 403},
		{"POST", `{"source":"octopus"}`, "foreign", "", "synthetic", 403},
		{"POST", `{"source":"octopus"}`, token, "https://foreign.test", "synthetic", 403},
		{"POST", `{"source":"octopus"}`, token, "null", "synthetic", 403},
		{"POST", `{"source":"bad"}`, token, "", "synthetic", 400},
		{"POST", `{"source":"octopus","range":1}`, token, "", "synthetic", 400},
		{"POST", `{"source":"octopus"} {}`, token, "", "synthetic", 400},
		{"POST", strings.Repeat(" ", 1100) + `{}`, token, "", "synthetic", 400},
		{"POST", `{"source":"octopus"}`, token, "http://example.com", "synthetic", 202},
	}
	for _, tc := range cases {
		before := calls
		r := httptest.NewRequest(tc.method, "http://example.com/api/v1/refresh", strings.NewReader(tc.body))
		if tc.auth != "" {
			r.SetBasicAuth("synthetic", tc.auth)
		}
		r.Header.Set("X-CSRF-Token", tc.csrf)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
		}
		if tc.code != 202 && calls != before {
			t.Fatal("rejected mutation enqueued")
		}
	}
}
