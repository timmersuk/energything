// enqueue registers exceptional repair work. It never fetches or recovers attempts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/storage"
	"os"
	"time"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(args []string) error {
	f := flag.NewFlagSet("enqueue", flag.ContinueOnError)
	dir := f.String("data-dir", "", "Existing server data directory")
	identity := f.Int64("identity", 0, "Confirmed selected local identity ID")
	from := f.String("from", "", "First inclusive date (UTC for Octopus; provider date for Growatt)")
	to := f.String("to", "", "Last inclusive date")
	operation := f.String("operation-id", "", "Immutable operation ID")
	if e := f.Parse(args); e != nil {
		return e
	}
	first, e1 := time.Parse("2006-01-02", *from)
	last, e2 := time.Parse("2006-01-02", *to)
	if f.NArg() != 0 || *dir == "" || *identity <= 0 || *operation == "" || e1 != nil || e2 != nil || first.After(last) || last.After(time.Now().UTC()) || last.Sub(first) > 30*24*time.Hour {
		return errors.New("require existing data-dir, selected identity, operation-id and paired inclusive dates spanning at most 31 dates")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, e := storage.OpenQueue(ctx, *dir)
	if e != nil {
		return e
	}
	defer db.Close()
	ids, e := db.Identities(ctx)
	if e != nil {
		return e
	}
	var selected *storage.Identity
	for _, i := range ids {
		if i.ID == *identity && i.Selected {
			v := i
			selected = &v
		}
	}
	if selected == nil {
		return errors.New("confirmed selected identity not found")
	}
	kind, days := "history", 1
	if selected.Provider == "octopus" {
		days = 14
	}
	if selected.Stream == "generation" {
		kind = "aggregate"
		days = 7
	}
	jobs := []storage.Job{}
	end := last.AddDate(0, 0, 1)
	for start := first; start.Before(end); {
		stop := start.AddDate(0, 0, days)
		if stop.After(end) {
			stop = end
		}
		date := ""
		if selected.Provider == "growatt" {
			date = start.Format("2006-01-02")
		}
		jobs = append(jobs, storage.Job{IdentityID: selected.ID, Kind: kind, Policy: "range", Start: storage.At(start), End: storage.At(stop), ProviderDate: date, NextDue: storage.At(time.Now())})
		start = stop
	}
	out, e := db.EnsureRange(ctx, *operation, jobs)
	if e != nil {
		return e
	}
	fmt.Printf("Enqueued %d durable repair units; the server performs collection.\n", len(out))
	return nil
}
