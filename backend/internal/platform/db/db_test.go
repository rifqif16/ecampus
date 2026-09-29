package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/ecampus/backend/internal/platform/db"
)

type failingBeginner struct{ err error }

func (f failingBeginner) Begin(context.Context) (pgx.Tx, error) { return nil, f.err }

func TestNewPool_RejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts db.PoolOptions
	}{
		{"zero max conns", db.PoolOptions{MaxConns: 0, MinConns: 0}},
		{"negative min conns", db.PoolOptions{MaxConns: 5, MinConns: -1}},
		{"min greater than max", db.PoolOptions{MaxConns: 2, MinConns: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := db.NewPool(context.Background(), "postgres://u:p@localhost/db", tt.opts)
			if !errors.Is(err, db.ErrInvalidPoolOptions) {
				t.Fatalf("error = %v, want ErrInvalidPoolOptions", err)
			}
			if pool != nil {
				t.Fatal("expected nil pool on error")
			}
		})
	}
}

func TestNewPool_RejectsMalformedURL(t *testing.T) {
	pool, err := db.NewPool(context.Background(), "://not a url", db.DefaultPoolOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if pool != nil {
		t.Fatal("expected nil pool on error")
	}
}

func TestNewPool_FailsWhenDatabaseUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"
	pool, err := db.NewPool(ctx, url, db.DefaultPoolOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if pool != nil {
		t.Fatal("expected nil pool on error")
	}
}

func TestWithTx_ReturnsBeginError(t *testing.T) {
	sentinel := errors.New("begin failed")
	called := false

	err := db.WithTx(context.Background(), failingBeginner{err: sentinel}, func(db.DBTX) error {
		called = true
		return nil
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
	if called {
		t.Fatal("fn must not run when begin fails")
	}
}
