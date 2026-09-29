package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	envHTTPAddr     = "HTTP_ADDR"
	envDatabaseURL  = "DATABASE_URL"
	defaultHTTPAddr = ":8080"
)

var ErrMissingEnv = errors.New("missing required environment variable")

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func Load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv(envDatabaseURL))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("%w: %s", ErrMissingEnv, envDatabaseURL)
	}

	httpAddr := strings.TrimSpace(getenv(envHTTPAddr))
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	return Config{HTTPAddr: httpAddr, DatabaseURL: databaseURL}, nil
}
