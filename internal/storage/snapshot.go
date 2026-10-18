package storage

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"modernc.org/sqlite"
)

func snapshotURI(path string, query string) string {
	path = strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: query}
	return u.String()
}

// Snapshot uses its own read-only connection, never the serialized write pool.
// A stable read transaction avoids repeated backup restarts under ongoing writes.
// WAL can grow for the bounded duration; rollback releases the pinned generation.
func (s *Store) Snapshot(ctx context.Context, destination string) error {
	return s.snapshot(ctx, destination, 128, time.Millisecond)
}

// SnapshotSource opens a deliberately copied, stopped legacy database without
// migrations or recovery. Callers must never supply the retained live directory.
func SnapshotSource(ctx context.Context, path string) (*Store, error) {
	if _, err := ValidateSnapshot(ctx, path); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func (s *Store) snapshot(ctx context.Context, destination string, pages int32, pause time.Duration) (err error) {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", snapshotURI(s.path, "mode=ro&_pragma=busy_timeout(100)"))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN"); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "ROLLBACK")
	var version int
	if err = conn.QueryRowContext(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	return conn.Raw(func(raw any) (err error) {
		b, err := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		}).NewBackup(snapshotURI(destination, "_pragma=busy_timeout(100)"))
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, b.Finish()) }()
		for {
			if err = ctx.Err(); err != nil {
				return err
			}
			more, e := b.Step(pages)
			if e != nil {
				var se *sqlite.Error
				if !errors.As(e, &se) || (se.Code()&255 != 5 && se.Code()&255 != 6) {
					return e
				}
			} else if !more {
				return nil
			}
			timer := time.NewTimer(pause)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	})
}

// ValidateSnapshot is read-only and performs neither migration nor recovery.
func ValidateSnapshot(ctx context.Context, path string) (int, error) {
	db, err := sql.Open("sqlite", snapshotURI(path, "mode=ro&immutable=1"))
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var result string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil || result != "ok" {
		return 0, errors.New("snapshot integrity check failed")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return 0, err
	}
	bad := rows.Next()
	err = errors.Join(rows.Err(), rows.Close())
	if bad || err != nil {
		return 0, errors.New("snapshot foreign key check failed")
	}
	var version, count int
	err = db.QueryRowContext(ctx, "SELECT coalesce(max(version),0),count(*) FROM schema_migrations").Scan(&version, &count)
	if err != nil || version < 1 || version > schemaVersion || count != version {
		return 0, errors.New("snapshot schema unsupported")
	}
	return version, nil
}
