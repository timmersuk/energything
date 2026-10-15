package web

import (
	"context"
	"encoding/json"
	"github.com/timmersuk/energything/internal/testsupport"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAggregateRoutesFiltersPaging(t *testing.T) {
	s, h := browsingFixture(t, true)
	if e := testsupport.SeedAggregates(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/records/growatt/aggregates", "/api/v1/growatt/aggregates"} {
		for _, auth := range []string{"", "wrong"} {
			if w := request(h, path, auth); w.Code != 401 {
				t.Fatal(w.Code)
			}
		}
		w := request(h, path, "synthetic")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code, w.Body)
		}
		r := httptest.NewRequest("POST", path, nil)
		r.SetBasicAuth("synthetic", "synthetic")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 405 {
			t.Fatal(w.Code)
		}
	}
	path := "/api/v1/growatt/aggregates?from=2026-01-15&to=2026-01-17&limit=2"
	seen := map[string]bool{}
	for n := 0; n < 3; n++ {
		w := request(h, path, "synthetic")
		var v struct {
			Records    []struct{ PlantID, Period, Value, Unit, Calendar, RetrievedAt string }
			NextCursor string `json:"next_cursor"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil {
			t.Fatal(w.Body)
		}
		var decoded map[string]any
		json.Unmarshal(w.Body.Bytes(), &decoded)
		for _, raw := range decoded["records"].([]any) {
			r := raw.(map[string]any)
			key := r["plant_id"].(string) + r["period"].(string)
			if seen[key] || r["unit"] != "kWh" || r["retrieved_at"] == nil {
				t.Fatal(r)
			}
			seen[key] = true
		}
		if n < 2 && v.NextCursor == "" {
			t.Fatal("missing cursor")
		}
		if n == 2 && v.NextCursor != "" {
			t.Fatal("extra cursor")
		}
		if n == 0 {
			bad := request(h, "/api/v1/growatt/aggregates?limit=2&plant_id=SYN-PLANT-A&cursor="+url.QueryEscape(v.NextCursor), "synthetic")
			if bad.Code != 400 {
				t.Fatal("cursor filter mismatch")
			}
		}
		path = "/api/v1/growatt/aggregates?from=2026-01-15&to=2026-01-17&limit=2&cursor=" + url.QueryEscape(v.NextCursor)
	}
	if len(seen) != 6 {
		t.Fatal(seen)
	}
	for _, q := range []string{"limit=501", "limit=0", "from=bad", "to=2026-01-01&from=2026-01-02", "resolution=month", "metric=export", "cursor=bad", "plant_id=a&plant_id=b", "unknown=x"} {
		w := request(h, "/api/v1/growatt/aggregates?"+q, "synthetic")
		if w.Code != 400 {
			t.Fatal(q, w.Code)
		}
	}
	w := request(h, "/records/growatt/aggregates?plant_id=SYN-PLANT-A", "synthetic")
	body := w.Body.String()
	for _, want := range []string{"1.234567890123456789", "SYN-PLANT-A", "reported_zero_availability_unknown", "calendar timezone is unverified", "<details>"} {
		if !strings.Contains(body, want) {
			t.Fatal("missing", want)
		}
	}
	if strings.Contains(body, "SYN-PLANT-B") {
		t.Fatal("plant filter")
	}
	if w = request(h, "/records/octopus", "synthetic"); w.Code != 200 {
		t.Fatal("existing provider unavailable")
	}
}
