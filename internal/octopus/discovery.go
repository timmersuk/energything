// Package octopus owns confirmed electricity discovery and bounded protocol validation.
package octopus

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

type Account struct {
	Properties []struct {
		Points []Point `json:"electricity_meter_points"`
	} `json:"properties"`
}
type Point struct {
	MPAN     string `json:"mpan"`
	IsExport *bool  `json:"is_export"`
	Meters   []struct {
		Serial string `json:"serial_number"`
	} `json:"meters"`
	// Description is ephemeral conflict evidence only; never persisted or exposed.
	Description json.RawMessage `json:"-"`
}

func (p *Point) UnmarshalJSON(b []byte) error {
	type plain Point
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = Point(v)
	var fields any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&fields); err != nil {
		return err
	}
	p.Description, _ = json.Marshal(fields)
	return nil
}

type Choice struct{ MPAN, Serial string }
type Meter struct{ MPAN, Serial, Direction string }

func (m Meter) Key() string {
	b, _ := json.Marshal([]string{m.MPAN, m.Serial, m.Direction})
	return string(b)
}

type Stream struct {
	State      string
	Meter      *Meter
	Candidates []Meter
}
type Discovery struct{ Import, Export Stream }

func validIdentifier(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// Select never infers a direction from configuration, tariffs or missing fields.
func Select(a Account, importChoice, exportChoice Choice) Discovery {
	selectStream := func(direction string, choice Choice) Stream {
		out := Stream{State: "unavailable", Candidates: []Meter{}}
		seen := map[string]string{}
		conflict := false
		for _, property := range a.Properties {
			for _, p := range property.Points {
				if p.IsExport == nil || (*p.IsExport) != (direction == "export") {
					continue
				}
				for _, m := range p.Meters {
					if !validIdentifier(p.MPAN) || !validIdentifier(m.Serial) {
						conflict = true
						continue
					}
					meter := Meter{p.MPAN, m.Serial, direction}
					key := meter.Key()
					description := string(p.Description)
					if prior, exists := seen[key]; exists {
						if prior != description {
							conflict = true
						}
						continue
					}
					seen[key] = description
					out.Candidates = append(out.Candidates, meter)
				}
			}
		}
		sort.Slice(out.Candidates, func(i, j int) bool { return out.Candidates[i].Key() < out.Candidates[j].Key() })
		if conflict {
			out.State = "conflict"
			return out
		}
		matching := []Meter{}
		for _, m := range out.Candidates {
			if (choice.MPAN == "" || m.MPAN == choice.MPAN) && (choice.Serial == "" || m.Serial == choice.Serial) {
				matching = append(matching, m)
			}
		}
		if (choice.MPAN == "") != (choice.Serial == "") {
			out.State = "selection_required"
			return out
		}
		if len(matching) == 1 {
			out.State = "selected"
			out.Meter = &matching[0]
		} else if len(out.Candidates) > 0 || strings.TrimSpace(choice.MPAN+choice.Serial) != "" {
			out.State = "selection_required"
		}
		return out
	}
	return Discovery{selectStream("import", importChoice), selectStream("export", exportChoice)}
}
