// Package backup owns immutable online dump publication and strict restoration.
package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/timmersuk/energything/internal/storage"
)

const Latest = "latest.tar"
const marker = "initialized"

type Manifest struct {
	Format    int       `json:"format_version"`
	Schema    int       `json:"schema_version"`
	Build     string    `json:"build_id"`
	Started   time.Time `json:"snapshot_started_at"`
	Completed time.Time `json:"snapshot_completed_at"`
	Bytes     int64     `json:"database_bytes"`
	SHA256    string    `json:"sha256"`
}
type Status struct {
	State           string    `json:"state"`
	LastSuccess     time.Time `json:"last_success"`
	SnapshotTime    time.Time `json:"snapshot_time"`
	AgeSeconds      int64     `json:"age_seconds"`
	DurationSeconds float64   `json:"duration_seconds"`
	Bytes           int64     `json:"bytes"`
	NextDue         time.Time `json:"next_due"`
	Failure         string    `json:"last_failure,omitempty"`
}
type Publisher struct {
	Dir, Build        string
	Interval, Timeout time.Duration
	mu                sync.Mutex
	status            Status
	publishMu         sync.Mutex
	fault             func(string) error
}

func (p *Publisher) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.status
	if !s.SnapshotTime.IsZero() {
		s.AgeSeconds = int64(time.Since(s.SnapshotTime).Seconds())
	}
	return s
}
func (p *Publisher) checkpoint(point string) error {
	if p.fault != nil {
		return p.fault(point)
	}
	return nil
}

func (p *Publisher) Publish(ctx context.Context, s *storage.Store) (err error) {
	p.publishMu.Lock()
	defer p.publishMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()
	started := time.Now().UTC()
	p.mu.Lock()
	p.status.State = "running"
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.status.NextDue = time.Now().UTC().Add(p.Interval)
		p.status.DurationSeconds = time.Since(started).Seconds()
		if err != nil {
			p.status.State = "error"
			p.status.Failure = "Backup failed; last published dump retained. Check storage capacity, permissions and backup deadline."
			err = errors.New(p.status.Failure)
		}
	}()
	for _, name := range []string{Latest, marker} {
		info, e := os.Lstat(filepath.Join(p.Dir, name))
		if e == nil && !info.Mode().IsRegular() {
			return errors.New("publication state must be regular files")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	stage, err := os.MkdirTemp(p.Dir, ".energything-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	dbPath := filepath.Join(stage, "energything.sqlite")
	if err = s.Snapshot(ctx, dbPath); err != nil {
		return err
	}
	version, err := storage.ValidateSnapshot(ctx, dbPath)
	if err != nil {
		return err
	}
	f, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, contextReader{ctx, f}); err != nil {
		return err
	}
	m := Manifest{1, version, p.Build, started, time.Now().UTC(), info.Size(), hex.EncodeToString(hash.Sum(nil))}
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	archive, err := os.OpenFile(filepath.Join(stage, Latest), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(archive)
	data, err := json.Marshal(m)
	if err == nil {
		err = tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg})
	}
	if err == nil {
		_, err = tw.Write(data)
	}
	if err == nil {
		err = tw.WriteHeader(&tar.Header{Name: "energything.sqlite", Mode: 0600, Size: m.Bytes, Typeflag: tar.TypeReg})
	}
	if err == nil {
		_, err = io.Copy(tw, contextReader{ctx, f})
	}
	err = errors.Join(err, tw.Close())
	if err == nil {
		err = p.checkpoint("before_sync")
	}
	if err == nil {
		err = archive.Sync()
	}
	err = errors.Join(err, archive.Close())
	if err != nil {
		return err
	}
	// Both snapshot and archive are closed before publishing on every platform.
	if err = f.Close(); err != nil {
		return err
	}
	if err = p.checkpoint("before_rename"); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(filepath.Join(stage, Latest), filepath.Join(p.Dir, Latest)); err != nil {
		return err
	}
	if err = p.checkpoint("after_rename"); err != nil {
		return err
	}
	if err = syncDirectory(p.Dir); err != nil {
		return err
	}
	if err = writeMarker(p.Dir); err != nil {
		return err
	}
	p.mu.Lock()
	p.status.State = "idle"
	p.status.Failure = ""
	p.status.LastSuccess = time.Now().UTC()
	p.status.SnapshotTime = m.Started
	p.status.Bytes = m.Bytes
	p.mu.Unlock()
	return nil
}
func (p *Publisher) Run(ctx context.Context, s *storage.Store) {
	// Startup publication is completed synchronously before collectors start.
	timer := time.NewTimer(p.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			_ = p.Publish(ctx, s)
			timer.Reset(p.Interval)
		}
	}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}

func writeMarker(dir string) error {
	if info, err := os.Lstat(filepath.Join(dir, marker)); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("invalid initialization marker")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(dir, ".energything-marker-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.WriteString("energything dump format 1\n")
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(dir, marker)); err != nil {
		return err
	}
	return syncDirectory(dir)
}

