package growatt

import (
	"github.com/timmersuk/energything/internal/storage"
	"testing"
	"time"
)

func TestQualifiedClockDoesNotInventIdentityOrDST(t *testing.T) {
	for _, tc := range []struct {
		wall    string
		count   int
		quality string
	}{
		{"2026-10-15 12:00:00", 1, "regional_clock_candidate_seasonal_unverified"},
		{"2026-01-15 12:00:00", 1, "regional_clock_candidate_seasonal_unverified"},
		{"2026-03-29 01:30:00", 0, "regional_clock_gap"},
		{"2026-10-25 01:30:00", 2, "regional_clock_overlap_ambiguous"},
	} {
		wall, _ := time.Parse("2006-01-02 15:04:05", tc.wall)
		loc, _ := time.LoadLocation("Europe/London")
		if got := ClockCandidates(wall, loc); len(got) != tc.count {
			t.Fatal(tc, got)
		}
		s := storage.Sample{Quality: "unresolved_time", OriginalTime: map[string]string{"time": `"` + tc.wall + `"`, "calendar": `{"timeInMillis":0}`}}
		QualifyClock(&s, "Europe/London")
		if s.Quality != tc.quality || s.Instant != nil || s.OriginalTime["calendar"] == "" {
			t.Fatal(tc, s)
		}
	}
}
