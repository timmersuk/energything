package growatt

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type Device struct{ PlantID, Serial, Family string }

func (d Device) Key() string { b, _ := json.Marshal([]string{d.PlantID, d.Serial}); return string(b) }

type Discovery struct {
	Candidates []Device
	Selected   *Device
	State      string
	Retries    []Retry
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return string(x)
	}
	return ""
}
func (c *Client) list(ctx context.Context, path, key string, form url.Values, retries *[]Retry) ([]map[string]any, error) {
	out := []map[string]any{}
	expected := -1
	seen := map[string]bool{}
	for page := 1; page <= MaxPages; page++ {
		form.Set("page", strconv.Itoa(page))
		form.Set("perpage", "100")
		data, e := c.request(ctx, path, "GET", form, page, retries)
		if e != nil {
			return nil, e
		}
		count, ok := integer(data["count"])
		if !ok || count < 0 {
			return nil, problem("pagination", "missing_count", page)
		}
		if count > MaxRows {
			return nil, problem("pagination", "count_budget", page)
		}
		if expected < 0 {
			expected = count
		} else if expected != count {
			return nil, problem("pagination", "count_changed", page)
		}
		rows, ok := data[key].([]any)
		if !ok {
			return nil, problem("protocol", "missing_results", page)
		}
		b, _ := json.Marshal(rows)
		if len(rows) > 0 && seen[string(b)] {
			return nil, problem("pagination", "repeated_page", page)
		}
		seen[string(b)] = true
		if len(rows) > 100 || len(out)+len(rows) > expected || len(rows) == 0 && len(out) < expected {
			return nil, problem("pagination", "count_mismatch", page)
		}
		for _, row := range rows {
			x, ok := row.(map[string]any)
			if !ok {
				return nil, problem("protocol", "missing_results", page)
			}
			out = append(out, x)
		}
		if len(out) == expected {
			return out, nil
		}
	}
	return nil, problem("pagination", "count_budget", MaxPages)
}
func (c *Client) Discover(ctx context.Context, plant, serial string) (Discovery, error) {
	d := Discovery{Candidates: []Device{}, State: "unavailable"}
	if plant != "" && !identifier(plant) || serial != "" && !identifier(serial) {
		return d, problem("configuration", "invalid", 0)
	}
	plants, e := c.list(ctx, "/v1/plant/list", "plants", url.Values{}, &d.Retries)
	if e != nil {
		return d, e
	}
	seenPlants := map[string]bool{}
	seenDevices := map[string]string{}
	for _, p := range plants {
		id := text(p["plant_id"])
		if !identifier(id) {
			return d, problem("protocol", "invalid_identity", 0)
		}
		if seenPlants[id] {
			return d, problem("protocol", "conflict", 0)
		}
		seenPlants[id] = true
		if plant != "" && id != plant {
			continue
		}
		devices, e := c.list(ctx, "/v1/device/list", "devices", url.Values{"plant_id": {id}}, &d.Retries)
		if e != nil {
			return d, e
		}
		for _, v := range devices {
			sn := text(v["device_sn"])
			if !identifier(sn) {
				return d, problem("protocol", "invalid_identity", 0)
			}
			family := text(v["type"])
			key := id + "/" + sn
			if old, ok := seenDevices[key]; ok {
				if old != family {
					return d, problem("protocol", "conflict", 0)
				}
				continue
			}
			seenDevices[key] = family
			d.Candidates = append(d.Candidates, Device{id, sn, family})
		}
	}
	eligible := []Device{}
	for _, v := range d.Candidates {
		if v.Family == "5" && (serial == "" || v.Serial == serial) {
			eligible = append(eligible, v)
		}
	}
	if len(eligible) == 1 {
		d.Selected = &eligible[0]
		d.State = "selected"
	} else if len(eligible) > 1 || plant != "" || serial != "" {
		d.State = "selection_required"
	}
	return d, nil
}
