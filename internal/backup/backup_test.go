package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
)

func privateTemp(t *testing.T) string {
	t.Helper()
	path := t.TempDir()
	if e := os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	return path
}

func setup(t *testing.T) (*Publisher, *storage.Store) {
	t.Helper()
	s, e := storage.Open(context.Background(), privateTemp(t))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	p := &Publisher{Dir: privateTemp(t), Build: "synthetic", Interval: time.Second, Timeout: 10 * time.Second}
	return p, s
}
func TestPublishRestoreAllTablesAndImmutableOpenGeneration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("open-inode replacement is a Linux production contract; exercised in both platform containers")
	}
	p, s := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context, *storage.Store) error{testsupport.SeedBrowsing, testsupport.SeedAggregates, testsupport.SeedAudit, testsupport.SeedGenerationProgress} {
		if e := seed(ctx, s); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.ReserveAggregateCall(ctx, storage.At(time.Now())); e != nil {
		t.Fatal(e)
	}
	if e := p.Publish(ctx, s); e != nil {
		t.Fatal(e)
	}
	old, e := os.Open(filepath.Join(p.Dir, Latest))
	if e != nil {
		t.Fatal(e)
	}
	defer old.Close()
	// Multiple atomic replacements while an external reader holds an old inode.
	first := privateTemp(t)
	if _, e = Restore(ctx, filepath.Join(p.Dir, Latest), first); e != nil {
		t.Fatal(e)
	}
	reference := filepath.Join(privateTemp(t), "reference.sqlite")
	if e = s.Snapshot(ctx, reference); e != nil {
		t.Fatal(e)
	}
	compareDatabases(t, reference, filepath.Join(first, "energything.sqlite"))
	for i := 0; i < 3; i++ {
		if e = p.Publish(ctx, s); e != nil {
			t.Fatal(e)
		}
	}
	copied := filepath.Join(privateTemp(t), Latest)
	f, e := os.Create(copied)
	if e != nil {
		t.Fatal(e)
	}
	buf := make([]byte, 257)
	for {
		n, e := old.Read(buf)
		if n > 0 {
			if _, w := f.Write(buf[:n]); w != nil {
				t.Fatal(w)
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	restored := privateTemp(t)
	if _, e = Restore(ctx, copied, restored); e != nil {
		t.Fatal(e)
	}
	compareDatabases(t, filepath.Join(first, "energything.sqlite"), filepath.Join(restored, "energything.sqlite"))
}
func compareDatabases(t *testing.T, a, b string) {
	t.Helper()
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(a)+"?mode=ro&immutable=1")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec("ATTACH DATABASE ? AS restored", b); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if e != nil {
		t.Fatal(e)
	}
	var names []string
	for rows.Next() {
		var n string
		if e = rows.Scan(&n); e != nil {
			t.Fatal(e)
		}
		names = append(names, n)
	}
	rows.Close()
	for _, name := range names {
		var n int
		for _, pair := range [][2]string{{"main", "restored"}, {"restored", "main"}} {
			q := `SELECT count(*) FROM (SELECT * FROM ` + pair[0] + `."` + name + `" EXCEPT SELECT * FROM ` + pair[1] + `."` + name + `")`
			if e = db.QueryRow(q).Scan(&n); e != nil || n != 0 {
				t.Fatal(name, n, e)
			}
		}
	}
}
func TestPublicationFaultsKeepCompleteGenerationAndVisibleError(t *testing.T) {
	for _, point := range []string{"before_sync", "before_rename", "after_rename"} {
		t.Run(point, func(t *testing.T) {
			p, s := setup(t)
			ctx := context.Background()
			if e := p.Publish(ctx, s); e != nil {
				t.Fatal(e)
			}
			before, e := os.ReadFile(filepath.Join(p.Dir, Latest))
			if e != nil {
				t.Fatal(e)
			}
			p.fault = func(x string) error {
				if x == point {
					return syscall.ENOSPC
				}
				return nil
			}
			if e = p.Publish(ctx, s); e == nil {
				t.Fatal("fault ignored")
			}
			after, e := os.ReadFile(filepath.Join(p.Dir, Latest))
			if e != nil {
				t.Fatal(e)
			}
			if point != "after_rename" && !bytes.Equal(before, after) {
				t.Fatal("previous generation overwritten")
			}
			if _, e = Restore(ctx, filepath.Join(p.Dir, Latest), privateTemp(t)); e != nil {
				t.Fatal(e)
			}
			if p.Status().State != "error" || p.Status().LastSuccess.IsZero() {
				t.Fatal(p.Status())
			}
		})
	}
}
func TestStartupFailuresAndAuthoritativeRestore(t *testing.T) {
	p, s := setup(t)
	ctx := context.Background()
	if e := p.Publish(ctx, s); e != nil {
		t.Fatal(e)
	}
	runtime := privateTemp(t)
	dir, restored, e := Prepare(ctx, runtime, p.Dir)
	if e != nil || !restored {
		t.Fatal(restored, e)
	}
	if dir != runtime {
		t.Fatal("runtime path changed", dir)
	}
	if e = os.WriteFile(filepath.Join(dir, "energything.sqlite"), []byte("crashed leftovers"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"energything.sqlite-wal", "energything.sqlite-shm", "energything.sqlite.tmp"} {
		if e = os.WriteFile(filepath.Join(runtime, name), []byte("incomplete"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	dir, restored, e = Prepare(ctx, runtime, p.Dir)
	if e != nil || !restored {
		t.Fatal(restored, e)
	}
	if dir != runtime {
		t.Fatal("restart changed runtime path", dir)
	}
	if _, e = storage.ValidateSnapshot(ctx, filepath.Join(dir, "energything.sqlite")); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(p.Dir, Latest)); e != nil {
		t.Fatal(e)
	}
	if _, _, e = Prepare(ctx, runtime, p.Dir); e == nil {
		t.Fatal("silently reset established mount")
	}
	if e = os.WriteFile(filepath.Join(p.Dir, Latest), []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e = Prepare(ctx, runtime, p.Dir); e == nil {
		t.Fatal("accepted corrupt dump")
	}
	legacy := privateTemp(t)
	os.WriteFile(filepath.Join(legacy, "energything.sqlite"), nil, 0600)
	if _, _, e = Prepare(ctx, runtime, legacy); e == nil {
		t.Fatal("accepted legacy layout")
	}
	if e = ValidatePaths(runtime, runtime); e == nil {
		t.Fatal("accepted overlap")
	}
}

func TestCompletedBackupHasOneFixedName(t *testing.T) {
	p, s := setup(t)
	for i := 0; i < 2; i++ {
		if e := p.Publish(context.Background(), s); e != nil {
			t.Fatal(e)
		}
		entries, e := os.ReadDir(p.Dir)
		if e != nil {
			t.Fatal(e)
		}
		if len(entries) != 2 {
			t.Fatal("unexpected backup files", entries)
		}
		for _, entry := range entries {
			if entry.IsDir() || (entry.Name() != Latest && entry.Name() != marker) {
				t.Fatal("unexpected backup path", entry.Name())
			}
		}
	}
}
func TestStrictArchiveAndDeadline(t *testing.T) {
	for _, name := range []string{"../energything.sqlite", "/energything.sqlite", "unexpected"} {
		path := filepath.Join(privateTemp(t), Latest)
		f, e := os.Create(path)
		if e != nil {
			t.Fatal(e)
		}
		tw := tar.NewWriter(f)
		tw.WriteHeader(&tar.Header{Name: name, Size: 1, Typeflag: tar.TypeReg})
		tw.Write([]byte("x"))
		tw.Close()
		f.Close()
		if _, e = Restore(context.Background(), path, privateTemp(t)); e == nil {
			t.Fatal(name)
		}
	}
	p, s := setup(t)
	if e := p.Publish(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := p.Publish(ctx, s); e == nil {
		t.Fatal("ignored cancellation")
	}
	if _, e := Restore(context.Background(), filepath.Join(p.Dir, Latest), privateTemp(t)); e != nil {
		t.Fatal(e)
	}
}

func TestManifestCorruptionAndUnexpectedEntries(t *testing.T) {
	p, s := setup(t)
	ctx := context.Background()
	if e := p.Publish(ctx, s); e != nil {
		t.Fatal(e)
	}
	original, e := os.ReadFile(filepath.Join(p.Dir, Latest))
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"hash", "length", "format", "schema", "duplicate", "link", "missing"} {
		t.Run(kind, func(t *testing.T) {
			var out bytes.Buffer
			tw := tar.NewWriter(&out)
			tr := tar.NewReader(bytes.NewReader(original))
			for {
				h, e := tr.Next()
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal(e)
				}
				data, e := io.ReadAll(tr)
				if e != nil {
					t.Fatal(e)
				}
				if h.Name == "manifest.json" {
					if kind == "missing" {
						continue
					}
					var m Manifest
					if e = json.Unmarshal(data, &m); e != nil {
						t.Fatal(e)
					}
					switch kind {
					case "hash":
						m.SHA256 = string(bytes.Repeat([]byte("0"), 64))
					case "length":
						m.Bytes++
					case "format":
						m.Format++
					case "schema":
						m.Schema++
					}
					data, e = json.Marshal(m)
					if e != nil {
						t.Fatal(e)
					}
					h.Size = int64(len(data))
				}
				if kind == "link" && h.Name == "energything.sqlite" {
					h.Typeflag = tar.TypeSymlink
					h.Linkname = "/elsewhere"
					h.Size = 0
					data = nil
				}
				if e = tw.WriteHeader(h); e != nil {
					t.Fatal(e)
				}
				if _, e = tw.Write(data); e != nil {
					t.Fatal(e)
				}
				if kind == "duplicate" && h.Name == "manifest.json" {
					tw.WriteHeader(h)
					tw.Write(data)
				}
			}
			if e = tw.Close(); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(privateTemp(t), Latest)
			if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = Restore(ctx, path, privateTemp(t)); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}

func TestRestoreDoesNotDeleteExistingDestination(t *testing.T) {
	p, s := setup(t)
	if e := p.Publish(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	destination := privateTemp(t)
	path := filepath.Join(destination, "energything.sqlite")
	if e := os.WriteFile(path, []byte("keep-existing"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Restore(context.Background(), filepath.Join(p.Dir, Latest), destination); e == nil {
		t.Fatal("accepted existing file")
	}
	data, e := os.ReadFile(path)
	if e != nil || string(data) != "keep-existing" {
		t.Fatal("damaged existing destination", e)
	}
}

func TestCrashLosesOnlyUnsnapshottedStateAndFreshRestore(t *testing.T) {
	ctx := context.Background()
	runtime, dumps := privateTemp(t), privateTemp(t)
	dir, restored, e := Prepare(ctx, runtime, dumps)
	if e != nil || restored {
		t.Fatal(restored, e)
	}
	s, e := storage.Open(ctx, dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = testsupport.SeedBrowsing(ctx, s); e != nil {
		t.Fatal(e)
	}
	p := Publisher{Dir: dumps, Interval: time.Second, Timeout: time.Minute}
	if e = p.Publish(ctx, s); e != nil {
		t.Fatal(e)
	}
	ref := filepath.Join(privateTemp(t), "ref.sqlite")
	if e = s.Snapshot(ctx, ref); e != nil {
		t.Fatal(e)
	}
	// Accepted later generation is deliberately never published.
	if e = testsupport.SeedAggregates(ctx, s); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	dir, restored, e = Prepare(ctx, runtime, dumps)
	if e != nil || !restored {
		t.Fatal(restored, e)
	}
	if e = storage.CompareSnapshots(ctx, ref, filepath.Join(dir, "energything.sqlite")); e != nil {
		t.Fatal(e)
	}
}
