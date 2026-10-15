package main

import (
	"testing"
	"time"
)

func TestAggregateRange(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		day, from, to, end string
		valid              bool
	}{
		{"", "2026-01-12", "", "2026-01-19", true},
		{"", "2026-01-12", "2026-01-14", "2026-01-15", true},
		{"", "2026-10-13", "", "2026-10-16", true},
		{"2026-01-15", "", "", "2026-01-16", true},
		{"2026-01-15", "2026-01-15", "", "", false},
		{"", "2026-01-12", "2026-01-19", "", false},
		{"", "2026-10-16", "", "", false},
		{"", "2026-10-13", "2026-10-16", "", false},
		{"", "", "2026-01-12", "", false},
	} {
		start, end, e := aggregateRange(tc.day, tc.from, tc.to, now)
		if (e == nil) != tc.valid {
			t.Fatal(tc, e)
		}
		if tc.valid && (end.Format("2006-01-02") != tc.end || !end.After(start)) {
			t.Fatal(tc, start, end)
		}
	}
}
