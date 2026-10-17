package growatt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// A growing day must retain its history rather than discard every page when a
// new observation arrives. No clock-to-UTC conversion is involved.
func TestDayCountGrowth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		counts []int
		sizes  []int
		want   int
		fail   bool
	}{
		{"one new reading", []int{101, 102}, []int{100, 2}, 102, false},
		{"growth crosses page", []int{101, 201, 202}, []int{100, 100, 2}, 202, false},
		{"shrinking count", []int{102, 101}, []int{100, 1}, 0, true},
		{"early empty", []int{101, 102}, []int{100, 0}, 0, true},
		{"short intermediate page", []int{201, 202}, []int{100, 50}, 0, true},
		{"overrun", []int{101, 102}, []int{100, 3}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := NewClient("synthetic", roundTrip(func(r *http.Request) (*http.Response, error) {
				if calls >= len(tc.counts) {
					t.Fatal("unbounded paging")
				}
				rows := make([]map[string]any, tc.sizes[calls])
				for i := range rows {
					rows[i] = map[string]any{"time": fmt.Sprintf("2026-10-16 %02d:%02d:00", (calls*100+i)/60, (calls*100+i)%60), "soc": 50}
				}
				b, _ := json.Marshal(map[string]any{"error_code": 0, "data": map[string]any{"count": tc.counts[calls], "datas": rows}})
				calls++
				return reply(string(b)), nil
			}))
			w, err := c.FetchDay(context.Background(), Device{"123", "SYN-A", "5"}, "2026-10-16", "Europe/London")
			if tc.fail {
				if err == nil {
					t.Fatal("inconsistent pages accepted")
				}
				return
			}
			if err != nil || len(w.Samples) != tc.want || calls != len(tc.counts) {
				t.Fatalf("history lost on growth: rows=%d calls=%d err=%v", len(w.Samples), calls, err)
			}
			for _, s := range w.Samples {
				if s.Instant != nil {
					t.Fatal("invented measurement instant")
				}
			}
			for _, e := range w.Evidence {
				if !e.Complete {
					t.Fatal("completed traversal marked incomplete")
				}
			}
		})
	}
}
