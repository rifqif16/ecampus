//go:build integration

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/rifqif16/ecampus/backend/internal/platform/db"
)

const eventually = 15 * time.Second

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if ctr != nil {
		t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	}
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return url
}

func freeAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func waitForStatus(t *testing.T, url string, want int) {
	t.Helper()

	deadline := time.Now().Add(eventually)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == want {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s never returned %d", url, want)
}

func TestMigrateThenServe_AgainstRealDatabase(t *testing.T) {
	dbURL := startPostgres(t)
	addr := freeAddr(t)
	cfg := env(map[string]string{"DATABASE_URL": dbURL, "HTTP_ADDR": addr})

	for i := 0; i < 2; i++ { // second run must be a no-op
		if err := run(context.Background(), []string{"migrate"}, cfg, io.Discard); err != nil {
			t.Fatalf("migrate run %d: %v", i+1, err)
		}
	}

	pool, err := db.NewPool(context.Background(), dbURL, db.DefaultPoolOptions())
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_proc WHERE proname = 'set_updated_at'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("set_updated_at present = %d, err = %v", n, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"serve"}, cfg, io.Discard) }()

	waitForStatus(t, fmt.Sprintf("http://%s/healthz", addr), http.StatusOK)
	waitForStatus(t, fmt.Sprintf("http://%s/readyz", addr), http.StatusOK)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v, want nil after graceful shutdown", err)
		}
	case <-time.After(eventually):
		t.Fatal("serve did not stop after cancel")
	}
}
