package octopus

import (
	"encoding/json"
	"testing"
)

func TestIndependentSelection(t *testing.T) {
	var a Account
	err := json.Unmarshal([]byte(`{"properties":[{"electricity_meter_points":[{"mpan":"synthetic-import-a","is_export":false,"meters":[{"serial_number":"a"}]},{"mpan":"synthetic-import-b","is_export":false,"meters":[{"serial_number":"b"}]},{"mpan":"synthetic-export","is_export":true,"meters":[{"serial_number":"e"}]},{"mpan":"unknown","is_export":null,"meters":[{"serial_number":"u"}]}]}]}`), &a)
	if err != nil {
		t.Fatal(err)
	}
	d := Select(a, Choice{}, Choice{})
	if d.Import.State != "selection_required" || d.Export.State != "selected" {
		t.Fatal(d)
	}
	d = Select(a, Choice{"synthetic-import-a", "a"}, Choice{"wrong", "e"})
	if d.Import.State != "selected" || d.Export.State != "selection_required" {
		t.Fatal(d)
	}
	a.Properties = append(a.Properties, a.Properties[0])
	if Select(a, Choice{"synthetic-import-a", "a"}, Choice{}).Import.State != "selected" {
		t.Fatal("duplicate discovery")
	}
	a.Properties[1].Points = append([]Point(nil), a.Properties[1].Points...)
	a.Properties[1].Points[0].Description = json.RawMessage(`{"mpan":"synthetic-import-a","is_export":false,"meters":[{"serial_number":"a"}],"profile_class":2}`)
	if Select(a, Choice{"synthetic-import-a", "a"}, Choice{}).Import.State != "conflict" {
		t.Fatal("conflicting description")
	}
}
