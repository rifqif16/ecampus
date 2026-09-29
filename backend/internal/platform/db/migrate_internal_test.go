package db

import (
	"errors"
	"testing"
)

func TestToMigrateURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"postgres scheme", "postgres://u:p@localhost:5432/db?sslmode=disable", "pgx5://u:p@localhost:5432/db?sslmode=disable", false},
		{"postgresql scheme", "postgresql://u:p@localhost/db", "pgx5://u:p@localhost/db", false},
		{"unsupported scheme", "mysql://u:p@localhost/db", "", true},
		{"no scheme", "localhost/db", "", true},
		{"malformed", "postgres://u:p@local host/db", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toMigrateURL(tt.in)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidDatabaseURL) {
				t.Fatalf("error = %v, want ErrInvalidDatabaseURL", err)
			}
			if got != tt.want {
				t.Fatalf("url = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToMigrateURL_ErrorDoesNotLeakPassword(t *testing.T) {
	_, err := toMigrateURL("postgres://u:s3cret@local host/db")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); containsAny(got, "s3cret") {
		t.Fatalf("error leaks password: %q", got)
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		for i := 0; i+len(n) <= len(s); i++ {
			if s[i:i+len(n)] == n {
				return true
			}
		}
	}
	return false
}
