// Creates a new synthetic audit database; never opens retained preview data.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"github.com/timmersuk/energything/internal/web"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	serve := flag.Bool("serve", false, "Serve new isolated synthetic fixture on 127.0.0.1:8086 without any collectors")
	flag.Parse()
	dir, e := os.MkdirTemp(".cache", "task06a-browser-")
	must(e)
	s, e := storage.Open(context.Background(), dir)
	must(e)
	must(testsupport.SeedBrowsing(context.Background(), s))
	must(testsupport.SeedAggregates(context.Background(), s))
	must(testsupport.SeedAudit(context.Background(), s))
	must(testsupport.SeedGenerationProgress(context.Background(), s))
	must(s.Close())
	path, e := filepath.Abs(dir)
	must(e)
	fmt.Println(path)
	if *serve {
		s, e = storage.Open(context.Background(), dir)
		must(e)
		defer s.Close()
		loc, e := time.LoadLocation("Europe/London")
		must(e)
		h, e := web.NewRouter(config.Config{Username: "synthetic", Password: "synthetic", Timezone: loc}, "task06a-synthetic", s)
		must(e)
		must(http.ListenAndServe("127.0.0.1:8086", h))
	}
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
