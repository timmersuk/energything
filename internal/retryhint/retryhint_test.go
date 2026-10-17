package retryhint

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBoundedRetryHints(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, header, body, source, state string
		wait                              time.Duration
	}{
		{"seconds", "120", `{"token":"private-secret"}`, "retry_after_header", "unrecognized", 2 * time.Minute},
		{"date", now.Add(time.Hour).Format(http.TimeFormat), "", "retry_after_header", "absent", time.Hour},
		{"body", "", `{"retry_after":900,"error_code":10012,"message":"private-secret"}`, "retry_after_body", "recognized", 15 * time.Minute},
		{"nested", "bad", `{"data":{"retry_after_seconds":"600"}}`, "retry_after_body", "recognized", 10 * time.Minute},
		{"absolute", "", `{"error":{"retry_at":"2026-10-16T14:00:00+01:00"}}`, "retry_at_body", "recognized", time.Hour},
		{"later body", "120", `{"retry_after_seconds":600}`, "retry_after_body", "recognized", 10 * time.Minute},
		{"later header", "900", `{"retry_after_seconds":600}`, "retry_after_header", "recognized", 15 * time.Minute},
		{"bad fields", "bad", `{"retry_after":-1,"retry_at":"2026-10-16 13:00:00","token":"private-secret"}`, "", "unrecognized", 0},
		{"overflow", "18446744073709551615", `{"retry_after":"9223372036854775807"}`, "", "unrecognized", 0},
		{"no units guessed", "", `{"reset":1792155600,"message":"private-secret"}`, "", "unrecognized", 0},
		{"invalid offset", "", `{"retry_at":"2026-10-17T12:00:00+24:00"}`, "", "unrecognized", 0},
		{"outside storage domain", "", `{"retry_at":"9999-12-31T23:59:59.999999999Z"}`, "", "unrecognized", 0},
		{"html", "120", "<html>private-secret</html>", "retry_after_header", "invalid_json", 2 * time.Minute},
		{"oversize", "120", strings.Repeat("x", MaxBody+1), "retry_after_header", "oversized", 2 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hint := Parse(http.Header{"Retry-After": {tc.header}}, strings.NewReader(tc.body), now)
			if hint.Source != tc.source || hint.BodyStatus != tc.state || (tc.wait > 0 && !hint.NotBefore.Equal(now.Add(tc.wait))) || (tc.wait == 0 && !hint.NotBefore.IsZero()) {
				t.Fatal(hint)
			}
		})
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestBodyReadFailurePreservesHeader(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	h := Parse(http.Header{"Retry-After": {"600"}}, errorReader{}, now)
	if !h.NotBefore.Equal(now.Add(10*time.Minute)) || h.BodyStatus != "read_error" {
		t.Fatal(h)
	}
}
