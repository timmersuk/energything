package octopus

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Oldest is a single-page metadata/list probe over the full supported storage
// date domain. It never ingests observations or follows an archive's next page.
// Ordering is documented; explicit year0001 bounds were accepted in the private
// installation probe on2026-10-15. Empty/default-range assumptions are avoided.
func (c *Client) Oldest(ctx context.Context, m Meter, to time.Time) (*time.Time, json.RawMessage, error) {
	if !validIdentifier(m.MPAN) || !validIdentifier(m.Serial) || (m.Direction != "import" && m.Direction != "export") {
		return nil, nil, problem("configuration", "invalid", 0)
	}
	path := "/v1/electricity-meter-points/" + m.MPAN + "/meters/" + m.Serial + "/consumption/"
	q := url.Values{"period_from": {"0001-01-01T00:00:00Z"}, "period_to": {to.UTC().Format(time.RFC3339)}, "page_size": {"1"}, "order_by": {"period"}}
	retries := []Retry{}
	b, e := c.get(ctx, "https://api.octopus.energy"+path+"?"+q.Encode(), path, q, 1, &retries)
	if e != nil {
		return nil, nil, e
	}
	var envelope struct {
		Count   *int            `json:"count"`
		Results []Reading       `json:"results"`
		Next    json.RawMessage `json:"next"`
	}
	if json.Unmarshal(b, &envelope) != nil || envelope.Count == nil || *envelope.Count < 0 || envelope.Results == nil || len(envelope.Results) > 1 || len(envelope.Next) == 0 {
		return nil, nil, problem("protocol", "invalid", 1)
	}
	if (*envelope.Count == 0) != (len(envelope.Results) == 0) || (*envelope.Count <= 1) != (string(envelope.Next) == "null") {
		return nil, nil, problem("pagination", "count_mismatch", 1)
	}
	evidence, _ := json.Marshal(map[string]any{"probe": "oldest_explicit_year0001", "count": *envelope.Count, "results": envelope.Results, "observed_to": to.UTC().Format(time.RFC3339)})
	if len(envelope.Results) == 0 {
		return nil, evidence, nil
	}
	r := envelope.Results[0]
	start, e := time.Parse(time.RFC3339, r.Start)
	if e != nil {
		return nil, evidence, problem("validation", "invalid_timestamp", 1)
	}
	w := Window{Readings: []Reading{r}}
	if e = Validate(&w, start, start.Add(Slot), to); e != nil {
		return nil, evidence, e
	}
	return &start, evidence, nil
}
