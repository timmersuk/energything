package storage_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/web"
)

func TestAuthenticatedBrowsingAndCommitDuringOnlineSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	s, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	// Synthetic padding forces at least 128 page-step pauses in the production
	// snapshot path. This SQL connection is closed before concurrent activity.
	raw, e := sql.Open("sqlite", filepath.Join(dir, "energything.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = raw.ExecContext(ctx, `CREATE TABLE snapshot_padding(payload BLOB); INSERT INTO snapshot_padding VALUES(zeroblob(67108864))`)
	closeErr := raw.Close()
	if e != nil || closeErr != nil {
		t.Fatal(e, closeErr)
	}
	handler, e := web.NewRouter(config.Config{Username: "synthetic-user", Password: "synthetic-password", Timezone: time.UTC}, "synthetic", s)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "snapshot.sqlite")
	done := make(chan error, 1)
	go func() { done <- s.Snapshot(ctx, path) }()
	for {
		if _, e = os.Stat(path); e == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil).WithContext(ctx)
	request.SetBasicAuth("synthetic-user", "synthetic-password")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if _, e = s.ReserveAggregateCall(ctx, storage.At(time.Now())); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-done:
		t.Fatal("snapshot completed before authenticated read and write", e)
	default:
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if _, e = storage.ValidateSnapshot(ctx, path); e != nil {
		t.Fatal(e)
	}
}
