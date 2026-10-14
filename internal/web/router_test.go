package web

import (
	"github.com/timmersuk/energything/internal/config"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRoutesAndAuth(t *testing.T) {
	c, err := config.Parse(func(k string) string {
		switch k {
		case "ENERGYTHING_USERNAME":
			return "test"
		case "ENERGYTHING_PASSWORD":
			return "test-password"
		case "OCTOPUS_API_KEY":
			return "private-synthetic"
		}
		return ""
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewRouter(c, "test-build")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, auth string
		status             int
		contains           string
	}{
		{"GET", "/healthz", "", 200, "ok\n"},
		{"GET", "/", "", 401, "Authentication required"},
		{"GET", "/", "wrong", 401, "Authentication required"},
		{"GET", "/static/style.css", "", 401, "Authentication required"},
		{"GET", "/static/style.css", "wrong", 401, "Authentication required"},
		{"GET", "/", "valid", 200, "credentials are incomplete"},
		{"GET", "/static/style.css", "valid", 200, ":root"},
		{"GET", "/api/v1/status", "valid", 404, "not_found"},
		{"GET", "/records/octopus", "valid", 404, "not_found"},
		{"GET", "/missing", "", 404, "not_found"},
		{"POST", "/healthz", "", 405, "method_not_allowed"},
		{"POST", "/", "valid", 405, "method_not_allowed"},
	} {
		t.Run(tc.method+tc.path+tc.auth, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.auth != "" {
				password := "wrong"
				if tc.auth == "valid" {
					password = c.Password
				}
				r.SetBasicAuth(c.Username, password)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if w.Code == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing challenge")
			}
			if strings.Contains(w.Body.String(), "private-synthetic") {
				t.Fatal("secret leaked")
			}
		})
	}
}

func TestEmptyPageAndTemplateEscaping(t *testing.T) {
	c, err := config.Parse(func(k string) string {
		if k == "ENERGYTHING_USERNAME" || k == "ENERGYTHING_PASSWORD" {
			return "test"
		}
		return ""
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewRouter(c, "<script>synthetic</script>")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("test", "test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Count(w.Body.String(), "Unconfigured") != 2 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("template failed to escape build ID")
	}
}
