// growatt-aggregate performs one explicit daily unit against a stopped/private database.
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
	discovery := flag.String("discovery", "", "Existing private confirmed Growatt discovery report")
	flag.Parse()
	date, e := time.Parse("2006-01-02", *day)
	if e != nil || date.After(time.Now().UTC()) || date.Year() < 1 || *dir == "" || *discovery == "" || os.Getenv("GROWATT_API_TOKEN") == "" {
		return fmt.Errorf("require token, -data-dir, -discovery and one nonfuture -day")
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
	jobs, e := db.EnsureJobs(ctx, []storage.Job{{IdentityID: ids[0].ID, Kind: "aggregate", Policy: "backfill", Start: storage.At(date), End: storage.At(date.AddDate(0, 0, 1)), ProviderDate: *day, NextDue: storage.At(time.Now())}})
	if e != nil {
		return fmt.Errorf("aggregate unit unavailable")
	}
	if e = db.ResumeJob(ctx, jobs[0].ID, storage.At(time.Now())); e != nil {
		return fmt.Errorf("aggregate unit could not resume")
	}
	if e = collection.CollectGrowattAggregateUnit(ctx, db, growatt.NewClient(os.Getenv("GROWATT_API_TOKEN"), nil), jobs[0], plant); e != nil {
		return e
	}
	fmt.Println("Bounded daily aggregate accepted; private values retained in local storage")
	return nil
}