// Prepare never opens leftover runtime databases. Each startup uses a new
// private generation; only owned stale generations are removed after validation.
func Prepare(ctx context.Context, runtime, dumps string) (dir string, restored bool, err error) {
	if err = ValidatePaths(runtime, dumps); err != nil {
		return
	}
	for _, name := range []string{"energything.sqlite", "energything.sqlite-wal", "energything.sqlite-shm"} {
		if _, e := os.Lstat(filepath.Join(dumps, name)); !os.IsNotExist(e) {
			err = errors.New("legacy persistent database detected; perform deliberate stopped-copy conversion")
			return
		}
	}
	dir, err = os.MkdirTemp(runtime, ".energything-runtime-")
	if err != nil {
		return
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	_, e := os.Lstat(filepath.Join(dumps, Latest))
	if e == nil {
		_, err = Restore(ctx, filepath.Join(dumps, Latest), dir)
		if err != nil {
			return
		}
		restored = true
	} else if os.IsNotExist(e) {
		entries, readErr := os.ReadDir(dumps)
		if readErr != nil {
			err = readErr
			return
		}
		// Any state means this is not an unambiguous initial install.
		if len(entries) != 0 {
			err = errors.New("published dump missing on nonempty mount; restore a retained latest.tar")
			return
		}
	} else {
		err = e
		return
	}
	entries, e := os.ReadDir(runtime)
	if e != nil {
		err = e
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".energything-runtime-") && entry.IsDir() && entry.Name() != filepath.Base(dir) {
			if err = os.RemoveAll(filepath.Join(runtime, entry.Name())); err != nil {
				return
			}
		}
	}
	ok = true
	return
}

func privateDirectory(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	// Reject symlinks/reparse links in every existing component.
	for part := abs; ; part = filepath.Dir(part) {
		info, e := os.Lstat(part)
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return errors.New("storage directory must not traverse links or files")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if filepath.Dir(part) == part {
			break
		}
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(abs, ".energything-write-")
	if err != nil {
		return errors.New("backup and runtime directories must be writable by UID 10001")
	}
	name := f.Name()
	return errors.Join(f.Close(), os.Remove(name))
}

// CheckOfflinePaths rejects links/overlap for conversion without applying the
// server's unmounted-runtime requirement to a stopped copy on external storage.
func CheckOfflinePaths(source, destination string) error {
	for _, path := range []string{source, filepath.Dir(destination)} {
		if e := privateDirectory(path); e != nil {
			return e
		}
	}
	for _, pair := range [][2]string{{source, destination}, {destination, source}} {
		rel, e := filepath.Rel(pair[0], pair[1])
		if e != nil {
			return e
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return errors.New("source and destination overlap")
		}
	}
	return nil
}
func ValidatePaths(runtime, dumps string) error {
	r, e := filepath.Abs(runtime)
	if e != nil {
		return e
	}
	d, e := filepath.Abs(dumps)
	if e != nil {
		return e
	}
	for _, pair := range [][2]string{{r, d}, {d, r}} {
		rel, e := filepath.Rel(pair[0], pair[1])
		if e != nil {
			return e
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return errors.New("runtime and dump directories must be disjoint")
		}
	}
	if e = privateDirectory(r); e != nil {
		return e
	}
	if e = privateDirectory(d); e != nil {
		return e
	}
	return validateRuntimeMount(r)
}

// Restore accepts exactly two regular archive entries, and never extracts paths.
// The caller supplies a new empty private directory, not a running database.
func Restore(ctx context.Context, archivePath, destination string) (m Manifest, err error) {
	info, err := os.Lstat(archivePath)
	if err != nil || !info.Mode().IsRegular() {
		return m, errors.New("published dump must be a regular file")
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return m, err
	}
	defer f.Close()
	tr := tar.NewReader(contextReader{ctx, f})
	seen := map[string]bool{}
	path := filepath.Join(destination, "energything.sqlite")
	ok := false
	created := false
	defer func() {
		if !ok && created {
			os.Remove(path)
		}
	}()
	var bytes int64
	var digest string
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return m, e
		}
		if h.Typeflag != tar.TypeReg || seen[h.Name] || (h.Name != "manifest.json" && h.Name != "energything.sqlite") {
			return m, errors.New("unexpected dump archive entry")
		}
		seen[h.Name] = true
		switch h.Name {
		case "manifest.json":
			if h.Size < 1 || h.Size > 16384 {
				return m, errors.New("invalid manifest size")
			}
			data, e := io.ReadAll(io.LimitReader(tr, 16385))
			if e != nil {
				return m, e
			}
			dec := json.NewDecoder(strings.NewReader(string(data)))
			dec.DisallowUnknownFields()
			if e = dec.Decode(&m); e != nil {
				return m, e
			}
			var extra any
			if dec.Decode(&extra) != io.EOF {
				return m, errors.New("invalid manifest tail")
			}
		case "energything.sqlite":
			// Bound by the opened archive's actual length; no declared-size allocation.
			if h.Size < 512 || h.Size > info.Size() {
				return m, errors.New("invalid snapshot length")
			}
			out, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return m, e
			}
			created = true
			hash := sha256.New()
			bytes, e = io.Copy(io.MultiWriter(out, hash), tr)
			if e == nil {
				e = out.Sync()
			}
			e = errors.Join(e, out.Close())
			if e != nil {
				return m, e
			}
			digest = hex.EncodeToString(hash.Sum(nil))
		}
	}
	if len(seen) != 2 || m.Format != 1 || m.Schema < 1 || m.Bytes != bytes || len(m.SHA256) != 64 || m.SHA256 != digest || m.Started.IsZero() || m.Completed.Before(m.Started) {
		return m, errors.New("dump manifest or checksum invalid")
	}
	version, e := storage.ValidateSnapshot(ctx, path)
	if e != nil {
		return m, e
	}
	if version != m.Schema {
		return m, errors.New("dump schema does not match manifest")
	}
	ok = true
	return m, nil
}
