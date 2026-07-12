package app

import (
	"context"
	"fmt"
	"io"

	"backend-infrastructure-go/internal/bootstrap"
	"backend-infrastructure-go/internal/config"
	platformlogging "backend-infrastructure-go/internal/platform/logging"
)

func Execute(ctx context.Context, output io.Writer, role string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	level, err := platformlogging.ParseLevel(cfg.LogLevel)
	if err != nil {
		return err
	}
	logger := platformlogging.New(output, level, cfg.ServiceName).With("role", role)
	var component bootstrap.Component
	switch role {
	case "api":
		component, err = BuildAPI(ctx, cfg, logger)
	case "worker":
		component, err = BuildWorker(ctx, cfg, logger)
	case "scheduler":
		component, err = BuildScheduler(ctx, cfg, logger)
	default:
		return fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		return fmt.Errorf("build %s: %w", role, err)
	}
	return bootstrap.Run(ctx, logger, cfg.ShutdownTimeout, component)
}
