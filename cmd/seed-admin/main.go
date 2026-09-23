package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/internal/app"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/config"
	platformlogging "github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	database "github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := cfg.ValidateAdminBootstrap(); err != nil {
		return fmt.Errorf("validate administrator bootstrap: %w", err)
	}
	level, err := platformlogging.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("configure logging: %w", err)
	}
	logger := platformlogging.New(os.Stdout, level, cfg.ServiceName).With("role", "seed-admin")
	pool, err := database.Open(ctx, database.Config{
		URL: cfg.DatabaseURL, MinConns: cfg.DatabaseMinConns, MaxConns: cfg.DatabaseMaxConns,
		HealthCheckPeriod: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pool.Close()
	if err := app.SeedBootstrapAdmin(ctx, pool, iam.NewPasswordHasher(iam.DefaultArgon2Params()), app.AdminBootstrap{
		Email: cfg.AdminEmail, Username: cfg.AdminUsername, Password: cfg.AdminPassword,
	}); err != nil {
		return err
	}
	logger.Info("administrator seed completed")
	return nil
}
