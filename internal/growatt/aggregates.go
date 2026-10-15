package growatt

import (
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type AggregateWindow struct {
	Aggregates []storage.Aggregate
	Evidence   []storage.Evidence
	Retries    []Retry
}

var aggregateDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

// permit durably reserves each transmission before credentials can leave the process.
// A quota deferral fails the complete unit; it never accepts a partial page set.
func (c *Client) FetchAggregateDay(ctx context.Context, plant, date string, permit func(context.Context) (time.Time, error)) (AggregateWindow, error) {
	return c.FetchAggregateRange(ctx, plant, date, date, permit)
}

// FetchAggregateRange retrieves up to seven inclusive dates in one bounded unit.
func (c *Client) FetchAggregateRange(ctx context.Context, plant, from, to string, permit func(context.Context) (time.Time, error)) (AggregateWindow, error) {
	w := AggregateWindow{}
	_, end, e := storage.AggregateDateBounds(from, to)
	if e != nil || end.AddDate(0, 0, -1).After(c.now().UTC()) || !identifier(plant) || permit == nil {
		return w, problem("configuration", "invalid", 0)
	}
	expected := -1
	assembled := 0
	seenPages := map[string]bool{}
	seen := map[string]string{}
	for page := 1; page <= MaxPages; page++ {
		if ctx.Err() != nil {
			return w, problem("network", "cancelled", page)
		}
		due, e := permit(ctx)
		if e != nil {
			return w, problem("storage", "io", page)
		}
		if !due.IsZero() {
			return w, &Error{Class: "rate_limit", Code: "limited", Page: page, DeferredUntil: due}
		}
		data, e := c.request(ctx, "/v1/plant/energy", "GET", url.Values{"plant_id": {plant}, "start_date": {from}, "end_date": {to}, "time_unit": {"day"}, "page": {strconv.Itoa(page)}, "perpage": {"100"}}, page, &w.Retries)
		if e != nil {
			return w, e
		}
		b, _ := json.Marshal(telemetry(map[string]any{"energys": data["energys"], "count": data["count"], "time_unit": data["time_unit"], "requested_from": from, "requested_to": to, "page": page}))
		w.Evidence = append(w.Evidence, storage.Evidence{Content: b, Retrieved: storage.At(c.now())})
		if data["time_unit"] != "day" {
			return w, problem("validation", "aggregate_unit", page)
		}
		count, ok := integer(data["count"])
		if !ok || count < 0 {
			return w, problem("pagination", "missing_count", page)
		}
		if count > MaxRows {
			return w, problem("pagination", "count_budget", page)
		}
		if expected < 0 {
			expected = count
		} else if expected != count {
			return w, problem("pagination", "count_changed", page)
		}
		items, ok := data["energys"].([]any)
		if !ok {
			return w, problem("protocol", "missing_results", page)
		}
		b, _ = json.Marshal(items)
		if len(items) > 0 && seenPages[string(b)] {
			return w, problem("pagination", "repeated_page", page)
		}
		seenPages[string(b)] = true
		if len(items) > 100 || assembled+len(items) > expected || (len(items) == 0 && assembled < expected) {
			return w, problem("pagination", "count_mismatch", page)
		}
		for _, item := range items {
			row, ok := item.(map[string]any)
			if !ok {
				return w, problem("protocol", "missing_results", page)
			}
			period, ok := row["date"].(string)
			_, dateErr := time.Parse("2006-01-02", period)
			if !ok || dateErr != nil || period < from || period > to {
				return w, problem("validation", "aggregate_date", page)
			}
			value, ok := row["energy"].(string)
			if !ok || len(value) > 128 || !aggregateDecimal.MatchString(value) {
				return w, problem("validation", "aggregate_decimal", page)
			}
			if old, ok := seen[period]; ok {
				a, _ := new(big.Rat).SetString(old)
				b, _ := new(big.Rat).SetString(value)
				if a.Cmp(b) != 0 {
					return w, problem("validation", "aggregate_conflict", page)
				}
			} else {
				seen[period] = value
				w.Aggregates = append(w.Aggregates, storage.Aggregate{PlantID: plant, Metric: "generation", Resolution: "day", Period: period, Value: value, Unit: "kWh", Calendar: storage.AggregateCalendar, Quality: storage.AggregateQuality(value), Evidence: page - 1})
			}
		}
		assembled += len(items)
		if assembled == expected {
			for i := range w.Evidence {
				w.Evidence[i].Complete = true
			}
			return w, nil
		}
	}
	return w, problem("pagination", "count_budget", MaxPages)
}
