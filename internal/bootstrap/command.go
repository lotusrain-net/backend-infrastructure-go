package bootstrap

import (
	"context"
	"fmt"
	"io"

	"backend-infrastructure-go/internal/config"
	platformlogging "backend-infrastructure-go/internal/platform/logging"
)

func Execute(ctx context.Context, output io.Writer, role string, components ...Component) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	level, err := platformlogging.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("configure logging: %w", err)
	}
	logger := platformlogging.New(output, level, cfg.ServiceName).With("role", role)
	logger.Info("service starting")
	if err := Run(ctx, logger, cfg.ShutdownTimeout, components...); err != nil {
		return fmt.Errorf("run %s: %w", role, err)
	}
	logger.Info("service stopped")
	return nil
}
