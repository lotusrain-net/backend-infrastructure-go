// Package database is a compatibility adapter for the public postgres package.
package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jyysy/backend-infrastructure-go/pkg/postgres"
)

type Config = postgres.Config
type Pinger = postgres.Pinger
type Beginner = postgres.Beginner

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) { return postgres.Open(ctx, cfg) }
func Health(ctx context.Context, pinger Pinger) error             { return postgres.Health(ctx, pinger) }
func RunInTx(ctx context.Context, beginner Beginner, fn func(context.Context, pgx.Tx) error) error {
	return postgres.RunInTx(ctx, beginner, fn)
}
