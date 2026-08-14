package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jyysy/backend-infrastructure-go/internal/config"
)

type migrator interface {
	Up() error
	Down() error
	Close() (error, error)
}
type migrationRuntime struct {
	migrator  migrator
	direction string
}

func (r *migrationRuntime) Run(context.Context) error {
	var err error
	switch r.direction {
	case "", "up":
		err = r.migrator.Up()
	case "down":
		err = r.migrator.Down()
	default:
		return fmt.Errorf("migration direction must be up or down: %q", r.direction)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
func (r *migrationRuntime) Shutdown(context.Context) error {
	sourceErr, databaseErr := r.migrator.Close()
	return errors.Join(sourceErr, databaseErr)
}
func BuildMigrate(cfg config.Config, direction string) (*migrationRuntime, error) {
	migrator, err := migrate.New(cfg.MigrationsSource, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}
	return &migrationRuntime{migrator: migrator, direction: direction}, nil
}
