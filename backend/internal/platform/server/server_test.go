package server_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
	"github.com/rifqif16/ecampus/backend/internal/platform/server"
)

const waitLimit = 5 * time.Second

func newLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	return &bytes.Buffer{}
}

func startRun(t *testing.T, handler http.Handler, shutdownTimeout time.Duration) (addr string, cancel context.CancelFunc, done <-chan error) {
	t.Helper()

	log, err := logger.New(newLogger(t), "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	t.Cleanup(cancelFn)
	result := make(chan error, 1)
	srv := server.NewHTTPServer(ln.Addr().String(), handler, log)
	go func() { result <- server.Run(ctx, srv, ln, shutdownTimeout) }()

	return ln.Addr().String(), cancelFn, result
}

func noKeepAliveClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
}

func waitDone(t *testing.T, done <-chan error) error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(waitLimit):
		t.Fatal("Run did not return in time")
		return nil
	}
}

func TestRun_ServesThenShutsDownCleanly(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	addr, cancel, done := startRun(t, ok, time.Second)

	resp, err := noKeepAliveClient().Get("http://" + addr)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	cancel()

	if err := waitDone(t, done); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
}

func TestRun_InFlightRequestCompletesDuringShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	addr, cancel, done := startRun(t, slow, waitLimit)

	statusCh := make(chan int, 1)
	go func() {
		resp, err := noKeepAliveClient().Get("http://" + addr)
		if err != nil {
			statusCh <- -1
			return
		}
		_ = resp.Body.Close()
		statusCh <- resp.StatusCode
	}()
	<-started

	cancel()

	select {
	case err := <-done:
		t.Fatalf("Run returned early (%v) while a request was in flight", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	if status := <-statusCh; status != http.StatusOK {
		t.Fatalf("in-flight request status = %d, want 200", status)
	}
	if err := waitDone(t, done); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
}

func TestRun_ShutdownTimeoutForcesCloseAndReportsError(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	stuck := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
	})
	addr, cancel, done := startRun(t, stuck, 50*time.Millisecond)

	go func() { _, _ = noKeepAliveClient().Get("http://" + addr) }()
	<-started

	cancel()

	err := waitDone(t, done)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRun_ReturnsErrorWhenServeFails(t *testing.T) {
	log, err := logger.New(newLogger(t), "debug")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_ = ln.Close() // Serve fails immediately on a closed listener.

	srv := server.NewHTTPServer(ln.Addr().String(), http.NotFoundHandler(), log)
	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background(), srv, ln, time.Second) }()

	if err := waitDone(t, done); err == nil {
		t.Fatal("expected serve error, got nil")
	}
}

func TestNewHTTPServer_AppliesProjectTimeouts(t *testing.T) {
	log, err := logger.New(newLogger(t), "info")
	if err != nil {
		t.Fatalf("logger: %v", err)
	}

	srv := server.NewHTTPServer(":8080", http.NotFoundHandler(), log)

	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second ||
		srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected timeouts: %+v", srv)
	}
	if srv.ErrorLog == nil || srv.Addr != ":8080" {
		t.Fatalf("addr/errorlog not set: %+v", srv)
	}
}
