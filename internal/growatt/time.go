package growatt

import (
	"encoding/json"
	"github.com/timmersuk/energything/internal/storage"
	"sort"
	"time"
)

// QualifyClock retains explicit candidate interpretations, never an invented
// measurement key. Regional versus fixed UTC+1 remains unresolved in winter.
// Calendar epochs and createTime are not independent measurement-time evidence.
func QualifyClock(s *storage.Sample, zone string) {
	if s.Quality == "contradictory_time" {
		return
	}
	var clock string
	if json.Unmarshal([]byte(s.OriginalTime["time"]), &clock) != nil {
		return
	}
	wall, e := time.Parse("2006-01-02 15:04:05", clock)
	if e != nil {
		return
	}
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return
	}
	candidates := ClockCandidates(wall, loc)
	values := []string{}
	for _, t := range candidates {
		values = append(values, t.Format(time.RFC3339))
	}
	b, _ := json.Marshal(values)
	s.OriginalTime["regional_candidates_utc"] = string(b)
	s.OriginalTime["regional_candidate_zone"] = zone
	s.OriginalTime["fixed_utc_plus_one_candidate"] = wall.Add(-time.Hour).Format(time.RFC3339)
	s.OriginalTime["time_policy"] = "readable clock candidates; current installation BST supported by 2026-10-15 comparison; seasonal behavior and stable identity unverified; calendar/createTime not measurement anchors"
	switch len(candidates) {
	case 0:
		s.Quality = "regional_clock_gap"
	case 1:
		s.Quality = "regional_clock_candidate_seasonal_unverified"
	default:
		s.Quality = "regional_clock_overlap_ambiguous"
	}
}

// ClockCandidates validates round trips so Go cannot silently pick an overlap
// occurrence or shift a nonexistent wall clock. No upload-delay bound is assumed.
func ClockCandidates(wall time.Time, loc *time.Location) []time.Time {
	offsets := map[int]bool{}
	for h := -48; h <= 48; h += 6 {
		_, offset := wall.Add(time.Duration(h) * time.Hour).In(loc).Zone()
		offsets[offset] = true
	}
	out := []time.Time{}
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		if candidate.In(loc).Format("2006-01-02 15:04:05") == wall.Format("2006-01-02 15:04:05") {
			out = append(out, candidate)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}
