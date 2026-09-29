package db_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/rifqif16/ecampus/backend/internal/platform/db"
)

func TestMigrate_RejectsUnsupportedScheme(t *testing.T) {
	err := db.Migrate("mysql://u:p@localhost/db", fstest.MapFS{}, "migrations")

	if !errors.Is(err, db.ErrInvalidDatabaseURL) {
		t.Fatalf("error = %v, want ErrInvalidDatabaseURL", err)
	}
}

func TestMigrate_FailsWhenSourceDirMissing(t *testing.T) {
	err := db.Migrate("postgres://u:p@127.0.0.1:1/db?sslmode=disable", fstest.MapFS{}, "migrations")

	if err == nil {
		t.Fatal("expected error for missing migrations directory, got nil")
	}
}
