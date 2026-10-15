// Synthetic browser storage, always separate from preview data.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"os"
	"path/filepath"
	"time"
)

func main() {
	existing := flag.String("reselect-data-dir", "", "Restore synthetic selected contexts after injected discovery outage")
	flag.Parse()
	ctx := context.Background()
	if *existing != "" {
		s, e := storage.OpenQueue(ctx, *existing)
		must(e)
		defer s.Close()
		ids, e := s.Identities(ctx)
		must(e)
		for n := range ids {
			ids[n].Selected = true
		}
		_, e = s.UpsertIdentities(ctx, ids)
		must(e)
		// Persist one synthetic calendar scan so browser acceptance can inspect
		// the expanding target even while provider discovery is unavailable.
		for _, i := range ids {
			if i.Stream != "generation" {
				continue
			}
			end := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)
			start := end.AddDate(0, 0, -7)
			_, e = s.PrepareSchedule(ctx, storage.Schedule{IdentityID: i.ID, Kind: "aggregate", Policy: "backfill", SearchPolicy: "calendar", Frontier: storage.At(end), SearchLimit: storage.At(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), Interval: 24 * time.Hour, Provenance: "Synthetic continuous calendar history"}, storage.At(time.Now()), false, func(storage.Schedule) storage.Job {
				return storage.Job{Start: storage.At(start), End: storage.At(end), ProviderDate: start.Format("2006-01-02")}
			})
			must(e)
			break
		}
		fmt.Println("Synthetic selected contexts restored for outage/refresh fixture")
		return
	}
	dir, e := os.MkdirTemp(".cache", "task06-browser-")
	must(e)
	s, e := storage.Open(ctx, dir)
	must(e)
	must(testsupport.SeedBrowsing(ctx, s))
	must(testsupport.SeedAggregates(ctx, s))
	must(s.Close())
	path, e := filepath.Abs(dir)
	must(e)
	fmt.Println(path)
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
