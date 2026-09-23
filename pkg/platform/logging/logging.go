// Package logging is a compatibility adapter for the public logging package.
package logging

import (
	"io"
	"log/slog"

	publiclogging "github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
)

func New(output io.Writer, level slog.Level, service string) *slog.Logger {
	return publiclogging.New(output, level, service)
}

func ParseLevel(value string) (slog.Level, error) { return publiclogging.ParseLevel(value) }
