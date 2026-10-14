package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/timmersuk/energything/internal/config"
	"net/http"
)

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func NewRouter(c config.Config, buildID string) (http.Handler, error) {
	if c.Username == "" || c.Password == "" {
		return nil, errors.New("web credentials are required")
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
		var b bytes.Buffer
		data := struct {
			BuildID          string
			Octopus, Growatt config.Provider
		}{buildID, c.Octopus, c.Growatt}
		if err := t.ExecuteTemplate(&b, "index.html", data); err != nil {
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
		case "/", "/static/style.css":
			protected.ServeHTTP(w, r)
		default:
			writeError(w, 404, "not_found", "Not found")
		}
	}), nil
}
