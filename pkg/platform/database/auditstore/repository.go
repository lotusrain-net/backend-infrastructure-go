package auditstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/audit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
)

const maxDatabaseInt32 = 1<<31 - 1

type Queries interface {
	CreateAuditLog(context.Context, dbgen.CreateAuditLogParams) (dbgen.AuditLog, error)
	ListFilteredAuditLogs(context.Context, dbgen.ListFilteredAuditLogsParams) ([]dbgen.AuditLog, error)
	CountFilteredAuditLogs(context.Context, dbgen.CountFilteredAuditLogsParams) (int64, error)
}

type Repository struct{ queries Queries }

func New(queries Queries) *Repository { return &Repository{queries: queries} }

func (repository *Repository) Create(ctx context.Context, event audit.NewEvent) (audit.Event, error) {
	if repository == nil || repository.queries == nil {
		return audit.Event{}, fmt.Errorf("audit queries are required")
	}
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return audit.Event{}, fmt.Errorf("encode audit metadata: %w", err)
	}
	actorID, err := optionalUUID(event.ActorID)
	if err != nil {
		return audit.Event{}, fmt.Errorf("parse actor ID: %w", err)
	}
	row, err := repository.queries.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
		RequestID: event.RequestID, ActorID: actorID, Action: event.Action, Result: string(event.Result),
		ResourceType: event.ResourceType, ResourceID: optionalText(event.ResourceID), IpAddress: cloneAddress(event.IPAddress),
		UserAgent: optionalText(event.UserAgent), Metadata: encoded,
	})
	if err != nil {
		return audit.Event{}, fmt.Errorf("create audit log: %w", err)
	}
	return toEvent(row)
}

func (repository *Repository) List(ctx context.Context, filter audit.Filter, limit, offset int) ([]audit.Event, int64, error) {
	if repository == nil || repository.queries == nil {
		return nil, 0, fmt.Errorf("audit queries are required")
	}
	if limit < 1 || offset < 0 || limit > maxDatabaseInt32 || offset > maxDatabaseInt32 {
		return nil, 0, fmt.Errorf("limit must be positive and offset non-negative")
	}
	params, err := newFilterParams(filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := repository.queries.ListFilteredAuditLogs(ctx, dbgen.ListFilteredAuditLogsParams{
		RequestID: params.requestID, ActorID: params.actorID, Action: params.action, Result: params.result,
		ResourceType: params.resourceType, ResourceID: params.resourceID, FromTime: params.fromTime, ToTime: params.toTime,
		Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list filtered audit logs: %w", err)
	}
	items := make([]audit.Event, 0, len(rows))
	for _, row := range rows {
		event, err := toEvent(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, event)
	}
	total, err := repository.queries.CountFilteredAuditLogs(ctx, dbgen.CountFilteredAuditLogsParams{
		RequestID: params.requestID, ActorID: params.actorID, Action: params.action, Result: params.result,
		ResourceType: params.resourceType, ResourceID: params.resourceID, FromTime: params.fromTime, ToTime: params.toTime,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count filtered audit logs: %w", err)
	}
	return items, total, nil
}

type filterParams struct {
	requestID    pgtype.Text
	actorID      pgtype.UUID
	action       pgtype.Text
	result       pgtype.Text
	resourceType pgtype.Text
	resourceID   pgtype.Text
	fromTime     pgtype.Timestamptz
	toTime       pgtype.Timestamptz
}

func newFilterParams(filter audit.Filter) (filterParams, error) {
	var actorID pgtype.UUID
	if filter.ActorID != "" {
		if err := actorID.Scan(filter.ActorID); err != nil {
			return filterParams{}, fmt.Errorf("parse audit actor filter: %w", err)
		}
	}
	return filterParams{
		requestID: optionalText(filter.RequestID), actorID: actorID, action: optionalText(filter.Action),
		result: optionalText(string(filter.Result)), resourceType: optionalText(filter.ResourceType), resourceID: optionalText(filter.ResourceID),
		fromTime: optionalTime(filter.From), toTime: optionalTime(filter.To),
	}, nil
}

func toEvent(row dbgen.AuditLog) (audit.Event, error) {
	if !row.ID.Valid || !row.CreatedAt.Valid {
		return audit.Event{}, fmt.Errorf("audit row has invalid ID or creation time")
	}
	metadata := make(map[string]any)
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return audit.Event{}, fmt.Errorf("decode audit metadata: %w", err)
		}
	}
	event := audit.Event{
		ID: uuidString(row.ID), RequestID: row.RequestID, Action: row.Action, Result: audit.Result(row.Result),
		ResourceType: row.ResourceType, IPAddress: cloneAddress(row.IpAddress), Metadata: metadata, CreatedAt: row.CreatedAt.Time,
	}
	if row.ActorID.Valid {
		value := uuidString(row.ActorID)
		event.ActorID = &value
	}
	if row.ResourceID.Valid {
		event.ResourceID = row.ResourceID.String
	}
	if row.UserAgent.Valid {
		event.UserAgent = row.UserAgent.String
	}
	return event, nil
}

func optionalUUID(value *string) (pgtype.UUID, error) {
	if value == nil || *value == "" {
		return pgtype.UUID{}, nil
	}
	var id pgtype.UUID
	if err := id.Scan(*value); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func optionalTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func cloneAddress(address *netip.Addr) *netip.Addr {
	if address == nil {
		return nil
	}
	cloned := *address
	return &cloned
}

func uuidString(id pgtype.UUID) string {
	value := id.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}
