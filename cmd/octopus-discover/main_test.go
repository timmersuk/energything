package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/timmersuk/energything/internal/octopus"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureSource struct {
	account    octopus.Account
	fetch      func(context.Context, octopus.Meter) (octopus.Window, error)
	calls      int
	accountErr error
}

func (f *fixtureSource) Account(context.Context, string) (octopus.Account, error) {
	return f.account, f.accountErr
}
func (f *fixtureSource) Fetch(ctx context.Context, m octopus.Meter, _, _ time.Time) (octopus.Window, error) {
	f.calls++
	return f.fetch(ctx, m)
}
func account(t *testing.T) *fixtureSource {
	t.Helper()
	var a octopus.Account
	if err := json.Unmarshal([]byte(`{"properties":[{"address_line_1":"PRIVATE ADDRESS","electricity_meter_points":[{"mpan":"synthetic-import","is_export":false,"meters":[{"serial_number":"old"},{"serial_number":"current"}]},{"mpan":"synthetic-export","is_export":true,"meters":[{"serial_number":"export"}]},{"mpan":"unknown","is_export":null,"meters":[{"serial_number":"unknown"}]}]}]}`), &a); err != nil {
		t.Fatal(err)
	}
	return &fixtureSource{account: a}
}

var end = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func readings(last time.Time) octopus.Window {
	return octopus.Window{Intervals: []octopus.Interval{{Start: last.Add(-octopus.Slot), End: last, KWh: "0"}}, Expected: 672}
}
func TestReportSteerAndPrivateFields(t *testing.T) {
	f := account(t)
	f.fetch = func(_ context.Context, m octopus.Meter) (octopus.Window, error) {
		if m.Serial == "old" {
			return octopus.Window{Expected: 672}, nil
		}
		return readings(end.Add(-time.Hour)), nil
	}
	r, err := investigate(context.Background(), f, "synthetic", end.Add(-octopus.MaxWindow), end)
	if err != nil || !r.Complete || f.calls != 3 || r.Unknown != 1 {
		t.Fatal(r, err, f.calls)
	}
	if r.Streams[0].Suggested == nil || r.Streams[0].Suggested.Serial != "current" || r.Streams[0].Candidates[0].Meter.Serial != "current" {
		t.Fatal("recent zero-energy readings should steer", r.Streams[0])
	}
	var b strings.Builder
	if err := render(&b, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OCTOPUS_IMPORT_SERIAL=current", "Tentative steer", "Only one confirmed identity", "0 / 672"} {
		if !strings.Contains(b.String(), want) {
			t.Fatal(want, b.String())
		}
	}
	for _, bad := range []string{"PRIVATE ADDRESS", `"consumption"`, "KWh", "API_KEY="} {
		if strings.Contains(b.String(), bad) {
			t.Fatal("private payload leaked", bad)
		}
	}
}
func TestAmbiguityErrorsAndOldReadingsNeverInferInactive(t *testing.T) {
	for _, scenario := range []string{"both_recent", "all_empty", "all_old", "network", "validation", "auth", "rate_limit", "unknown_error"} {
		t.Run(scenario, func(t *testing.T) {
			f := account(t)
			f.fetch = func(_ context.Context, m octopus.Meter) (octopus.Window, error) {
				if m.Direction == "export" {
					return readings(end), nil
				}
				switch scenario {
				case "both_recent":
					return readings(end), nil
				case "all_empty":
					return octopus.Window{Expected: 672}, nil
				case "all_old":
					return readings(end.Add(-72 * time.Hour)), nil
				default:
					if m.Serial == "current" {
						class := map[string]string{"network": "network", "validation": "validation", "auth": "authentication", "rate_limit": "rate_limit"}[scenario]
						if class == "" {
							return octopus.Window{}, errors.New("PRIVATE URL KEY PAYLOAD")
						}
						return octopus.Window{}, &octopus.Error{Class: class, Code: "unavailable"}
					}
					return readings(end), nil
				}
			}
			r, err := investigate(context.Background(), f, "synthetic", end.Add(-octopus.MaxWindow), end)
			if err != nil || r.Streams[0].Suggested != nil {
				t.Fatal(r, err)
			}
			if (scenario == "auth" || scenario == "rate_limit") && f.calls != 1 {
				t.Fatal("credential/rate storm", f.calls)
			}
			var b strings.Builder
			render(&b, r)
			if strings.Contains(b.String(), "PRIVATE") {
				t.Fatal("raw error leaked")
			}
		})
	}
}
func TestConflictsCancellationAndCandidateBudget(t *testing.T) {
	f := account(t)
	f.account.Properties = append(f.account.Properties, f.account.Properties[0])
	f.account.Properties[1].Points = append([]octopus.Point(nil), f.account.Properties[1].Points...)
	f.account.Properties[1].Points[0].Description = json.RawMessage(`{"changed":"synthetic"}`)
	f.fetch = func(context.Context, octopus.Meter) (octopus.Window, error) { return readings(end), nil }
	r, err := investigate(context.Background(), f, "synthetic", end.Add(-octopus.MaxWindow), end)
	if err != nil || r.Streams[0].Discovery != "conflict" || r.Streams[0].Suggested != nil || f.calls != 1 {
		t.Fatal(r, err, f.calls)
	}
	f = account(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = investigate(ctx, f, "synthetic", end.Add(-octopus.MaxWindow), end)
	if err != nil || r.Complete || f.calls != 0 || r.Streams[0].Suggested != nil {
		t.Fatal(r, err)
	}
	f = account(t)
	for i := 0; i < 21; i++ {
		point := f.account.Properties[0].Points[1]
		point.MPAN = "synthetic-" + strings.Repeat("x", i+1)
		f.account.Properties[0].Points = append(f.account.Properties[0].Points, point)
	}
	r, err = investigate(context.Background(), f, "synthetic", end.Add(-octopus.MaxWindow), end)
	if err != nil || r.Complete || f.calls != 0 {
		t.Fatal(r, err)
	}
}
func TestReportOutputNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "report.md")
	r := report{Generated: end, From: end.Add(-octopus.MaxWindow), To: end}
	name, err := writeReport(path, r)
	if err != nil || name != path {
		t.Fatal(name, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writeReport(path, r); err == nil {
		t.Fatal("overwrote file")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("changed report")
	}
}
func TestOptionsAndAccountFailureSanitized(t *testing.T) {
	var b strings.Builder
	get := func(string) string { return "" }
	if err := run(context.Background(), []string{"--help"}, get, &b); err != nil || !strings.Contains(b.String(), "Usage:") {
		t.Fatal(err, b.String())
	}
	for _, args := range [][]string{{"--days", "15"}, {"--timeout", "1h"}, {"--wat=PRIVATE"}, {}} {
		err := run(context.Background(), args, get, &b)
		if err == nil || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal(err)
		}
	}
	f := account(t)
	f.accountErr = errors.New("PRIVATE URL PAYLOAD")
	_, err := investigate(context.Background(), f, "synthetic", end.Add(-octopus.MaxWindow), end)
	if err == nil || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatal(err)
	}
}
