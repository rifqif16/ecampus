package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rifqif16/ecampus/backend/internal/platform/config"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestRun_RejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"too many arguments", []string{"serve", "extra"}},
		{"unknown command", []string{"explode"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(context.Background(), tt.args, env(nil), io.Discard)

			if !errors.Is(err, errUsage) {
				t.Fatalf("error = %v, want errUsage", err)
			}
		})
	}
}

func TestRun_FailsWithoutDatabaseURL(t *testing.T) {
	err := run(context.Background(), []string{"serve"}, env(nil), io.Discard)

	if !errors.Is(err, config.ErrMissingEnv) {
		t.Fatalf("error = %v, want ErrMissingEnv", err)
	}
}

func TestRun_FailsOnInvalidLogLevel(t *testing.T) {
	cfg := map[string]string{"DATABASE_URL": "postgres://u:p@localhost/db", "LOG_LEVEL": "loud"}

	err := run(context.Background(), []string{"migrate"}, env(cfg), io.Discard)

	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestServe_FailsFastWhenDatabaseUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := map[string]string{
		"DATABASE_URL": "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1",
		"HTTP_ADDR":    "127.0.0.1:0",
	}

	err := run(ctx, []string{"serve"}, env(cfg), io.Discard)

	if err == nil {
		t.Fatal("expected error when database is unreachable, got nil")
	}
}
