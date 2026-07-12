package migrations_test

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
)

func TestMigrationsUpDownUpAndSeedIdempotency(t *testing.T) {
	databaseURL := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL is not set")
	}

	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(directory)}).String()
	migrator, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = migrator.Close() })

	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("initial down: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if err := migrator.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second up error = %v, want ErrNoChange", err)
	}

	ctx := t.Context()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	seedSQL, err := os.ReadFile("000004_seed_rbac.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := conn.Exec(ctx, string(seedSQL)); err != nil {
			t.Fatalf("reapply idempotent seed: %v", err)
		}
	}

	var roles, permissions int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM roles WHERE name IN ('admin', 'user')").Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM permissions").Scan(&permissions); err != nil {
		t.Fatal(err)
	}
	if roles != 2 || permissions != 10 {
		t.Fatalf("seed counts roles=%d permissions=%d", roles, permissions)
	}

	_, err = conn.Exec(ctx, "INSERT INTO users (email, username, password_hash) VALUES ('invalid@example.com', '', 'hash')")
	if err == nil {
		t.Fatal("users_username_not_blank constraint accepted an empty username")
	}

	if err := migrator.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}
