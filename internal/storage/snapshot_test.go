package storage

import (
	"context"
	"database/sql"
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
