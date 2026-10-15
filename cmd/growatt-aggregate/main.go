// growatt-aggregate performs one bounded daily-generation window against a
// stopped/private database. Default window: seven inclusive dates, capped today.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("data-dir", "", "Stopped/private database directory; never a live server database")
	day := flag.String("day", "", "One provider calendar date YYYY-MM-DD")
	from := flag.String("from", "", "First provider date YYYY-MM-DD; defaults to a seven-date window")
	to := flag.String("to", "", "Optional inclusive last date; at most seven dates from -from")
	discovery := flag.String("discovery", "", "Existing private confirmed Growatt discovery report")
	flag.Parse()
	start, end, e := aggregateRange(*day, *from, *to, time.Now())
	if e != nil || *dir == "" || *discovery == "" || os.Getenv("GROWATT_API_TOKEN") == "" {
		return fmt.Errorf("require token, -data-dir, -discovery and nonfuture -from [-to] (one through seven dates), or -day alone")
	}
	b, e := os.ReadFile(*discovery)
	if e != nil {
		return fmt.Errorf("private confirmed discovery unavailable")
	}
	var report growatt.Discovery
	if json.Unmarshal(b, &report) != nil || report.Selected == nil || report.Selected.PlantID == "" {
		return fmt.Errorf("private confirmed plant selection unavailable")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	db, e := storage.Open(ctx, *dir)
	if e != nil {
		return fmt.Errorf("aggregate storage unavailable")
	}
	defer db.Close()
	plant := report.Selected.PlantID
	ids, e := db.UpsertIdentities(ctx, []storage.Identity{{Provider: "growatt", Stream: "generation", StableKey: plant, Selected: true, Metadata: storage.DiscoveryMetadata{PlantID: plant}}})
	if e != nil {
		return fmt.Errorf("plant identity unavailable")
	}
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(start), End: storage.At(end), ProviderDate: start.Format("2006-01-02"), NextDue: storage.At(time.Now())}})
	if e != nil {
		return fmt.Errorf("aggregate unit unavailable")
	}
	if e = db.ResumeJob(ctx, jobs[0].ID, storage.At(time.Now())); e != nil {
		return fmt.Errorf("aggregate unit could not resume")
	}
	if e = collection.CollectGrowattAggregateUnit(ctx, db, growatt.NewClient(os.Getenv("GROWATT_API_TOKEN"), nil), jobs[0], plant); e != nil {
		return e
	}
	fmt.Println("Bounded daily aggregate window accepted; private values retained in local storage")
	return nil
}

func aggregateRange(day, from, to string, now time.Time) (time.Time, time.Time, error) {
	if day != "" {
		if from != "" || to != "" {
			return time.Time{}, time.Time{}, storage.ErrInvalid
		}
		from, to = day, day
	}
	if from == "" {
		return time.Time{}, time.Time{}, storage.ErrInvalid
	}
	if to == "" {
		start, err := time.Parse("2006-01-02", from)
		if err != nil {
			return time.Time{}, time.Time{}, storage.ErrInvalid
		}
		to = start.AddDate(0, 0, 6).Format("2006-01-02")
		if today := now.UTC().Format("2006-01-02"); to > today {
			to = today
		}
	}
	start, end, err := storage.AggregateDateBounds(from, to)
	if err != nil || end.AddDate(0, 0, -1).After(now.UTC()) {
		return time.Time{}, time.Time{}, storage.ErrInvalid
	}
	return start, end, nil
}
