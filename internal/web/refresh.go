package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type Refresh func(context.Context, string, string) ([]int64, error)

func refreshHandler(refresh Refresh, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			writeError(w, 405, "method_not_allowed", "Method not allowed")
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(token)) != 1 {
			writeError(w, 403, "csrf", "Reload the authenticated status page for a CSRF token")
			return
		}
		if origins, ok := r.Header["Origin"]; ok {
			if len(origins) != 1 {
				writeError(w, 403, "origin", "Invalid origin")
				return
			}
			u, e := url.Parse(origins[0])
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if e != nil || u.Scheme != scheme || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				writeError(w, 403, "origin", "Invalid origin")
				return
			}
		}
		if r.URL.RawQuery != "" {
			writeError(w, 400, "invalid_request", "No query parameters are supported")
			return
		}
		var in struct {
			Source string `json:"source"`
			Stream string `json:"stream"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || (in.Source != "octopus" && in.Source != "growatt") || (in.Source == "octopus" && in.Stream != "" && in.Stream != "import" && in.Stream != "export") || (in.Source == "growatt" && in.Stream != "" && in.Stream != "telemetry" && in.Stream != "generation") {
			writeError(w, 400, "invalid_request", "Select a configured source and stream")
			return
		}
		if refresh == nil {
			writeError(w, 503, "unavailable", "Collection is unavailable")
			return
		}
		ids, e := refresh(r.Context(), in.Source, in.Stream)
		if e != nil {
			writeError(w, 503, "unavailable", "Selected collection work is unavailable; inspect source status")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_ = json.NewEncoder(w).Encode(map[string]any{"job_ids": ids})
	}
}

func csrfToken() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	mac := hmac.New(sha256.New, b)
	_, _ = mac.Write([]byte("energything/api/v1/refresh"))
	return hex.EncodeToString(mac.Sum(nil)), e
}
