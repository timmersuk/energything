// Package retryhint selects bounded, explicit provider retry instructions without
// retaining raw error responses or guessing units for reset headers/free text.
package retryhint

import (
	"bytes"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxBody = 64 * 1024

var offsetTime = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

type Hint struct {
	NotBefore    time.Time
	Source       string
	BodyStatus   string
	ProviderCode *int
}

func (h Hint) Diagnostic(status int) storage.Diagnostic {
	d := storage.Diagnostic{HTTPStatus: status, ProviderCode: h.ProviderCode, RetryHintSource: h.Source, RateLimitBody: h.BodyStatus}
	if !h.NotBefore.IsZero() {
		at := storage.At(h.NotBefore)
		d.RetryHintAt = &at
	}
	return d
}

// Parse examines Retry-After and a small JSON allowlist. Callers own Body.Close.
// Numeric retry_after/retry_after_seconds are seconds; retry_at requires an offset
// RFC3339 instant. All valid instructions are lower bounds: the latest wins.
func Parse(header http.Header, body io.Reader, now time.Time) Hint {
	h := Hint{BodyStatus: "absent"}
	add := func(at time.Time, source string) bool {
		if at.IsZero() || at.UTC().Year() < 1 || at.UTC().Year() > 9999 || !at.After(now) {
			return false
		}
		if at.Nanosecond()%1000 != 0 {
			at = at.Truncate(time.Microsecond).Add(time.Microsecond)
		}
		if at.UTC().Year() > 9999 {
			return false
		}
		if h.NotBefore.IsZero() || at.After(h.NotBefore) {
			h.NotBefore = at
			h.Source = source
		}
		return true
	}
	for _, value := range header.Values("Retry-After") {
		add(retryAfter(value, now), "retry_after_header")
	}
	if body == nil {
		return h
	}
	raw, err := io.ReadAll(io.LimitReader(body, MaxBody+1))
	if err != nil {
		h.BodyStatus = "read_error"
		return h
	}
	if len(raw) > MaxBody {
		h.BodyStatus = "oversized"
		return h
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return h
	}
	var root map[string]json.RawMessage
	if !json.Valid(raw) || json.Unmarshal(raw, &root) != nil || root == nil {
		h.BodyStatus = "invalid_json"
		return h
	}
	h.BodyStatus = "unrecognized"
	inspect := func(object map[string]json.RawMessage) {
		for _, key := range []string{"retry_after", "retry_after_seconds"} {
			value := scalar(object[key])
			if value == "" {
				continue
			}
			at := seconds(value, now)
			if key == "retry_after" && at.IsZero() {
				at, _ = http.ParseTime(value)
			}
			if add(at, "retry_after_body") {
				h.BodyStatus = "recognized"
			}
		}
		if value := scalar(object["retry_at"]); offsetTime.MatchString(value) {
			at, err := time.Parse(time.RFC3339Nano, value)
			if err == nil && add(at, "retry_at_body") {
				h.BodyStatus = "recognized"
			}
		}
	}
	inspect(root)
	for _, key := range []string{"data", "error"} {
		var nested map[string]json.RawMessage
		if json.Unmarshal(root[key], &nested) == nil {
			inspect(nested)
		}
	}
	if code := scalar(root["error_code"]); code != "" {
		if n, err := strconv.Atoi(code); err == nil {
			h.ProviderCode = &n
		}
	}
	return h
}
func scalar(raw json.RawMessage) string {
	if len(raw) == 0 || len(raw) > 128 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return string(number)
	}
	return ""
}
func seconds(value string, now time.Time) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return time.Time{}
		}
	}
	// Prevent duration overflow rather than wrapping or shortening a valid wait.
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n > uint64((1<<63-1)/int64(time.Second)) {
		return time.Time{}
	}
	return now.Add(time.Duration(n) * time.Second)
}
func retryAfter(value string, now time.Time) time.Time {
	if len(value) > 128 {
		return time.Time{}
	}
	value = strings.TrimSpace(value)
	if at := seconds(value, now); !at.IsZero() {
		return at
	}
	at, _ := http.ParseTime(value)
	return at
}
