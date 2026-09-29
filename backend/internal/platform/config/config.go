package config

import (
	"errors"
	"fmt"
	"strings"
)

const (
	envHTTPAddr    = "HTTP_ADDR"
	envDatabaseURL = "DATABASE_URL"
	envLogLevel    = "LOG_LEVEL"

	defaultHTTPAddr = ":8080"
	defaultLogLevel = "info"
)

var ErrMissingEnv = errors.New("missing required environment variable")

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    string
}

func Load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv(envDatabaseURL))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("%w: %s", ErrMissingEnv, envDatabaseURL)
	}

	return Config{
		HTTPAddr:    valueOrDefault(getenv(envHTTPAddr), defaultHTTPAddr),
		DatabaseURL: databaseURL,
		LogLevel:    valueOrDefault(getenv(envLogLevel), defaultLogLevel),
	}, nil
}

func valueOrDefault(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}
