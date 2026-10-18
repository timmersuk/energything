// dump is an offline transition/recovery tool. It never contacts providers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/timmersuk/energything/internal/backup"
	"github.com/timmersuk/energything/internal/storage"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "dump operation failed; verify copied source, empty destination and archive integrity")
		os.Exit(1)
	}
}
func run(args []string) error {
	f := flag.NewFlagSet("dump", flag.ContinueOnError)
	mode := f.String("mode", "", "convert or restore")
	source := f.String("source", "", "Stopped-copy directory for convert; archive file for restore")
	dest := f.String("destination", "", "New empty destination directory")
	stopped := f.Bool("stopped-copy", false, "Confirm source is a complete copy of a cleanly stopped legacy directory")
	timeout := f.Duration("timeout", 5*time.Minute, "Bounded operation deadline")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 || *source == "" || *dest == "" || *timeout <= 0 || (*mode != "convert" && *mode != "restore") {
		return errors.New("invalid dump arguments")
	}
	src, e := filepath.Abs(*source)
	if e != nil {
		return e
	}
	dst, e := filepath.Abs(*dest)
	if e != nil {
		return e
	}
	if *mode == "convert" {
		if !*stopped {
			return errors.New("requires stopped-copy confirmation")
		}
	}
	validationSource := src
	if *mode == "restore" {
		validationSource = filepath.Dir(src)
	}
	if e = backup.CheckOfflinePaths(validationSource, dst); e != nil {
		return e
	}
	// Exclusively create a new destination; never overwrite an existing backup.
	if e = os.Mkdir(dst, 0700); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if *mode == "restore" {
		_, e = backup.Restore(ctx, src, dst)
		return e
	}
	s, e := storage.SnapshotSource(ctx, filepath.Join(src, "energything.sqlite"))
	if e != nil {
		return e
	}
	p := backup.Publisher{Dir: dst, Build: "legacy-conversion", Timeout: *timeout, Interval: 15 * time.Minute}
	if e = p.Publish(ctx, s); e != nil {
		return e
	}
	verify, e := os.MkdirTemp("", "energything-conversion-check-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(verify)
	if _, e = backup.Restore(ctx, filepath.Join(dst, backup.Latest), verify); e != nil {
		return e
	}
	return storage.CompareSnapshots(ctx, filepath.Join(src, "energything.sqlite"), filepath.Join(verify, "energything.sqlite"))
}
