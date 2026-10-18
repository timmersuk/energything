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
	"time"
)

// Open applies pragmas on every connection using a constructed, escaped DSN.
// Store keeps SQL private; consumers use typed storage operations.
type Store struct {
	db            *sql.DB
	path          string
	aggregateHold time.Time
	now           func() time.Time
	// Test-only fault injection at transactional boundaries. Never exported.
	fault func(string) error
}

func (s *Store) Close() error { return s.db.Close() }

func Open(ctx context.Context, dir string) (*Store, error) {
	return open(ctx, dir, true)
}

// OpenQueue opens an existing server database without migration or orphan recovery.
// A repair CLI must never reconcile attempts owned by the running server.
func OpenQueue(ctx context.Context, dir string) (*Store, error) {
	return open(ctx, dir, false)
}

func open(ctx context.Context, dir string, server bool) (*Store, error) {
	if !server {
		if _, err := os.Stat(filepath.Join(dir, "energything.sqlite")); err != nil {
			return nil, ErrNotFound
		}
	}
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
	q.Set("_txlock", "immediate") // Serialize cross-process read/modify/write claims.
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
	s := &Store{db: db, path: path, now: time.Now}
	var initErr error
	if server {
		initErr = s.initialize(ctx)
	} else {
		var version int
		initErr = db.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&version)
		if initErr == nil && version != schemaVersion {
			initErr = errors.New("queue requires the current server schema")
		}
	}
	if initErr != nil {
		db.Close()
		return nil, initErr
	}
	return s, nil
}
