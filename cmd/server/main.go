package main

import (
	"context"
	"errors"
	"github.com/timmersuk/energything/internal/collection"
	"github.com/timmersuk/energything/internal/config"
	"github.com/timmersuk/energything/internal/storage"
	"github.com/timmersuk/energything/internal/web"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	db, err := storage.Open(ctx, c.DataDir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	scheduler := collection.NewScheduler(db, c)
	handler, err := web.NewRouterWithRefresh(c, BuildID, db, scheduler.Refresh)
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
	serveErr := serve(ctx, listener, handler, c.ShutdownTimeout)
	cancelWorker()
	<-done // HTTP and cleanup requests are bounded; close storage last.
	return serveErr
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
