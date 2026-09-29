//go:build integration

package db_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/rifqif16/ecampus/backend/internal/platform/db"
)

var (
	testPool    *pgxpool.Pool
	testConnStr string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if ctr != nil {
		defer func() { _ = ctr.Terminate(ctx) }()
	}
	if err != nil {
		println("start postgres container:", err.Error())
		return 1
	}

	testConnStr, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		println("connection string:", err.Error())
		return 1
	}

	testPool, err = db.NewPool(ctx, testConnStr, db.DefaultPoolOptions())
	if err != nil {
		println("new pool:", err.Error())
		return 1
	}
	defer testPool.Close()

	_, err = testPool.Exec(ctx, `CREATE TABLE items (
		id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		name TEXT NOT NULL
	)`)
	if err != nil {
		println("create table:", err.Error())
		return 1
	}

	return m.Run()
}

func countByName(t *testing.T, name string) int {
	t.Helper()

	var n int
	err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM items WHERE name = $1`, name).Scan(&n)
	if err != nil {
		t.Fatalf("count items: %v", err)
	}
	return n
}

func insertItem(ctx context.Context, tx db.DBTX, name any) error {
	_, err := tx.Exec(ctx, `INSERT INTO items (name) VALUES ($1)`, name)
	return err
}

func TestNewPool_ConnectsToRealDatabase(t *testing.T) {
	ctx := context.Background()

	pool, err := db.NewPool(ctx, testConnStr, db.PoolOptions{MaxConns: 3, MinConns: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer pool.Close()

	if got := pool.Config().MaxConns; got != 3 {
		t.Fatalf("MaxConns = %d, want 3", got)
	}
}

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()

	err := db.WithTx(ctx, testPool, func(tx db.DBTX) error {
		return insertItem(ctx, tx, "commit-case")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := countByName(t, "commit-case"); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}

func TestWithTx_RollsBackWhenFnFails(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("business rule violated")

	err := db.WithTx(ctx, testPool, func(tx db.DBTX) error {
		if err := insertItem(ctx, tx, "rollback-case"); err != nil {
			return err
		}
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
	if got := countByName(t, "rollback-case"); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}

func TestWithTx_UncommittedWritesInvisibleOutside(t *testing.T) {
	ctx := context.Background()

	err := db.WithTx(ctx, testPool, func(tx db.DBTX) error {
		if err := insertItem(ctx, tx, "isolation-case"); err != nil {
			return err
		}
		if got := countByName(t, "isolation-case"); got != 0 {
			t.Errorf("outside count = %d, want 0 before commit", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := countByName(t, "isolation-case"); got != 1 {
		t.Fatalf("count = %d, want 1 after commit", got)
	}
}

func TestWithTx_ConstraintViolationRollsBackEarlierWrites(t *testing.T) {
	ctx := context.Background()

	err := db.WithTx(ctx, testPool, func(tx db.DBTX) error {
		if err := insertItem(ctx, tx, "partial-case"); err != nil {
			return err
		}
		return insertItem(ctx, tx, nil) // violates NOT NULL
	})

	if err == nil {
		t.Fatal("expected constraint error, got nil")
	}
	if got := countByName(t, "partial-case"); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}

func TestWithTx_RollsBackWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	err := db.WithTx(ctx, testPool, func(tx db.DBTX) error {
		if err := insertItem(ctx, tx, "cancel-case"); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := countByName(t, "cancel-case"); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
}
