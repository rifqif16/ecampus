package logger

import (
	"fmt"
	"io"
	"log/slog"
)

func New(w io.Writer, level string) (*slog.Logger, error) {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parsed})
	return slog.New(handler), nil
}
