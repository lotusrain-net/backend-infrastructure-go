package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL               string
	MinConns          int32
	MaxConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

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

type Pinger interface {
	Ping(context.Context) error
}

func Health(ctx context.Context, pinger Pinger) error {
	if pinger == nil {
		return errors.New("database pinger is nil")
	}
	if err := pinger.Ping(ctx); err != nil {
		return fmt.Errorf("database health check: %w", err)
	}
	return nil
}

type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

func RunInTx(ctx context.Context, beginner Beginner, fn func(context.Context, pgx.Tx) error) error {
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
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
