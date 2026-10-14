package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteWriteReopen(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "data & #")
	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	check := func(db *sql.DB) {
		t.Helper()
		for i := 0; i < 3; i++ {
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var journal string
			var foreign, busy int
			if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreign); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
				t.Fatal(err)
			}
			if journal != "wal" || foreign != 1 || busy != 5000 {
				t.Fatalf("pragmas: %s %d %d", journal, foreign, busy)
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			db.SetMaxIdleConns(0) // Force replacement physical connections.
		}
	}
	check(db.db)
	if _, err := db.db.Exec("CREATE TABLE scaffold_probe (value TEXT NOT NULL); INSERT INTO scaffold_probe VALUES ('synthetic-value')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	check(db.db)
	var value string
	if err := db.db.QueryRow("SELECT value FROM scaffold_probe").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "synthetic-value" {
		t.Fatalf("reopened %q", value)
	}
	t.Log("SQLite WAL/foreign_keys/busy_timeout and write/close/reopen passed")
}
func TestInvalidDataDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	if db, err := Open(context.Background(), path); err == nil {
		db.Close()
		t.Fatal("accepted a file as data directory")
	}
}
func TestApplicationSchema(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("missing application tables: %d", count)
	}
}
