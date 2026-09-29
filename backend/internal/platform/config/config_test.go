package config_test

import (
	"errors"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/config"
)

func lookup(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestLoad(t *testing.T) {
	const dbURL = "postgres://u:p@localhost:5432/db"

	tests := []struct {
		name    string
		env     map[string]string
		want    config.Config
		wantErr error
	}{
		{
			name: "all values provided",
			env:  map[string]string{"DATABASE_URL": dbURL, "HTTP_ADDR": ":9000", "LOG_LEVEL": "debug"},
			want: config.Config{HTTPAddr: ":9000", DatabaseURL: dbURL, LogLevel: "debug"},
		},
		{
			name: "optional values default when unset",
			env:  map[string]string{"DATABASE_URL": dbURL},
			want: config.Config{HTTPAddr: ":8080", DatabaseURL: dbURL, LogLevel: "info"},
		},
		{
			name: "optional values default when blank",
			env:  map[string]string{"DATABASE_URL": dbURL, "HTTP_ADDR": "   ", "LOG_LEVEL": " "},
			want: config.Config{HTTPAddr: ":8080", DatabaseURL: dbURL, LogLevel: "info"},
		},
		{
			name: "values are trimmed",
			env:  map[string]string{"DATABASE_URL": "  " + dbURL + "  ", "HTTP_ADDR": " :9000 ", "LOG_LEVEL": " warn "},
			want: config.Config{HTTPAddr: ":9000", DatabaseURL: dbURL, LogLevel: "warn"},
		},
		{
			name:    "missing database url",
			env:     map[string]string{},
			wantErr: config.ErrMissingEnv,
		},
		{
			name:    "blank database url",
			env:     map[string]string{"DATABASE_URL": "  "},
			wantErr: config.ErrMissingEnv,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(lookup(tt.env))

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("config = %+v, want %+v", got, tt.want)
			}
		})
	}
}
