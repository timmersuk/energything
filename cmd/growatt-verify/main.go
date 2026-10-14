// growatt-verify is an explicit bounded verification path, not a scheduler.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/growatt"
	"github.com/timmersuk/energything/internal/storage"
	"os"
	"os/signal"
	"path/filepath"
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
	date := flag.String("day", "", "One provider date YYYY-MM-DD; optional history verification")
	report := flag.String("report", "", "Required private JSON report path (identifiers included)")
	collect := flag.Bool("collect", false, "Collect latest and optional day into the private database")
	dir := flag.String("data-dir", "", "Private verification database directory; never the live server directory")
	zone := flag.String("timezone", "Europe/London", "Provider date display timezone")
	flag.Parse()
	if *report == "" || (*collect && *dir == "") {
		return fmt.Errorf("specify -report and, for collection, a separate -data-dir")
	}
	location, e := time.LoadLocation(*zone)
	if e != nil {
		return fmt.Errorf("invalid provider timezone")
	}
	var from, to time.Time
	if *date != "" {
		from, to, e = growatt.DayBounds(*date, location)
		if e != nil || *date > time.Now().In(location).Format("2006-01-02") {
			return fmt.Errorf("day must be a non-future YYYY-MM-DD")
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, deadline := context.WithTimeout(ctx, 15*time.Minute)
	defer deadline()
	client := growatt.NewClient(os.Getenv("GROWATT_API_TOKEN"), nil)
	var d growatt.Discovery
	var identities []storage.Identity
	var db *storage.Store
	if *collect {
		db, e = storage.Open(ctx, *dir)
		if e != nil {
			return fmt.Errorf("private storage open failed")
		}
		defer db.Close()
		d, identities, e = collection.DiscoverGrowatt(ctx, db, client, os.Getenv("GROWATT_PLANT_ID"), os.Getenv("GROWATT_DEVICE_SERIAL"))
	} else {
		d, e = client.Discover(ctx, os.Getenv("GROWATT_PLANT_ID"), os.Getenv("GROWATT_DEVICE_SERIAL"))
	}
	if e != nil {
		return e
	}
	// The human opted into this private file; no identifiers go to ordinary stdout.
	if e = os.MkdirAll(filepath.Dir(*report), 0700); e != nil {
		return fmt.Errorf("private report directory unavailable")
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	if e = os.WriteFile(*report, b, 0600); e != nil {
		return fmt.Errorf("private report write failed")
	}
	if d.Selected == nil {
		return fmt.Errorf("Growatt selection unavailable; inspect private report and set plant/device selection")
	}
	if !*collect {
		fmt.Println("Private discovery report saved")
		return nil
	}
	var identity storage.Identity
	for _, v := range identities {
		if v.Selected {
			identity = v
		}
	}
	// Unresolved latest time needs bounds that do not pretend it was measured now.
	now := time.Now()
	jobs := []storage.Job{{IdentityID: identity.ID, Kind: "latest", Policy: "routine", Start: storage.At(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)), End: storage.At(time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)), NextDue: storage.At(now)}}
	if *date != "" {
		jobs = append(jobs, storage.Job{IdentityID: identity.ID, Kind: "history", Policy: "backfill", Start: storage.At(from), End: storage.At(to), ProviderDate: *date, NextDue: storage.At(now)})
	}
	jobs, e = db.EnsureJobs(ctx, jobs)
	if e != nil {
		return fmt.Errorf("private verification work unavailable")
	}
	var result error
	for _, job := range jobs {
		if e = db.ResumeJob(ctx, job.ID, storage.At(time.Now())); e != nil {
			return fmt.Errorf("private verification work could not resume")
		}
		result = errors.Join(result, collection.CollectGrowattUnit(ctx, db, client, job, *d.Selected, *zone))
	}
	if result != nil {
		return result
	}
	fmt.Println("Private latest/history verification accepted; inspect private database for details")
	return nil
}
