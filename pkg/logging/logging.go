package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New creates a JSON slog logger with a stable service attribute.
func New(output io.Writer, level slog.Level, service string) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With("service", service)
}

// ParseLevel parses one of the exact values debug, info, warn, or error.
func ParseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", value)
	}
}
