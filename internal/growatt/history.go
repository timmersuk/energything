package growatt

import (
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"net/url"
	"strconv"
	"time"
)

type Window struct {
	Samples  []storage.Sample
	Evidence []storage.Evidence
	Retries  []Retry
}

func (c *Client) FetchLatest(ctx context.Context, d Device) (Window, error) {
	w := Window{}
	if d.Family != "5" {
		return w, problem("configuration", "unsupported_family", 0)
	}
	data, e := c.request(ctx, "/v1/device/mix/mix_last_data", "POST", url.Values{"mix_sn": {d.Serial}}, 1, &w.Retries)
	if e != nil {
		return w, e
	}
	if len(data) == 0 {
		return w, problem("protocol", "missing_results", 1)
	}
	b, _ := json.Marshal(telemetry(data))
	w.Evidence = append(w.Evidence, storage.Evidence{Content: b, Retrieved: storage.At(c.now()), Complete: true})
	s, e := normalize(data, "", d.Serial, 0)
	if e != nil {
		return w, e
	}
	w.Samples = []storage.Sample{s}
	return w, nil
}
func (c *Client) FetchDay(ctx context.Context, d Device, date, zone string) (Window, error) {
	w := Window{}
	if d.Family != "5" {
		return w, problem("configuration", "unsupported_family", 0)
	}
	expected := -1
	rows := []map[string]any{}
	indices := []int{}
	seen := map[string]bool{}
	for page := 1; page <= MaxPages; page++ {
		form := url.Values{"mix_sn": {d.Serial}, "start_date": {date}, "end_date": {date}, "timezone_id": {zone}, "perpage": {"100"}, "page": {strconv.Itoa(page)}}
		data, e := c.request(ctx, "/v1/device/mix/mix_data", "POST", form, page, &w.Retries)
		if e != nil {
			return w, e
		}
		// Keep reproducible suspect telemetry and count context even when pagination fails.
		b, _ := json.Marshal(telemetry(map[string]any{"datas": data["datas"], "count": data["count"], "requested_date": date, "page": page, "assembled_before": len(rows)}))
		w.Evidence = append(w.Evidence, storage.Evidence{Content: b, Retrieved: storage.At(c.now()), Complete: false})
		count, ok := integer(data["count"])
		if !ok || count < 0 {
			return w, problem("pagination", "missing_count", page)
		}
		if count > MaxRows {
			return w, problem("pagination", "count_budget", page)
		}
		if expected < 0 {
			expected = count
		} else if count < expected {
			pe := problem("pagination", "count_changed", page)
			pe.Diagnostic.Expected = &expected
			pe.Diagnostic.Observed = &count
			return w, pe
		}
		// Telemetry can grow while paging. Follow the latest advertised total,
		// within the existing row/page budgets; a shrinking total still fails.
		expected = count
		items, ok := data["datas"].([]any)
		if !ok {
			return w, problem("protocol", "missing_results", page)
		}
		canonical, _ := json.Marshal(items)
		if len(items) > 0 && seen[string(canonical)] {
			return w, problem("pagination", "repeated_page", page)
		}
		seen[string(canonical)] = true
		if len(items) > 100 || len(rows)+len(items) > expected || len(rows)+len(items) < expected && len(items) != 100 {
			pe := problem("pagination", "count_mismatch", page)
			observed := len(rows) + len(items)
			pe.Diagnostic.Expected = &expected
			pe.Diagnostic.Observed = &observed
			return w, pe
		}
		for _, item := range items {
			x, ok := item.(map[string]any)
			if !ok || len(x) == 0 {
				return w, problem("protocol", "missing_results", page)
			}
			rows = append(rows, x)
			indices = append(indices, page-1)
		}
		if len(rows) == expected {
			for i, row := range rows {
				s, e := normalize(row, date, d.Serial, indices[i])
				if e != nil {
					return w, e
				}
				w.Samples = append(w.Samples, s)
			}
			for i := range w.Evidence {
				w.Evidence[i].Complete = true
			}
			return w, nil
		}
	}
	return w, problem("pagination", "count_budget", MaxPages)
}

// DayBounds are bookkeeping bounds only; they do not resolve measurement clocks.
func DayBounds(date string, location *time.Location) (time.Time, time.Time, error) {
	t, e := time.ParseInLocation("2006-01-02", date, location)
	return t, t.AddDate(0, 0, 1), e
}
