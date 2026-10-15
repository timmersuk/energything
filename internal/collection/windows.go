package collection

import (
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"time"
)

// Historical exploration is deliberately labelled a limit, not an archive bound.
// Detailed retention evidence is installation-scoped (~95 days, 2026-10-15).
const SearchDays = 120

// UnitSpec is shared by automatic planning and exceptional range repair.
func UnitSpec(i storage.Identity) (kind string, days int) {
	if i.Provider == "octopus" {
		return "history", 14
	}
	if i.Stream == "generation" {
		return "aggregate", 7
	}
	return "history", 1
}

func plans(i storage.Identity, c config.Config, now time.Time) []storage.Schedule {
	day := time.Date(now.In(c.Timezone).Year(), now.In(c.Timezone).Month(), now.In(c.Timezone).Day(), 0, 0, 0, 0, time.UTC)
	kind, _ := UnitSpec(i)
	interval := c.Octopus.RefreshInterval
	if i.Provider == "growatt" {
		interval = c.Growatt.HistoryInterval
		if i.Stream == "generation" {
			kind = "aggregate"
			interval = 24 * time.Hour
		}
	}
	if interval <= 0 {
		interval = time.Hour
	}
	end := day.AddDate(0, 0, 1)
	if i.Provider == "octopus" {
		end = now.UTC().Truncate(30 * time.Minute)
	}
	base := storage.Schedule{IdentityID: i.ID, Kind: kind, Policy: "routine", Interval: interval, Frontier: storage.At(end), SearchLimit: storage.At(day.AddDate(0, 0, -SearchDays)), Provenance: "120-day bounded exploration; earliest provider boundary unresolved; empties and zeros do not establish absence"}
	out := []storage.Schedule{base}
	if i.Provider == "growatt" && i.Stream == "telemetry" {
		for slot := 1; slot <= 2; slot++ {
			p := base
			p.Slot = slot
			out = append(out, p)
		}
		p := base
		p.Kind = "latest"
		p.Interval = c.Growatt.LatestInterval
		if p.Interval <= 0 {
			p.Interval = 5 * time.Minute
		}
		out = append(out, p)
	}
	base.Policy = "backfill"
	out = append(out, base)
	return out
}

func plannedWindow(p storage.Schedule, i storage.Identity, now time.Time, loc *time.Location) storage.Job {
	end := time.UnixMicro(int64(p.Frontier)).UTC()
	var start time.Time
	if p.Policy == "routine" {
		local := now.In(loc)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		if i.Provider == "octopus" {
			end = now.UTC().Truncate(30 * time.Minute)
			start = end.Add(-7 * 24 * time.Hour)
		} else {
			start = day.AddDate(0, 0, -p.Slot)
			end = start.AddDate(0, 0, 1)
			if p.Kind == "aggregate" {
				start = end.AddDate(0, 0, -7)
			}
		}
	} else {
		_, days := UnitSpec(i)
		start = end.AddDate(0, 0, -days)
		limit := time.UnixMicro(int64(p.SearchLimit))
		if start.Before(limit) {
			start = limit
		}
	}
	date := ""
	if i.Provider == "growatt" {
		date = start.Format("2006-01-02")
	}
	// Growatt bounds are provider-date bookkeeping; clocks remain qualified.
	return storage.Job{Start: storage.At(start), End: storage.At(end), ProviderDate: date}
}
