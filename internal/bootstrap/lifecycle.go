// Package bootstrap is a compatibility adapter for the public lifecycle package.
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/jyysy/backend-infrastructure-go/pkg/lifecycle"
)

type Component = lifecycle.Component

func Run(parent context.Context, logger *slog.Logger, timeout time.Duration, components ...Component) error {
	return lifecycle.Run(parent, logger, timeout, components...)
}
