package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"

	"github.com/jackc/pgx/v5/pgtype"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/platform/database/dbgen"
)

const defaultBatchSize = 500

type Queries interface {
	CreateAuditLog(context.Context, dbgen.CreateAuditLogParams) (dbgen.AuditLog, error)
	ListAuditLogs(context.Context, dbgen.ListAuditLogsParams) ([]dbgen.AuditLog, error)
}

type Repository struct {
	queries   Queries
	batchSize int
}

func New(queries Queries) *Repository { return NewWithBatchSize(queries, defaultBatchSize) }

func NewWithBatchSize(queries Queries, batchSize int) *Repository {
	if batchSize < 1 {
		batchSize = defaultBatchSize
	}
	return &Repository{queries: queries, batchSize: batchSize}
}

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
	if limit < 1 || offset < 0 {
		return nil, 0, fmt.Errorf("limit must be positive and offset non-negative")
	}

	matched := make([]audit.Event, 0, limit)
	var total int64
	for scanOffset := 0; ; scanOffset += repository.batchSize {
		if scanOffset > math.MaxInt32 {
			return nil, 0, fmt.Errorf("audit scan offset exceeds database limit")
		}
		rows, err := repository.queries.ListAuditLogs(ctx, dbgen.ListAuditLogsParams{
			Limit: int32(repository.batchSize), Offset: int32(scanOffset),
		})
		if err != nil {
			return nil, 0, fmt.Errorf("list audit logs: %w", err)
		}
		for _, row := range rows {
			event, err := toEvent(row)
			if err != nil {
				return nil, 0, err
			}
			if !matches(event, filter) {
				continue
			}
			if total >= int64(offset) && len(matched) < limit {
				matched = append(matched, event)
			}
			total++
		}
		if len(rows) < repository.batchSize {
			break
		}
	}
	return matched, total, nil
}

func matches(event audit.Event, filter audit.Filter) bool {
	if filter.RequestID != "" && event.RequestID != filter.RequestID {
		return false
	}
	if filter.ActorID != "" && (event.ActorID == nil || *event.ActorID != filter.ActorID) {
		return false
	}
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}
	if filter.Result != "" && event.Result != filter.Result {
		return false
	}
	if filter.ResourceType != "" && event.ResourceType != filter.ResourceType {
		return false
	}
	if filter.ResourceID != "" && event.ResourceID != filter.ResourceID {
		return false
	}
	if filter.From != nil && event.CreatedAt.Before(*filter.From) {
		return false
	}
	if filter.To != nil && event.CreatedAt.After(*filter.To) {
		return false
	}
	return true
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
