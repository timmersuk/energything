package growatt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAggregateWholeUnit(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"exact", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"1.234567890123456789"}]}`, true},
		{"zero", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"0"}]}`, true},
		{"empty", `{"count":0,"time_unit":"day","energys":[]}`, true},
		{"null", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":null}]}`, false},
		{"numeric", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":1.1}]}`, false},
		{"absent", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15"}]}`, false},
		{"placeholder", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"--"}]}`, false},
		{"negative", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"-1"}]}`, false},
		{"unit", `{"count":0,"time_unit":"month","energys":[]}`, false},
		{"date", `{"count":1,"time_unit":"day","energys":[{"date":"2026-01-16","energy":"1"}]}`, false},
		{"conflict", `{"count":2,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"1"},{"date":"2026-01-15","energy":"2"}]}`, false},
		{"equal", `{"count":2,"time_unit":"day","energys":[{"date":"2026-01-15","energy":"1"},{"date":"2026-01-15","energy":"1.0"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/v1/plant/energy" || r.Method != "GET" || r.URL.Query().Get("start_date") != r.URL.Query().Get("end_date") {
					t.Fatal("destination")
				}
				return reply(`{"error_code":0,"data":` + tc.body + `}`), nil
			}))
			w, e := c.FetchAggregateDay(context.Background(), "SYN-PLANT", "2026-01-15", func(context.Context) (time.Time, error) { return time.Time{}, nil })
			if (e == nil) != tc.valid {
				t.Fatal(e)
			}
			if tc.valid && len(w.Evidence) != 1 {
				t.Fatal(w)
			}
			if tc.name == "zero" && w.Aggregates[0].Quality != "reported_zero_availability_unknown" {
				t.Fatal("zero coverage claim")
			}
		})
	}
}
func TestAggregatePaginationAndNoInlineRetry(t *testing.T) {
	for _, mode := range []string{"complete", "changed", "cycle", "early", "quota", "cancelled", "http", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			permits := 0
			c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if mode == "http" || mode == "redirect" {
					res := reply("")
					res.StatusCode = 500
					if mode == "redirect" {
						res.StatusCode = 302
					}
					return res, nil
				}
				count := 2
				value := "1.0"
				if calls == 2 {
					value = "1"
					if mode == "changed" {
						count = 3
					}
				}
				items := fmt.Sprintf(`[{"date":"2026-01-15","energy":"%s"}]`, value)
				if mode == "cycle" {
					items = `[{"date":"2026-01-15","energy":"1.0"}]`
				}
				if mode == "early" && calls == 2 {
					items = `[]`
				}
				return reply(fmt.Sprintf(`{"error_code":0,"data":{"count":%d,"time_unit":"day","energys":%s}}`, count, items)), nil
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			w, e := c.FetchAggregateDay(ctx, "SYN-PLANT", "2026-01-15", func(context.Context) (time.Time, error) {
				permits++
				if mode == "quota" && permits == 2 {
					return time.Now().Add(5 * time.Minute), nil
				}
				return time.Time{}, nil
			})
			if (e == nil) != (mode == "complete") {
				t.Fatal(e)
			}
			if (mode == "http" || mode == "redirect") && calls != 1 {
				t.Fatal("hidden retries")
			}
			if mode == "complete" && (len(w.Aggregates) != 1 || !w.Evidence[1].Complete) {
				t.Fatal("incomplete duplicate collapse")
			}
			if mode == "quota" && strings.Contains(fmt.Sprint(e), "synthetic") {
				t.Fatal("secret")
			}
		})
	}
}

func TestAggregateSevenDayRange(t *testing.T) {
	for _, mode := range []string{"complete", "missing", "outside", "invalid", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				q := r.URL.Query()
				if q.Get("start_date") != "2026-01-12" || q.Get("end_date") != "2026-01-18" {
					t.Fatal(q)
				}
				rows := []map[string]any{}
				for n := 0; n < 7; n++ {
					if mode == "missing" && n == 2 {
						continue
					}
					date := fmt.Sprintf("2026-01-%02d", 12+n)
					row := map[string]any{"date": date, "energy": "1.234567890123456789"}
					if mode == "outside" && n == 6 {
						row["date"] = "2026-01-19"
					}
					if mode == "invalid" && n == 6 {
						row["energy"] = nil
					}
					rows = append(rows, row)
				}
				if mode == "conflict" {
					rows = append(rows, map[string]any{"date": "2026-01-12", "energy": "2"})
				}
				b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": len(rows), "time_unit": "day", "energys": rows}})
				return reply(string(b)), nil
			}))
			w, e := c.FetchAggregateRange(context.Background(), "SYN-PLANT", "2026-01-12", "2026-01-18", func(context.Context) (time.Time, error) { return time.Time{}, nil })
			valid := mode == "complete" || mode == "missing"
			if (e == nil) != valid || calls != 1 {
				t.Fatal(e, calls)
			}
			if valid {
				want := 7
				if mode == "missing" {
					want = 6
				}
				if len(w.Aggregates) != want || !w.Evidence[0].Complete {
					t.Fatal(w)
				}
			}
		})
	}
	for _, dates := range [][2]string{{"2026-01-12", "2026-01-19"}, {"2026-01-18", "2026-01-12"}, {"2026-01-12", "bad"}, {"2026-01-12", "2026-01-00"}} {
		c := NewClient("synthetic", roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("invalid range transmitted"); return nil, nil }))
		permits := 0
		if _, e := c.FetchAggregateRange(context.Background(), "SYN-PLANT", dates[0], dates[1], func(context.Context) (time.Time, error) { permits++; return time.Time{}, nil }); e == nil || permits != 0 {
			t.Fatal("invalid range reserved/transmitted", e)
		}
	}
}
