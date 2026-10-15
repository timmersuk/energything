package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func NewRouter(c config.Config, buildID string, store recordStore) (http.Handler, error) {
	if c.Username == "" || c.Password == "" {
		return nil, errors.New("web credentials are required")
	}
	if store == nil || c.Timezone == nil {
		return nil, errors.New("web storage and timezone are required")
	}
	t, err := templates()
	if err != nil {
		return nil, err
	}
	protected := require(c.Username, c.Password, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			writeError(w, 405, "method_not_allowed", "Method not allowed")
			return
		}
		if r.URL.Path == "/static/style.css" {
			data, _ := files.ReadFile("static/style.css")
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write(data)
			return
		}
		var data any
		var err error
		values, queryErr := url.ParseQuery(r.URL.RawQuery)
		if queryErr != nil {
			writeError(w, 400, "invalid_query", "Invalid query parameters")
			return
		}
		name := "status.html"
		if r.URL.Path == "/" || r.URL.Path == "/api/v1/status" {
			if len(values) != 0 {
				writeError(w, 400, "invalid_query", "Invalid query parameters")
				return
			}
			data, err = statusData(r.Context(), store, c, buildID)
		} else if r.URL.Path == "/records/growatt/aggregates" || r.URL.Path == "/api/v1/growatt/aggregates" {
			data, err = aggregateData(r.Context(), store, values, r.URL.Path)
			name = "aggregates.html"
		} else {
			growatt := strings.Contains(r.URL.Path, "growatt")
			var q recordQuery
			q, err = parseQuery(values, growatt, c.Timezone, time.Now())
			if err != nil {
				writeError(w, 400, "invalid_query", "Invalid range, filter, limit or cursor")
				return
			}
			data, err = recordsData(r.Context(), store, q, growatt, c.Timezone.String(), r.URL.Path)
			name = "octopus.html"
			if growatt {
				name = "growatt.html"
			}
		}
		if err != nil {
			if errors.Is(err, storage.ErrInvalid) {
				writeError(w, 400, "invalid_query", "Invalid range, filter, limit or cursor")
			} else {
				writeError(w, 503, "storage_unavailable", "Stored data temporarily unavailable")
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(data)
			return
		}
		var b bytes.Buffer
		if err := t.ExecuteTemplate(&b, name, data); err != nil {
			writeError(w, 500, "internal", "Page unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b.Bytes())
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		switch r.URL.Path {
		case "/healthz":
			if r.Method != "GET" {
				w.Header().Set("Allow", "GET")
				writeError(w, 405, "method_not_allowed", "Method not allowed")
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("ok\n"))
		case "/records/growatt/aggregates", "/api/v1/growatt/aggregates", "/", "/static/style.css", "/records/octopus", "/records/growatt", "/api/v1/status", "/api/v1/octopus/intervals", "/api/v1/growatt/samples":
			protected.ServeHTTP(w, r)
		default:
			writeError(w, 404, "not_found", "Not found")
		}
	}), nil
}
