package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

func require(username, password string, next http.Handler) http.Handler {
	u, p := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(password))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ru, rp, ok := r.BasicAuth()
		uh, ph := sha256.Sum256([]byte(ru)), sha256.Sum256([]byte(rp))
		a, b := subtle.ConstantTimeCompare(u[:], uh[:]), subtle.ConstantTimeCompare(p[:], ph[:])
		if !ok || a&b != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="energything", charset="UTF-8"`)
			writeError(w, 401, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
