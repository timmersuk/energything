package growatt

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxBody = 4 * 1024 * 1024
const MaxPages = 100
const MaxRows = 10000

type Error struct {
	Class, Code   string
	Page          int
	DeferredUntil time.Time
	Diagnostic    storage.Diagnostic
}

func (e *Error) Error() string { return "Growatt " + e.Class + "/" + e.Code }
func problem(class, code string, page int) *Error {
	return &Error{Class: class, Code: code, Page: page}
}

type Retry struct {
	Page, Attempt, Status int
	Code                  string
	Wait                  time.Duration
}
type Client struct {
	token  string
	http   *http.Client
	now    func() time.Time
	wait   func(context.Context, time.Duration) error
	jitter func() time.Duration
}

func NewClient(token string, transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{token: token, http: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now, jitter: func() time.Duration { return time.Duration(rand.IntN(251)) * time.Millisecond }, wait: func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}}
}
func identifier(s string) bool {
	if len(s) == 0 || len(s) > 100 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func destination(raw, method string, form url.Values) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "openapi.growatt.com" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || u.RawQuery != "" {
		return false
	}
	var keys []string
	switch u.Path {
	case "/v1/plant/list":
		if method != "GET" {
			return false
		}
		keys = []string{"page", "perpage"}
	case "/v1/plant/energy":
		if method != "GET" || !identifier(form.Get("plant_id")) || form.Get("time_unit") != "day" || form.Get("start_date") != form.Get("end_date") {
			return false
		}
		if _, e := time.Parse("2006-01-02", form.Get("start_date")); e != nil {
			return false
		}
		keys = []string{"plant_id", "start_date", "end_date", "time_unit", "page", "perpage"}
	case "/v1/device/list":
		if method != "GET" || !identifier(form.Get("plant_id")) {
			return false
		}
		keys = []string{"plant_id", "page", "perpage"}
	case "/v1/device/mix/mix_last_data":
		if method != "POST" || !identifier(form.Get("mix_sn")) {
			return false
		}
		keys = []string{"mix_sn"}
	case "/v1/device/mix/mix_data":
		if method != "POST" || !identifier(form.Get("mix_sn")) || form.Get("start_date") != form.Get("end_date") {
			return false
		}
		if _, e := time.Parse("2006-01-02", form.Get("start_date")); e != nil {
			return false
		}
		if _, e := time.LoadLocation(form.Get("timezone_id")); e != nil {
			return false
		}
		keys = []string{"mix_sn", "start_date", "end_date", "timezone_id", "page", "perpage"}
	default:
		return false
	}
	if len(form) != len(keys) {
		return false
	}
	for _, k := range keys {
		if len(form[k]) != 1 {
			return false
		}
	}
	if form.Has("page") {
		n, e := strconv.Atoi(form.Get("page"))
		if e != nil || n < 1 || n > MaxPages || form.Get("perpage") != "100" {
			return false
		}
	}
	return true
}
func integer(v any) (int, bool) {
	s := ""
	switch x := v.(type) {
	case json.Number:
		s = string(x)
	case string:
		s = x
	default:
		return 0, false
	}
	n, e := strconv.Atoi(s)
	return n, e == nil
}
func (c *Client) request(ctx context.Context, path, method string, form url.Values, page int, retries *[]Retry) (map[string]any, error) {
	raw := "https://openapi.growatt.com" + path
	if !destination(raw, method, form) || c.token == "" {
		return nil, problem("configuration", "invalid", page)
	}
	maxAttempts := 3
	if path == "/v1/plant/energy" {
		maxAttempts = 1
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, problem("network", "cancelled", page)
		}
		target := raw
		var body io.Reader
		if method == "GET" {
			target += "?" + form.Encode()
		} else {
			body = strings.NewReader(form.Encode())
		}
		req, _ := http.NewRequestWithContext(ctx, method, target, body)
		req.Header.Set("token", c.token)
		if method == "POST" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, e := c.http.Do(req)
		status := 0
		class, code := "network", "io"
		delay := time.Duration(1<<(attempt-1))*time.Second + c.jitter()
		if e == nil {
			status = resp.StatusCode
			if status == 200 {
				b, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
				resp.Body.Close()
				if ctx.Err() != nil {
					return nil, problem("network", "cancelled", page)
				}
				if readErr != nil {
					return nil, problem("protocol", "incomplete", page)
				}
				if len(b) > MaxBody {
					return nil, problem("protocol", "body_budget", page)
				}
				if !json.Valid(b) {
					return nil, problem("protocol", "invalid_json", page)
				}
				dec := json.NewDecoder(bytes.NewReader(b))
				dec.UseNumber()
				var envelope map[string]any
				if dec.Decode(&envelope) != nil || envelope == nil {
					return nil, problem("protocol", "missing_results", page)
				}
				n, ok := integer(envelope["error_code"])
				if !ok {
					return nil, problem("protocol", "missing_error_code", page)
				}
				if n != 0 {
					switch n {
					case 10011:
						return nil, problem("authentication", "unauthorized", page)
					case 10012:
						return nil, &Error{Class: "rate_limit", Code: "limited", Page: page, DeferredUntil: c.now().Add(5 * time.Minute)}
					default:
						pe := problem("protocol", "provider_error", page)
						pe.Diagnostic.ProviderCode = &n
						return nil, pe
					}
				}
				data, ok := envelope["data"].(map[string]any)
				if !ok {
					return nil, problem("protocol", "missing_results", page)
				}
				return data, nil
			}
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()
			switch {
			case status == 401 || status == 403:
				return nil, problem("authentication", "unauthorized", page)
			case status == 429:
				class, code = "rate_limit", "limited"
			case status == 408:
				code = "timeout"
			case status >= 500 && status <= 599:
				code = "unavailable"
			default:
				pe := problem("protocol", "http_status", page)
				pe.Diagnostic.HTTPStatus = status
				return nil, pe
			}
			if sec, e := strconv.ParseUint(retryAfter, 10, 32); e == nil {
				delay = time.Duration(sec) * time.Second
			} else if date, e := http.ParseTime(retryAfter); e == nil {
				delay = date.Sub(c.now())
				if delay < 0 {
					delay = 0
				}
			}
		} else {
			if ctx.Err() != nil {
				return nil, problem("network", "cancelled", page)
			}
			if x, ok := e.(interface{ Timeout() bool }); ok && x.Timeout() {
				code = "timeout"
			}
		}
		*retries = append(*retries, Retry{page, attempt, status, code, delay})
		if delay > 30*time.Second {
			return nil, &Error{Class: class, Code: code, Page: page, DeferredUntil: c.now().Add(delay)}
		}
		if attempt == maxAttempts {
			return nil, problem(class, code, page)
		}
		if c.wait(ctx, delay) != nil {
			return nil, problem("network", "cancelled", page)
		}
	}
	return nil, problem("network", "io", page)
}

// Selected telemetry excludes transcript and discovery material recursively.
func telemetry(v any) any {
	switch x := v.(type) {
	case map[string]any:
		o := map[string]any{}
		for k, v := range x {
			key := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(k))
			safe := true
			for _, bad := range []string{"token", "password", "secret", "authorization", "apikey", "cookie", "header", "address", "account", "credential", "url"} {
				if strings.Contains(key, bad) {
					safe = false
				}
			}
			if safe {
				o[k] = telemetry(v)
			}
		}
		return o
	case []any:
		o := make([]any, len(x))
		for i, v := range x {
			o[i] = telemetry(v)
		}
		return o
	default:
		return v
	}
}
