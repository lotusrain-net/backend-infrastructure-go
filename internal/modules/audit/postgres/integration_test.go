package postgres_test

import (
	"os"
	"testing"
	"time"

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
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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

	verifyCombinedServerSideFilters(t, connection, repository)
}

func verifyCombinedServerSideFilters(t *testing.T, connection *pgx.Conn, repository *postgres.Repository) {
	t.Helper()
	ctx := t.Context()
	actor := "8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66"
	otherActor := "2a7cbbae-b086-4714-aa6d-7eb08e3eb59d"
	from := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Hour)
	insert := func(requestID, actorID, action, result, resourceType, resourceID string, createdAt time.Time) {
		t.Helper()
		if _, err := connection.Exec(ctx, `
			INSERT INTO audit_logs (request_id, actor_id, action, result, resource_type, resource_id, metadata, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, $7)
		`, requestID, actorID, action, result, resourceType, resourceID, createdAt); err != nil {
			t.Fatal(err)
		}
	}

	insert("combined", actor, "task.create", "failure", "task", "task-3", from.Add(3*time.Hour))
	insert("combined", actor, "task.create", "failure", "task", "task-3", from.Add(2*time.Hour))
	insert("wrong-request", actor, "task.create", "failure", "task", "task-3", from.Add(2*time.Hour))
	insert("combined", otherActor, "task.create", "failure", "task", "task-3", from.Add(2*time.Hour))
	insert("combined", actor, "auth.login", "failure", "task", "task-3", from.Add(2*time.Hour))
	insert("combined", actor, "task.create", "success", "task", "task-3", from.Add(2*time.Hour))
	insert("combined", actor, "task.create", "failure", "session", "task-3", from.Add(2*time.Hour))
	insert("combined", actor, "task.create", "failure", "task", "task-other", from.Add(2*time.Hour))
	insert("combined", actor, "task.create", "failure", "task", "task-3", from.Add(-time.Minute))
	insert("combined", actor, "task.create", "failure", "task", "task-3", to.Add(time.Minute))

	items, total, err := repository.List(ctx, audit.Filter{
		RequestID: "combined", ActorID: actor, Action: "task.create", Result: audit.ResultFailure,
		ResourceType: "task", ResourceID: "task-3", From: &from, To: &to,
	}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 1 || !items[0].CreatedAt.Equal(from.Add(2*time.Hour)) {
		t.Fatalf("combined filter items=%#v total=%d", items, total)
	}
}
