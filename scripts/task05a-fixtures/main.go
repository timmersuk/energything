package main

import (
	"context"
	"fmt"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"os"
	"path/filepath"
)

func main() {
	dir, e := os.MkdirTemp(".cache", "task05a-browser-")
	must(e)
	s, e := storage.Open(context.Background(), dir)
	must(e)
	must(testsupport.SeedBrowsing(context.Background(), s))
	must(testsupport.SeedAggregates(context.Background(), s))
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
