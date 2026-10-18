package main

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/backup"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/web"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var BuildID = "local"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run(ctx context.Context) (err error) {
	c, err := config.FromEnv()
	if err != nil {
		return err
	}
	runtimeRoot, err := filepath.Abs("energything-runtime")
	if err != nil {
		return errors.New("runtime path cannot be resolved")
	}
	c.DataDir, err = filepath.Abs(c.DataDir)
	if err != nil {
		return errors.New("ENERGYTHING_DATA_DIR cannot be resolved")
	}
	startup, cancelStartup := context.WithTimeout(ctx, c.BackupTimeout)
	runtimeDir, restored, err := backup.Prepare(startup, runtimeRoot, c.DataDir)
	if err != nil {
		cancelStartup()
		return errors.New("dump restore failed; check storage configuration, legacy layout or restore a valid retained archive")
	}
	db, err := storage.Open(startup, runtimeDir)
	if err != nil {
		cancelStartup()
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if restored {
		db.HoldAggregateCalls(time.Now(), c.Growatt.AggregateDailyCapEnabled)
	}
	backups := &backup.Publisher{Dir: c.DataDir, Build: BuildID, Interval: c.BackupInterval, Timeout: c.BackupTimeout}
	if err = backups.Publish(startup, db); err != nil {
		cancelStartup()
		return err
	}
	cancelStartup()
	if c.Growatt.State == "valid" {
		if err := db.ReconsiderAggregateDeferrals(ctx, storage.At(time.Now()), c.Growatt.AggregateDailyCapEnabled); err != nil {
			return errors.New("aggregate budget configuration could not be applied")
		}
	}
	scheduler := collection.NewScheduler(db, c)
	handler, err := web.NewRouterWithBackup(c, BuildID, db, scheduler.Refresh, backups.Status)
	if err != nil {
		return errors.New("web initialization failed")
	}
	listener, err := net.Listen("tcp", c.HTTPAddr)
	if err != nil {
		return errors.New("HTTP listen failed; check address and whether the port is already in use")
	}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		scheduler.Run(workerCtx)
	}()
	backupDone := make(chan struct{})
	go func() { defer close(backupDone); backups.Run(workerCtx, db) }()
	serveErr := serve(ctx, listener, handler, c.ShutdownTimeout)
	cancelWorker()
	<-done // HTTP and cleanup requests are bounded; close storage last.
	<-backupDone
	cleanup, cancelCleanup := context.WithTimeout(context.Background(), c.BackupTimeout)
	defer cancelCleanup()
	return errors.Join(serveErr, backups.Publish(cleanup, db))
}
func serve(ctx context.Context, listener net.Listener, handler http.Handler, timeout time.Duration) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := server.Shutdown(shutdown)
	if err != nil {
		_ = server.Close()
	}
	serveErr := <-result
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(err, serveErr)
}
