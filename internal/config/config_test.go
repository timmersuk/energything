package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfiguration(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name, key, value string
		fatal            bool
		state            string
	}{
		{"empty sources", "", "", false, "disabled"},
		{"missing username", "ENERGYTHING_USERNAME", "", true, ""},
		{"missing password", "ENERGYTHING_PASSWORD", "", true, ""},
		{"invalid address", "ENERGYTHING_HTTP_ADDR", "secret-invalid", true, ""},
		{"invalid timezone", "ENERGYTHING_TIMEZONE", "secret-invalid", true, ""},
		{"invalid timeout", "ENERGYTHING_SHUTDOWN_TIMEOUT", "0s", true, ""},
		{"partial source", "OCTOPUS_API_KEY", "secret-invalid", false, "error"},
		{"bad interval", "OCTOPUS_REFRESH_INTERVAL", "secret-invalid", false, "error"},
		{"future backfill", "OCTOPUS_BACKFILL_START", "2099-01-01", false, "disabled"},
		{"bad date", "OCTOPUS_BACKFILL_START", "secret-invalid", false, "disabled"},
		{"unpaired selector", "OCTOPUS_IMPORT_MPAN", "secret-invalid", false, "error"},
		{"unpaired range", "OCTOPUS_RANGE_REFRESH_START", "2026-10-01", false, "disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"ENERGYTHING_USERNAME": "test-user", "ENERGYTHING_PASSWORD": "test-password"}
			if tc.key != "" {
				env[tc.key] = tc.value
			}
			c, err := Parse(func(k string) string { return env[k] }, now)
			if (err != nil) != tc.fatal {
				t.Fatalf("error=%v", err)
			}
			if err != nil {
				if strings.Contains(err.Error(), "secret-invalid") {
					t.Fatal("secret leaked")
				}
				return
			}
			if c.Octopus.State != tc.state {
				t.Fatalf("state=%s", c.Octopus.State)
			}
			if strings.Contains(c.Octopus.Message, "secret-invalid") {
				t.Fatal("secret leaked")
			}
			if c.Growatt.State != "disabled" {
				t.Fatal("other source affected")
			}
		})
	}
}
func TestValidProvidersAndRanges(t *testing.T) {
	env := map[string]string{"ENERGYTHING_USERNAME": "test", "ENERGYTHING_PASSWORD": "test", "OCTOPUS_API_KEY": "test", "OCTOPUS_ACCOUNT_NUMBER": "test", "GROWATT_API_TOKEN": "test", "OCTOPUS_RANGE_REFRESH_START": "2026-10-01", "OCTOPUS_RANGE_REFRESH_END": "2026-10-14", "ENERGYTHING_RANGE_REFRESH_ID": "synthetic"}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		change func()
		state  string
	}{
		{"valid", func() {}, "valid"},
		{"reversed", func() {
			env["OCTOPUS_RANGE_REFRESH_START"] = "2026-10-14"
			env["OCTOPUS_RANGE_REFRESH_END"] = "2026-10-01"
		}, "valid"},
		{"missing ID", func() {
			env["ENERGYTHING_RANGE_REFRESH_ID"] = ""
			env["OCTOPUS_RANGE_REFRESH_START"] = "2026-10-01"
			env["OCTOPUS_RANGE_REFRESH_END"] = "2026-10-14"
		}, "valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			c, err := Parse(func(k string) string { return env[k] }, now)
			if err != nil {
				t.Fatal(err)
			}
			if c.Octopus.State != tc.state || c.Growatt.State != "valid" {
				t.Fatalf("states %s %s", c.Octopus.State, c.Growatt.State)
			}
		})
	}
}
func TestIncompleteSelectionDoesNotDisableOtherStream(t *testing.T) {
	env := map[string]string{"ENERGYTHING_USERNAME": "synthetic", "ENERGYTHING_PASSWORD": "synthetic", "OCTOPUS_API_KEY": "synthetic", "OCTOPUS_ACCOUNT_NUMBER": "synthetic", "OCTOPUS_IMPORT_MPAN": "synthetic", "ENERGYTHING_OCTOPUS_ONCE": "true"}
	c, err := Parse(func(k string) string { return env[k] }, time.Now())
	if err != nil || c.Octopus.State != "valid" {
		t.Fatal(c.Octopus.State, err)
	}
	env["ENERGYTHING_OCTOPUS_ONCE"] = "PRIVATE"
	_, err = Parse(func(k string) string { return env[k] }, time.Now())
	if err != nil {
		t.Fatal(err)
	}
}
