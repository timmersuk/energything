package growatt

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestDiscoveryAndGenerationCountGrowth(t *testing.T) {
	t.Run("discovery", func(t *testing.T) {
		calls := 0
		c := NewClient("synthetic", roundTrip(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return reply(`{"error_code":0,"data":{"count":2,"plants":[{"plant_id":"SYN-A"}]}}`), nil
			}
			return reply(`{"error_code":0,"data":{"count":3,"plants":[{"plant_id":"SYN-B"},{"plant_id":"SYN-C"}]}}`), nil
		}))
		rows, err := c.list(context.Background(), "/v1/plant/list", "plants", url.Values{}, &[]Retry{})
		if err != nil || len(rows) != 3 || calls != 2 {
			t.Fatalf("growing discovery discarded: rows=%d calls=%d err=%v", len(rows), calls, err)
		}
	})
	t.Run("generation", func(t *testing.T) {
		calls, permits := 0, 0
		c := NewClient("synthetic", roundTrip(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return reply(`{"error_code":0,"data":{"count":2,"time_unit":"day","energys":[{"date":"2026-01-01","energy":"1"}]}}`), nil
			}
			return reply(`{"error_code":0,"data":{"count":3,"time_unit":"day","energys":[{"date":"2026-01-02","energy":"2"},{"date":"2026-01-03","energy":"3"}]}}`), nil
		}))
		w, err := c.FetchAggregateRange(context.Background(), "SYN-PLANT", "2026-01-01", "2026-01-03", func(context.Context) (time.Time, error) {
			permits++
			return time.Time{}, nil
		})
		if err != nil || len(w.Aggregates) != 3 || calls != 2 || permits != 2 {
			t.Fatalf("growing generation discarded: rows=%d calls=%d permits=%d err=%v", len(w.Aggregates), calls, permits, err)
		}
		if len(w.Evidence) != 2 || !w.Evidence[1].Complete {
			t.Fatal("missing completed evidence")
		}
	})
}

func TestDiscoveryAndGenerationCountShrink(t *testing.T) {
	for _, generation := range []bool{false, true} {
		calls := 0
		c := NewClient("synthetic", roundTrip(func(*http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return reply(`{"error_code":0,"data":{"count":3,"time_unit":"day","plants":[{"plant_id":"SYN-A"}],"energys":[{"date":"2026-01-01","energy":"1"}]}}`), nil
			}
			return reply(`{"error_code":0,"data":{"count":2,"time_unit":"day","plants":[],"energys":[]}}`), nil
		}))
		var err error
		if generation {
			_, err = c.FetchAggregateDay(context.Background(), "SYN-PLANT", "2026-01-01", func(context.Context) (time.Time, error) { return time.Time{}, nil })
		} else {
			_, err = c.list(context.Background(), "/v1/plant/list", "plants", url.Values{}, &[]Retry{})
		}
		pe, ok := err.(*Error)
		if !ok || pe.Code != "count_changed" || calls != 2 {
			t.Fatalf("shrinking count accepted: generation=%v calls=%d err=%v", generation, calls, err)
		}
	}
}
