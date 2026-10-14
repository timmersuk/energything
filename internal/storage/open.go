package storage

import (
	"context"
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Open applies pragmas on every connection using a constructed, escaped DSN.
// No application tables are created in task 01.
func Open(ctx context.Context, dir string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("ENERGYTHING_DATA_DIR cannot be created; grant the service directory access")
	}
	probe, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return nil, errors.New("ENERGYTHING_DATA_DIR must be writable by the service user (container UID 10001)")
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if closeErr != nil || removeErr != nil {
		return nil, errors.New("ENERGYTHING_DATA_DIR write check could not be cleaned up")
	}
	path, err := filepath.Abs(filepath.Join(dir, "energything.sqlite"))
	if err != nil {
		return nil, errors.New("ENERGYTHING_DATA_DIR path cannot be resolved")
	}
	slash := filepath.ToSlash(path)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	u := url.URL{Scheme: "file", Path: slash}
	q := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "foreign_keys(1)", "busy_timeout(5000)"} {
		q.Add("_pragma", p)
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, errors.New("SQLite initialization failed")
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("SQLite cannot open DATA_DIR/energything.sqlite; check directory and database permissions")
	}
	return db, nil
}
