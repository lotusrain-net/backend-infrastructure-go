package postgres_test

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/audit/postgres"
	"backend-infrastructure-go/internal/platform/database/dbgen"
)

func TestIntegrationCreateAndListAuditLog(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}
	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close(ctx) })
	if _, err := connection.Exec(ctx, `
		CREATE TEMP TABLE audit_logs (
			id UUID PRIMARY KEY DEFAULT '00000000-0000-0000-0000-000000000001',
			request_id TEXT NOT NULL,
			actor_id UUID,
			action TEXT NOT NULL,
			result TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT,
			ip_address INET,
			user_agent TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		t.Fatal(err)
	}

	repository := postgres.New(dbgen.New(connection))
	created, err := repository.Create(ctx, audit.NewEvent{
		RequestID: "integration-request", Action: "administration.user.create", Result: audit.ResultSuccess,
		ResourceType: "user", Metadata: map[string]any{"source": "integration"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items, total, err := repository.List(ctx, audit.Filter{RequestID: "integration-request"}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || total != 1 || len(items) != 1 || items[0].Metadata["source"] != "integration" {
		t.Fatalf("created=%#v items=%#v total=%d", created, items, total)
	}
}
