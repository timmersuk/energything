package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShutdownDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; close(finished) })
	result := make(chan error, 1)
	go func() { result <- serve(ctx, listener, h, 50*time.Millisecond) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := http.Client{Timeout: 2 * time.Second}
		resp, _ := client.Get("http://" + listener.Addr().String())
		if resp != nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected shutdown deadline, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown unbounded")
	}
	<-clientDone
}

func TestCancelDuringRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, finished := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(finished) })
	result := make(chan error, 1)
	go func() { result <- serve(ctx, listener, h, time.Second) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := http.Client{Timeout: 2 * time.Second}
		resp, _ := client.Get("http://" + listener.Addr().String())
		if resp != nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown unbounded")
	}
	<-finished
	<-clientDone
}
func TestBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("ENERGYTHING_HTTP_ADDR", listener.Addr().String())
	t.Setenv("ENERGYTHING_USERNAME", "test")
	t.Setenv("ENERGYTHING_PASSWORD", "test")
	t.Chdir(t.TempDir())
	dumps := "data"
	t.Setenv("ENERGYTHING_DATA_DIR", dumps)
	for i := 0; i < 2; i++ {
		if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "HTTP listen failed") {
			t.Fatal("startup did not reach the occupied port", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dumps, "latest.tar")); err != nil {
		t.Fatal("DATA_DIR did not receive the backup", err)
	}
	if _, err := os.Stat(filepath.Join(dumps, "energything.sqlite")); !os.IsNotExist(err) {
		t.Fatal("live SQLite appeared in DATA_DIR", err)
	}
	if _, err := os.Stat(filepath.Join("energything-runtime", "energything.sqlite")); err != nil {
		t.Fatal("internal runtime database missing", err)
	}
}
