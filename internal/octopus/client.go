package octopus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://api.octopus.energy/v1/"
const MaxBody = 4 * 1024 * 1024

// Error contains only enumerated public context; underlying URLs/bodies are discarded.
type Error struct {
	Class, Code   string
	Page          int
	DeferredUntil time.Time
}

func (e *Error) Error() string { return "Octopus " + e.Class + "/" + e.Code }
func problem(class, code string, page int) *Error {
	return &Error{Class: class, Code: code, Page: page}
}

type Retry struct {
	Page, Attempt, Status int
	Code                  string
	Wait                  time.Duration
}
type Client struct {
	key    string
	http   *http.Client
	now    func() time.Time
	wait   func(context.Context, time.Duration) error
	jitter func() time.Duration
}

// Transport injection cannot change the production destination policy or redirect rule.
func NewClient(key string, transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{key: key, http: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now, wait: func(ctx context.Context, d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}, jitter: func() time.Duration { return time.Duration(rand.IntN(251)) * time.Millisecond }}
}
func destination(raw, path string, query url.Values) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "api.octopus.energy" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Path != path || u.RawPath != "" {
		return problem("pagination", "invalid", 0)
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return problem("pagination", "invalid", 0)
	}
	for k, v := range q {
		if len(v) != 1 {
			return problem("pagination", "invalid", 0)
		}
		if k == "page" {
			n, e := strconv.Atoi(v[0])
			if e != nil || n < 1 || n > MaxPages {
				return problem("pagination", "invalid", 0)
			}
		} else if len(query[k]) != 1 || query.Get(k) != v[0] {
			return problem("pagination", "invalid", 0)
		}
	}
	for k, v := range query {
		if len(q[k]) != 1 || q.Get(k) != v[0] {
			return problem("pagination", "invalid", 0)
		}
	}
	return nil
}
func (c *Client) get(ctx context.Context, raw, path string, q url.Values, page int, retries *[]Retry) ([]byte, error) {
	if destination(raw, path, q) != nil {
		return nil, problem("pagination", "invalid", page)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return nil, problem("network", "cancelled", page)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, problem("protocol", "invalid", page)
		}
		req.SetBasicAuth(c.key, "")
		response, requestErr := c.http.Do(req)
		status := 0
		code, class := "io", "network"
		delay := time.Duration(1<<attempt)*time.Second + c.jitter()
		deferred := time.Time{}
		if requestErr == nil {
			status = response.StatusCode
			retryAfter := response.Header.Get("Retry-After")
			if status == 200 {
				body, bodyErr := io.ReadAll(io.LimitReader(response.Body, MaxBody+1))
				response.Body.Close()
				if bodyErr == nil {
					if len(body) > MaxBody || !json.Valid(body) {
						return nil, problem("protocol", "invalid", page)
					}
					return body, nil
				}
				if ctx.Err() != nil {
					return nil, problem("network", "cancelled", page)
				}
				var timeout interface{ Timeout() bool }
				if !errors.As(bodyErr, &timeout) || !timeout.Timeout() {
					return nil, problem("protocol", "incomplete", page)
				}
				requestErr = bodyErr // Discard the incomplete body and use this page's budget.
			} else {
				response.Body.Close()
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
					return nil, problem("protocol", "invalid", page)
				}
				if seconds, err := strconv.ParseUint(retryAfter, 10, 32); err == nil {
					delay = time.Duration(seconds) * time.Second
				} else if date, err := http.ParseTime(retryAfter); err == nil {
					delay = date.Sub(c.now())
					if delay < 0 {
						delay = 0
					}
				}
				if delay > 30*time.Second {
					deferred = c.now().Add(delay)
				}
			}
		}
		if requestErr != nil {
			if ctx.Err() != nil {
				return nil, problem("network", "cancelled", page)
			}
			var timeout interface{ Timeout() bool }
			if errors.As(requestErr, &timeout) && timeout.Timeout() {
				code = "timeout"
			}
		}
		*retries = append(*retries, Retry{page, attempt + 1, status, code, delay})
		if !deferred.IsZero() {
			return nil, &Error{class, code, page, deferred}
		}
		if attempt == 2 {
			return nil, problem(class, code, page)
		}
		if c.wait(ctx, delay) != nil {
			return nil, problem("network", "cancelled", page)
		}
	}
	return nil, problem("network", "io", page)
}
func (c *Client) Account(ctx context.Context, number string) (Account, error) {
	var a Account
	if !validIdentifier(number) {
		return a, problem("configuration", "invalid", 0)
	}
	path := "/v1/accounts/" + number + "/"
	retries := []Retry{}
	b, err := c.get(ctx, "https://api.octopus.energy"+path, path, url.Values{}, 0, &retries)
	if err != nil {
		return a, err
	}
	if json.Unmarshal(b, &a) != nil || a.Properties == nil {
		return Account{}, problem("protocol", "incomplete", 0)
	}
	return a, nil
}

// Selected evidence contains telemetry only. Pagination destinations, discovery,
// transcripts and credential-shaped fields are deliberately excluded recursively.
func telemetry(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			key := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(k))
			safe := true
			for _, bad := range []string{"token", "password", "secret", "authorization", "apikey", "cookie", "header", "address", "account", "credential", "url"} {
				if strings.Contains(key, bad) {
					safe = false
				}
			}
			if safe {
				out[k] = telemetry(v)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = telemetry(v)
		}
		return out
	default:
		return v
	}
}
