package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/audit/postgres"
	"backend-infrastructure-go/internal/platform/database/dbgen"
)

func TestRepositoryCreatesAuditLogThroughDBGen(t *testing.T) {
	actor := "8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66"
	ip := netip.MustParseAddr("203.0.113.9")
	queries := &queriesStub{created: row("00000000-0000-0000-0000-000000000001", "request-1", actor, "auth.login", "success", "session", time.Now())}
	repository := postgres.New(queries)

	got, err := repository.Create(context.Background(), audit.NewEvent{
		RequestID: "request-1", ActorID: &actor, Action: "auth.login", Result: audit.ResultSuccess,
		ResourceType: "session", ResourceID: "session-1", IPAddress: &ip, UserAgent: "agent", Metadata: map[string]any{"method": "password"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID != "00000000-0000-0000-0000-000000000001" || got.ActorID == nil || *got.ActorID != actor {
		t.Fatalf("Create() = %#v", got)
	}
	if queries.createArg.RequestID != "request-1" || !queries.createArg.ActorID.Valid || queries.createArg.IpAddress == nil {
		t.Fatalf("CreateAuditLog params = %#v", queries.createArg)
	}
	var metadata map[string]any
	if err := json.Unmarshal(queries.createArg.Metadata, &metadata); err != nil || metadata["method"] != "password" {
		t.Fatalf("metadata = %s err=%v", queries.createArg.Metadata, err)
	}
}

func TestRepositoryFiltersBeforeApplyingPagination(t *testing.T) {
	from := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	queries := &queriesStub{rows: []dbgen.AuditLog{
		row("00000000-0000-0000-0000-000000000004", "request-4", "", "task.create", "failure", "task", from.Add(3*time.Hour)),
		row("00000000-0000-0000-0000-000000000003", "request-3", "", "task.create", "failure", "task", from.Add(2*time.Hour)),
		row("00000000-0000-0000-0000-000000000002", "request-2", "", "auth.login", "success", "session", from.Add(time.Hour)),
		row("00000000-0000-0000-0000-000000000001", "request-1", "", "task.create", "failure", "task", from.Add(-time.Hour)),
	}}
	repository := postgres.NewWithBatchSize(queries, 2)

	items, total, err := repository.List(context.Background(), audit.Filter{
		Action: "task.create", Result: audit.ResultFailure, ResourceType: "task", From: &from,
	}, 1, 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 2 || len(items) != 1 || items[0].RequestID != "request-3" {
		t.Fatalf("List() items=%#v total=%d", items, total)
	}
	if queries.listCalls < 3 {
		t.Fatalf("ListAuditLogs calls = %d, want batched scan through exhaustion", queries.listCalls)
	}
}

func TestRepositoryPropagatesDBGenErrors(t *testing.T) {
	want := errors.New("query failed")
	repository := postgres.New(&queriesStub{listErr: want})
	_, _, err := repository.List(context.Background(), audit.Filter{}, 20, 0)
	if !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
}

type queriesStub struct {
	created   dbgen.AuditLog
	createArg dbgen.CreateAuditLogParams
	createErr error
	rows      []dbgen.AuditLog
	listErr   error
	listCalls int
}

func (queries *queriesStub) CreateAuditLog(_ context.Context, arg dbgen.CreateAuditLogParams) (dbgen.AuditLog, error) {
	queries.createArg = arg
	return queries.created, queries.createErr
}

func (queries *queriesStub) ListAuditLogs(_ context.Context, arg dbgen.ListAuditLogsParams) ([]dbgen.AuditLog, error) {
	queries.listCalls++
	if queries.listErr != nil {
		return nil, queries.listErr
	}
	start := int(arg.Offset)
	if start >= len(queries.rows) {
		return []dbgen.AuditLog{}, nil
	}
	end := start + int(arg.Limit)
	if end > len(queries.rows) {
		end = len(queries.rows)
	}
	return queries.rows[start:end], nil
}

func row(id, requestID, actorID, action, result, resourceType string, createdAt time.Time) dbgen.AuditLog {
	metadata := []byte(`{"source":"test"}`)
	return dbgen.AuditLog{
		ID: uuid(id), RequestID: requestID, ActorID: nullableUUID(actorID), Action: action, Result: result,
		ResourceType: resourceType, Metadata: metadata, CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
	}
}

func uuid(value string) pgtype.UUID {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		panic(err)
	}
	return id
}

func nullableUUID(value string) pgtype.UUID {
	if value == "" {
		return pgtype.UUID{}
	}
	return uuid(value)
}
