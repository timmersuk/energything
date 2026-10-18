// Synthetic container lifecycle support. Never point this at retained data.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/timmersuk/energything/internal/backup"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: probe seed|snapshot|runtime synthetic-data-directory")
	}
	dir := os.Args[2]
	switch os.Args[1] {
	case "runtime":
		pool, err := x509.SystemCertPool()
		must(err)
		if len(pool.Subjects()) == 0 {
			panic("no TLS roots")
		}
		loc, err := time.LoadLocation("Europe/London")
		must(err)
		_, offset := time.Date(2026, 7, 1, 0, 0, 0, 0, loc).Zone()
		if offset != 3600 {
			panic("timezone transition missing")
		}
	case "seed":
		ctx := context.Background()
		runtime, err := os.MkdirTemp("", "synthetic-runtime-")
		must(err)
		defer os.RemoveAll(runtime)
		s, err := storage.Open(ctx, runtime)
		must(err)
		must(testsupport.SeedBrowsing(ctx, s))
		must(testsupport.SeedAggregates(ctx, s))
		must(testsupport.SeedAudit(ctx, s))
		must(testsupport.SeedGenerationProgress(ctx, s))
		_, err = s.ReserveAggregateCall(ctx, storage.At(time.Now()))
		must(err)
		// Browsing fixtures deliberately leave one attempt running. Complete its
		// startup recovery before recording the stable lifecycle comparison.
		_, err = s.RecoverInterruptedAttempts(ctx, storage.At(time.Now()))
		must(err)
		p := backup.Publisher{Dir: dir, Build: "synthetic-seed", Interval: time.Second, Timeout: time.Minute}
		must(p.Publish(ctx, s))
		must(s.Close())
	case "snapshot":
		// Read only an immutable published dump, including while the server runs.
		runtime, err := os.MkdirTemp("", "synthetic-restore-")
		must(err)
		defer os.RemoveAll(runtime)
		_, err = backup.Restore(context.Background(), filepath.Join(dir, backup.Latest), runtime)
		must(err)
		db, err := sql.Open("sqlite", "file:"+runtime+"/energything.sqlite?mode=ro")
		must(err)
		defer db.Close()
		var integrity string
		must(db.QueryRow("PRAGMA integrity_check").Scan(&integrity))
		if integrity != "ok" {
			panic("integrity failure")
		}
		fk, err := db.Query("PRAGMA foreign_key_check")
		must(err)
		if fk.Next() {
			panic("foreign key failure")
		}
		must(fk.Close())
		tables, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		must(err)
		var names []string
		for tables.Next() {
			var name string
			must(tables.Scan(&name))
			names = append(names, name)
		}
		must(tables.Err())
		must(tables.Close())
		for _, name := range names {
			rows, err := db.Query(`SELECT * FROM "` + strings.ReplaceAll(name, `"`, `""`) + `"`)
			must(err)
			cols, err := rows.Columns()
			must(err)
			var records []string
			for rows.Next() {
				values := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range values {
					ptrs[i] = &values[i]
				}
				must(rows.Scan(ptrs...))
				b, err := json.Marshal(values)
				must(err)
				records = append(records, string(b))
			}
			must(rows.Err())
			must(rows.Close())
			// Sort outside SQL so every column/type contributes without ordering assumptions.
			sort.Strings(records)
			b, err := json.Marshal(records)
			must(err)
			fmt.Printf("%s %d %x\n", name, len(records), sha256.Sum256(b))
		}
	default:
		panic("unknown mode")
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
