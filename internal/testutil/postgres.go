// Package testutil provides isolated infrastructure fixtures for integration tests.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres creates a random schema and removes only that schema at cleanup.
// AUTH_TEST_DATABASE_URL must point to a disposable integration database.
func Postgres(t *testing.T, migrationCount int) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AUTH_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "auth_test_" + uuid.NewString()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		_ = admin.Close(ctx)
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		_ = admin.Close(context.Background())
	})
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = quoted
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	_, file, _, _ := runtime.Caller(0)
	files, e := filepath.Glob(filepath.Join(filepath.Dir(file), "../../db/migrations/*.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if migrationCount > 0 {
		files = files[:migrationCount]
	}
	for _, path := range files {
		sql, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(sql)); e != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), e)
		}
	}
	return pool
}
