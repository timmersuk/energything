package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlowSnapshotAllowsCommitAndReadAndReleasesWAL(t *testing.T) {
	s, dir, _, job := fixture(t, "octopus")
	accepted(t, s, job, "0.100")
	// Inflate a synthetic table to guarantee a long stepped snapshot.
	if _, e := s.db.Exec(`CREATE TABLE snapshot_padding(id INTEGER PRIMARY KEY, payload BLOB); INSERT INTO snapshot_padding(payload) VALUES(zeroblob(4194304))`); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "copy.sqlite")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.snapshot(ctx, out, 8, 2*time.Millisecond) }()
	// Wait for destination creation; transaction pin follows immediately.
	for i := 0; i < 100; i++ {
		if _, e := os.Stat(out); e == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)
	for i := 0; i < 10; i++ {
		if _, e := s.db.ExecContext(ctx, `INSERT INTO snapshot_padding(payload) VALUES(zeroblob(4096))`); e != nil {
			t.Fatal(e)
		}
		if _, e := s.QueryIntervals(ctx, RecordQuery{}); e != nil {
			t.Fatal(e)
		}
	}
	select {
	case e := <-done:
		t.Fatalf("backup finished before concurrent commits: %v", e)
	default:
	}
	wal, e := os.Stat(filepath.Join(dir, "energything.sqlite-wal"))
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("pinned WAL bytes after ten concurrent writes: %d", wal.Size())
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if _, e = ValidateSnapshot(ctx, out); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); e != nil {
		t.Fatal(e)
	}
	wal, e = os.Stat(filepath.Join(dir, "energything.sqlite-wal"))
	if e != nil || wal.Size() != 0 {
		t.Fatal("read pin leaked", e)
	}
	db, e := sql.Open("sqlite", snapshotURI(out, "immutable=1&mode=ro"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var n int
	if e = db.QueryRow("SELECT count(*) FROM intervals").Scan(&n); e != nil || n != 2 {
		t.Fatal(n, e)
	}
}

func TestSnapshotCancellationAndQuotaRestartHold(t *testing.T) {
	s, _, _, _ := fixture(t, "octopus")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.Snapshot(ctx, filepath.Join(t.TempDir(), "cancel.sqlite")); e == nil {
		t.Fatal("ignored cancellation")
	}
	now := time.Now().UTC()
	for _, daily := range []bool{false, true} {
		s.HoldAggregateCalls(now, daily)
		due, e := s.ReserveAggregateCallWithDailyCap(context.Background(), At(now), daily)
		want := 5 * time.Minute
		if daily {
			want = 24 * time.Hour
		}
		if e != nil || !due.Equal(now.Add(want)) {
			t.Fatal(due, e)
		}
	}
}

func TestSnapshotDeadlineReleasesPinnedReadAndUnsupportedSchema(t *testing.T) {
	s, _, _, _ := fixture(t, "octopus")
	if _, e := s.db.Exec(`CREATE TABLE padding(payload BLOB); INSERT INTO padding VALUES(zeroblob(4194304))`); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	e := s.snapshot(ctx, filepath.Join(t.TempDir(), "cancel.sqlite"), 1, 2*time.Millisecond)
	if !errors.Is(e, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatal("unbounded/ignored cancellation", e)
	}
	var busy, log, checkpointed int
	if e = s.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &checkpointed); e != nil || busy != 0 {
		t.Fatal("cancelled snapshot leaked read pin", e)
	}
	if _, e = s.db.Exec("INSERT INTO schema_migrations VALUES(9)"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "future.sqlite")
	if e = s.Snapshot(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if _, e = ValidateSnapshot(context.Background(), path); e == nil {
		t.Fatal("accepted future schema")
	}
}

func TestSnapshotPreservesOldSchemaForNormalUpgrade(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "energything.sqlite")
	db, e := sql.Open("sqlite", snapshotURI(source, ""))
	if e != nil {
		t.Fatal(e)
	}
	if e = migrate(ctx, db, legacyScripts(t)[:7], ""); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	s, e := SnapshotSource(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	dest := t.TempDir()
	path := filepath.Join(dest, "energything.sqlite")
	if e = s.Snapshot(ctx, path); e != nil {
		t.Fatal(e)
	}
	if v, e := ValidateSnapshot(ctx, path); e != nil || v != 7 {
		t.Fatal(v, e)
	}
	upgraded, e := Open(ctx, dest)
	if e != nil {
		t.Fatal(e)
	}
	defer upgraded.Close()
	var v int
	if e = upgraded.db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&v); e != nil || v != 8 {
		t.Fatal(v, e)
	}
}

func TestSnapshotBusySourceIsBounded(t *testing.T) {
	s, _, _, _ := fixture(t, "octopus")
	if _, e := s.db.Exec("PRAGMA journal_mode=DELETE"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.db.Exec("BEGIN EXCLUSIVE"); e != nil {
		t.Fatal(e)
	}
	defer s.db.Exec("ROLLBACK")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	if e := s.Snapshot(ctx, filepath.Join(t.TempDir(), "busy.sqlite")); e == nil {
		t.Fatal("ignored locked source")
	}
	if time.Since(started) > time.Second {
		t.Fatal("lock wait exceeded bound")
	}
}
