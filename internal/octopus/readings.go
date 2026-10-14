package octopus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"

	"time"
)

const MaxPages = 16
const MaxRecords = 2048
const MaxWindow = 14 * 24 * time.Hour
const Slot = 30 * time.Minute

type Reading struct {
	Energy   json.RawMessage `json:"consumption"`
	Start    string          `json:"interval_start"`
	End      string          `json:"interval_end"`
	Evidence int             `json:"-"`
}
type Evidence struct {
	Content   json.RawMessage
	Retrieved time.Time
	Complete  bool
}
type Interval struct {
	Start, End time.Time
	KWh        string
	Evidence   int
}
type Window struct {
	Readings                               []Reading
	Intervals                              []Interval
	Evidence                               []Evidence
	Retries                                []Retry
	Expected, Missing, Outside, Duplicates int
}

func validBounds(from, to time.Time) bool {
	return from.Year() >= 1 && to.Year() <= 9999 && to.After(from) && to.Sub(from) <= MaxWindow && from.Unix()%1800 == 0 && to.Unix()%1800 == 0 && from.Nanosecond() == 0 && to.Nanosecond() == 0
}

// Fetch is one atomic provider unit: callers retain evidence on any error, but
// may commit Intervals only when error is nil. No partial-page acceptance.
func (c *Client) Fetch(ctx context.Context, m Meter, from, to time.Time) (Window, error) {
	out := Window{}
	if !validIdentifier(m.MPAN) || !validIdentifier(m.Serial) || (m.Direction != "import" && m.Direction != "export") || !validBounds(from, to) {
		return out, problem("configuration", "invalid", 0)
	}
	path := "/v1/electricity-meter-points/" + m.MPAN + "/meters/" + m.Serial + "/consumption/"
	q := url.Values{"period_from": {from.UTC().Format(time.RFC3339)}, "period_to": {to.Add(-time.Second).UTC().Format(time.RFC3339)}, "page_size": {"100"}, "order_by": {"period"}}
	next := "https://api.octopus.energy" + path + "?" + q.Encode()
	seen := map[string]bool{}
	contents := map[[32]byte]bool{}
	count := -1
	for page := 1; next != ""; page++ {
		if page > MaxPages {
			return out, problem("pagination", "incomplete", page)
		}
		if err := destination(next, path, q); err != nil {
			return out, problem("pagination", "invalid", page)
		}
		u, _ := url.Parse(next)
		u.RawQuery = u.Query().Encode()
		canonical := u.String()
		if seen[canonical] {
			return out, problem("pagination", "conflict", page)
		}
		seen[canonical] = true
		b, err := c.get(ctx, next, path, q, page, &out.Retries)
		if err != nil {
			return out, err
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal(b, &envelope) != nil || envelope == nil {
			return out, problem("protocol", "invalid", page)
		}
		results, exists := envelope["results"]
		if !exists || len(results) == 0 || results[0] != '[' {
			return out, problem("protocol", "incomplete", page)
		}
		var selected any
		decoder := json.NewDecoder(bytes.NewReader(results))
		decoder.UseNumber()
		if decoder.Decode(&selected) != nil {
			return out, problem("protocol", "invalid", page)
		}
		evidence, _ := json.Marshal(map[string]any{"results": telemetry(selected), "page": page})
		out.Evidence = append(out.Evidence, Evidence{evidence, c.now(), false})
		var rows []Reading
		if json.Unmarshal(results, &rows) != nil {
			return out, problem("validation", "invalid", page)
		}
		if len(out.Readings)+len(rows) > MaxRecords {
			return out, problem("protocol", "incomplete", page)
		}
		for i := range rows {
			rows[i].Evidence = len(out.Evidence) - 1
		}
		out.Readings = append(out.Readings, rows...)
		// Count is required and must be stable across pages, including duplicates.
		var pageCount int
		if raw, ok := envelope["count"]; !ok || json.Unmarshal(raw, &pageCount) != nil || pageCount < 0 || pageCount > MaxRecords {
			return out, problem("protocol", "incomplete", page)
		}
		if count < 0 {
			count = pageCount
		}
		if count != pageCount {
			return out, problem("protocol", "conflict", page)
		}
		raw, ok := envelope["next"]
		if !ok {
			return out, problem("protocol", "incomplete", page)
		}
		next = ""
		if string(raw) != "null" {
			if json.Unmarshal(raw, &next) != nil || next == "" {
				return out, problem("protocol", "invalid", page)
			}
		}
		digest := sha256.Sum256(results)
		if len(rows) > 0 && contents[digest] {
			return out, problem("pagination", "conflict", page)
		}
		contents[digest] = true
		if next != "" && len(rows) == 0 {
			return out, problem("pagination", "incomplete", page)
		}
	}
	if len(out.Readings) != count {
		return out, problem("protocol", "incomplete", len(out.Evidence))
	}
	for i := range out.Evidence {
		out.Evidence[i].Complete = true
	}
	err := Validate(&out, from, to, c.now())
	if len(out.Evidence) > 0 {
		var selected map[string]any
		d := json.NewDecoder(bytes.NewReader(out.Evidence[0].Content))
		d.UseNumber()
		_ = d.Decode(&selected)
		selected["window_context"] = map[string]any{"from_inclusive": from.UTC().Format(time.RFC3339), "to_exclusive": to.UTC().Format(time.RFC3339), "boundary_neighbors_excluded": out.Outside, "identical_duplicates": out.Duplicates, "assembled_records": len(out.Readings)}
		out.Evidence[0].Content, _ = json.Marshal(selected)
	}
	return out, err
}
