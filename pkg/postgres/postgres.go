package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config describes PostgreSQL connection-pool settings.
type Config struct {
	// URL is the PostgreSQL connection URL.
	URL string
	// MinConns is the minimum number of connections kept in the pool.
	MinConns int32
	// MaxConns is the maximum number of connections allowed in the pool.
	MaxConns int32
	// MaxConnLifetime is the maximum lifetime of a connection; zero uses pgx defaults.
	MaxConnLifetime time.Duration
	// MaxConnIdleTime is the maximum idle time of a connection; zero uses pgx defaults.
	MaxConnIdleTime time.Duration
	// HealthCheckPeriod controls how often idle connections are health checked; zero uses pgx defaults.
	HealthCheckPeriod time.Duration
}

// Validate checks URL, pool boundaries, and durations.
func (c Config) Validate() error {
	if c.URL == "" {
		return errors.New("database URL is required")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return errors.New("database URL must use postgres or postgresql scheme")
	}
	if c.MinConns < 0 {
		return errors.New("database MinConns must not be negative")
	}
	if c.MaxConns <= 0 {
		return errors.New("database MaxConns must be positive")
	}
	if c.MinConns > c.MaxConns {
		return errors.New("database MinConns must not exceed MaxConns")
	}
	if c.MaxConnLifetime < 0 || c.MaxConnIdleTime < 0 || c.HealthCheckPeriod < 0 {
		return errors.New("database durations must not be negative")
	}
	return nil
}

// Open creates a pool and verifies connectivity before returning it.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConns = cfg.MaxConns
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Pinger is implemented by PostgreSQL clients that expose a health probe.
type Pinger interface {
	// Ping verifies that the database is reachable.
	Ping(context.Context) error
}

// Health verifies that pinger is configured and reachable.
func Health(ctx context.Context, pinger Pinger) error {
	if nilInterface(pinger) {
		return errors.New("database pinger is nil")
	}
	if err := pinger.Ping(ctx); err != nil {
		return fmt.Errorf("database health check: %w", err)
	}
	return nil
}

// Beginner starts pgx transactions.
type Beginner interface {
	// Begin starts a transaction.
	Begin(context.Context) (pgx.Tx, error)
}

// RunInTx runs fn in a transaction, committing on success and rolling back on
// callback error or panic. Panics are rethrown after rollback.
func RunInTx(ctx context.Context, beginner Beginner, fn func(context.Context, pgx.Tx) error) error {
	if nilInterface(beginner) {
		return errors.New("transaction beginner is nil")
	}
	if fn == nil {
		return errors.New("transaction callback is nil")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(ctx)
			panic(recovered)
		}
	}()
	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
