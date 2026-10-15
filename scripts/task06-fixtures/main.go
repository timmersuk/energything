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
