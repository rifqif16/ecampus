//go:build integration

package db_test

import (
	"context"
	"testing"
	"time"

	dbfs "github.com/rifqif16/ecampus/backend/db"
	"github.com/rifqif16/ecampus/backend/internal/platform/db"
)

func migrateOrFail(t *testing.T) {
	t.Helper()

	if err := db.Migrate(testConnStr, dbfs.Migrations, dbfs.MigrationsDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestMigrate_AppliesMigrationsAndIsIdempotent(t *testing.T) {
	migrateOrFail(t)
	migrateOrFail(t) // second run must be a no-op, not an error

	var n int
	err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_proc WHERE proname = 'set_updated_at'`).Scan(&n)
	if err != nil {
		t.Fatalf("query pg_proc: %v", err)
	}
	if n != 1 {
		t.Fatalf("set_updated_at functions = %d, want 1", n)
	}
}

func TestMigrate_SetUpdatedAtFunctionRefreshesTimestamp(t *testing.T) {
	ctx := context.Background()
	migrateOrFail(t)

	_, err := testPool.Exec(ctx, `
		CREATE TABLE trigger_probe (
			id         INT PRIMARY KEY,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TRIGGER trigger_probe_set_updated_at
			BEFORE UPDATE ON trigger_probe
			FOR EACH ROW EXECUTE FUNCTION set_updated_at();
		INSERT INTO trigger_probe (id, updated_at) VALUES (1, '2000-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatalf("prepare probe table: %v", err)
	}

	if _, err := testPool.Exec(ctx, `UPDATE trigger_probe SET id = 1 WHERE id = 1`); err != nil {
		t.Fatalf("update probe row: %v", err)
	}

	var updatedAt time.Time
	if err := testPool.QueryRow(ctx, `SELECT updated_at FROM trigger_probe WHERE id = 1`).Scan(&updatedAt); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if !updatedAt.After(time.Now().Add(-time.Minute)) {
		t.Fatalf("updated_at = %v, want a recent timestamp", updatedAt)
	}
}
