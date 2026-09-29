package logger_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rifqif16/ecampus/backend/internal/platform/logger"
)

func TestNew_ValidLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "INFO", "Warn"} {
		t.Run(level, func(t *testing.T) {
			if _, err := logger.New(&bytes.Buffer{}, level); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNew_InvalidLevels(t *testing.T) {
	for _, level := range []string{"", "verbose", "trace-ish"} {
		t.Run("level="+level, func(t *testing.T) {
			log, err := logger.New(&bytes.Buffer{}, level)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if log != nil {
				t.Fatal("expected nil logger on error")
			}
		})
	}
}

func TestNew_EmitsJSONAndFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	log, err := logger.New(&buf, "info")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Debug("hidden")
	log.Info("shown", "request_id", "abc")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), buf.String())
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if entry["msg"] != "shown" || entry["level"] != "INFO" || entry["request_id"] != "abc" {
		t.Fatalf("unexpected log entry: %v", entry)
	}
}
