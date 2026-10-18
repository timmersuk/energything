package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateDirectoriesAndMountIsolation(t *testing.T) {
	path := privateTemp(t)
	if e := os.Chmod(path, 0777); e != nil {
		t.Fatal(e)
	}
	if e := privateDirectory(path); e == nil {
		t.Fatal("accepted publicly accessible directory")
	}
	if e := os.Chmod(path, 0700); e != nil {
		t.Fatal(e)
	}
	if e := privateDirectory(path); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(privateTemp(t), "linked")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if e := privateDirectory(filepath.Join(link, "child")); e == nil {
		t.Fatal("followed ancestor symlink")
	}
	if e := validateRuntimeMount("/proc/energything-runtime"); e == nil {
		t.Fatal("accepted runtime beneath a separate mount")
	}
	parent := privateTemp(t)
	if e := os.Chmod(parent, 0755); e != nil {
		t.Fatal(e)
	}
	if e := CheckOfflinePaths(path, filepath.Join(parent, "new-private-directory")); e != nil {
		t.Fatal("rejected ordinary destination parent", e)
	}
}
func TestInitialCrashStagingCleanup(t *testing.T) {
	runtime, dumps := privateTemp(t), privateTemp(t)
	stage := filepath.Join(dumps, ".energything-stage-synthetic")
	if e := os.Mkdir(stage, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(stage, "partial"), []byte("incomplete"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, restored, e := Prepare(context.Background(), runtime, dumps); e != nil || restored {
		t.Fatal(restored, e)
	}
	if _, e := os.Stat(stage); !os.IsNotExist(e) {
		t.Fatal("abandoned stage retained", e)
	}
}
