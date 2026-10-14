// Creates NEW synthetic databases only; cannot target an existing/preview store.
package main

import (
	"context"
	"fmt"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/octopus"
	"github.com/timmersuk/energything/internal/storage"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func main() {
	root, err := os.MkdirTemp(".cache", "task04-browser-")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	for _, scenario := range []string{"selected", "ambiguous", "empty"} {
		db, err := storage.Open(ctx, filepath.Join(root, scenario))
		if err != nil {
			panic(err)
		}
		if scenario != "empty" {
			c := octopus.NewClient("synthetic", transport(func(r *http.Request) (*http.Response, error) {
				body := `{"properties":[{"electricity_meter_points":[{"mpan":"synthetic-import","is_export":false,"meters":[{"serial_number":"import"}]},{"mpan":"synthetic-export","is_export":true,"meters":[{"serial_number":"export"}]}]}]}`
				if scenario == "ambiguous" {
					body = strings.Replace(body, `"serial_number":"import"}`, `"serial_number":"import"},{"serial_number":"other"}`, 1)
				}
				if strings.Contains(r.URL.Path, "consumption") {
					body = `{"count":1,"next":null,"results":[{"consumption":0.100000000000000001,"interval_start":"2026-03-29T00:00:00Z","interval_end":"2026-03-29T00:30:00Z","quality":"synthetic"}]}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			from := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
			to := from.Add(23 * time.Hour)
			if err := collection.OctopusOnce(ctx, db, c, config.Provider{Account: "synthetic"}, from, to); err != nil {
				panic(err)
			}
		}
		if err := db.Close(); err != nil {
			panic(err)
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	fmt.Println(absolute)
}
