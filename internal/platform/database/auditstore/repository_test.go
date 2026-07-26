package auditstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/platform/database/auditstore"
	"backend-infrastructure-go/internal/platform/database/dbgen"
)

func TestRepositoryCreatesAuditLogThroughDBGen(t *testing.T) {
	actor := "8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66"
	ip := netip.MustParseAddr("203.0.113.9")
	queries := &queriesStub{created: row("00000000-0000-0000-0000-000000000001", "request-1", actor, "auth.login", "success", "session", time.Now())}
	repository := auditstore.New(queries)

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

func TestRepositoryDelegatesCombinedFiltersPaginationAndCountToDBGen(t *testing.T) {
	from := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Hour)
	actor := "8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66"
	queries := &queriesStub{
		rows:  []dbgen.AuditLog{row("00000000-0000-0000-0000-000000000003", "request-3", actor, "task.create", "failure", "task", from.Add(2*time.Hour))},
		count: 2,
	}
	repository := auditstore.New(queries)

	items, total, err := repository.List(context.Background(), audit.Filter{
		RequestID: "request-3", ActorID: actor, Action: "task.create", Result: audit.ResultFailure,
		ResourceType: "task", ResourceID: "task-3", From: &from, To: &to,
	}, 1, 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 2 || len(items) != 1 || items[0].RequestID != "request-3" {
		t.Fatalf("List() items=%#v total=%d", items, total)
	}
	if queries.listCalls != 1 || queries.countCalls != 1 {
		t.Fatalf("query calls list=%d count=%d, want one each", queries.listCalls, queries.countCalls)
	}
	if queries.listArg.Limit != 1 || queries.listArg.Offset != 1 || !queries.listArg.RequestID.Valid || queries.listArg.RequestID.String != "request-3" || !queries.listArg.ActorID.Valid || !queries.listArg.FromTime.Valid || !queries.listArg.ToTime.Valid {
		t.Fatalf("ListFilteredAuditLogs params = %#v", queries.listArg)
	}
	if !queries.countArg.ResourceID.Valid || queries.countArg.ResourceID.String != "task-3" || queries.countArg.Result.String != "failure" {
		t.Fatalf("CountFilteredAuditLogs params = %#v", queries.countArg)
	}
}

func TestRepositoryPropagatesDBGenErrors(t *testing.T) {
	want := errors.New("query failed")
	repository := auditstore.New(&queriesStub{listErr: want})
	_, _, err := repository.List(context.Background(), audit.Filter{}, 20, 0)
	if !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
}

func TestRepositoryPropagatesCountError(t *testing.T) {
	want := errors.New("count failed")
	repository := auditstore.New(&queriesStub{countErr: want})
	_, _, err := repository.List(context.Background(), audit.Filter{}, 20, 0)
	if !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
}

type queriesStub struct {
	created    dbgen.AuditLog
	createArg  dbgen.CreateAuditLogParams
	createErr  error
	rows       []dbgen.AuditLog
	count      int64
	listArg    dbgen.ListFilteredAuditLogsParams
	countArg   dbgen.CountFilteredAuditLogsParams
	listErr    error
	countErr   error
	listCalls  int
	countCalls int
}

func (queries *queriesStub) CreateAuditLog(_ context.Context, arg dbgen.CreateAuditLogParams) (dbgen.AuditLog, error) {
	queries.createArg = arg
	return queries.created, queries.createErr
}

func (queries *queriesStub) ListFilteredAuditLogs(_ context.Context, arg dbgen.ListFilteredAuditLogsParams) ([]dbgen.AuditLog, error) {
	queries.listCalls++
	queries.listArg = arg
	if queries.listErr != nil {
		return nil, queries.listErr
	}
	return queries.rows, nil
}

func (queries *queriesStub) CountFilteredAuditLogs(_ context.Context, arg dbgen.CountFilteredAuditLogsParams) (int64, error) {
	queries.countCalls++
	queries.countArg = arg
	return queries.count, queries.countErr
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
