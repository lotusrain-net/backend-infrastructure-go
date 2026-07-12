package database

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestIntegrationTransactionCommitAndRollback(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}

	ctx := t.Context()
	pool, err := Open(ctx, Config{URL: databaseURL, MinConns: 0, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)

	if _, err := conn.Exec(ctx, "CREATE TEMP TABLE sp02_tx_test (value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if err := RunInTx(ctx, conn, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO sp02_tx_test (value) VALUES ('committed')")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	wantRollback := errors.New("rollback requested")
	if err := RunInTx(ctx, conn, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO sp02_tx_test (value) VALUES ('rolled-back')"); err != nil {
			return err
		}
		return wantRollback
	}); !errors.Is(err, wantRollback) {
		t.Fatalf("rollback error = %v", err)
	}

	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM sp02_tx_test").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}
