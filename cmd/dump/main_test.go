package main

import (
	"context"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/testsupport"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertStoppedCopyWithoutMutatingAndRestore(t *testing.T) {
	source := t.TempDir()
	if e := os.Chmod(source, 0700); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	s, e := storage.Open(ctx, source)
	if e != nil {
		t.Fatal(e)
	}
	if e = testsupport.SeedBrowsing(ctx, s); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(source, "energything.sqlite")
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "new-dumps")
	args := []string{"-mode", "convert", "-stopped-copy", "-source", source, "-destination", dest}
	if e = run(args); e != nil {
		t.Fatal(e)
	}
	after, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if string(before) != string(after) {
		t.Fatal("conversion mutated source")
	}
	if e = run(args); e == nil {
		t.Fatal("overwrote existing dump destination")
	}
	restored := filepath.Join(t.TempDir(), "restored")
	if e = run([]string{"-mode", "restore", "-source", filepath.Join(dest, "latest.tar"), "-destination", restored}); e != nil {
		t.Fatal(e)
	}
	if e = storage.CompareSnapshots(ctx, path, filepath.Join(restored, "energything.sqlite")); e != nil {
		t.Fatal(e)
	}
}
