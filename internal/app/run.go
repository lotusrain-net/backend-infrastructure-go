package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/config"
	bootstrap "github.com/lotusrain-net/backend-infrastructure-go/pkg/lifecycle"
	platformlogging "github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
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
	return executeComponent(ctx, logger, cfg.ShutdownTimeout, component)
}

func executeComponent(ctx context.Context, logger *slog.Logger, shutdownTimeout time.Duration, component bootstrap.Component) error {
	logger.Info("service starting")
	if err := bootstrap.Run(ctx, logger, shutdownTimeout, component); err != nil {
		return err
	}
	logger.Info("service stopped")
	return nil
}
