package task

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend-infrastructure-go/internal/platform/database/dbgen"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type DefinitionQueries interface {
	CreateTaskDefinition(context.Context, dbgen.CreateTaskDefinitionParams) (dbgen.TaskDefinition, error)
}

type PostgresDefinitionStore struct {
	queries DefinitionQueries
}

func NewPostgresDefinitionStore(queries DefinitionQueries) *PostgresDefinitionStore {
	return &PostgresDefinitionStore{queries: queries}
}

func (store *PostgresDefinitionStore) CreateDefinition(ctx context.Context, definition NewDefinition) (Definition, error) {
	row, err := store.queries.CreateTaskDefinition(ctx, dbgen.CreateTaskDefinitionParams{
		Name:           definition.Name,
		TaskType:       definition.TaskType,
		Description:    definition.Description,
		DefaultPayload: definition.DefaultPayload,
		MaxRetries:     int32(definition.MaxRetries),
		TimeoutSeconds: int32(definition.Timeout / time.Second),
	})
	if err != nil {
		return Definition{}, err
	}
	return definitionFromRow(row), nil
}

type ExecutionQueries interface {
	CreateTaskExecution(context.Context, dbgen.CreateTaskExecutionParams) (dbgen.TaskExecution, error)
	GetTaskExecution(context.Context, pgtype.UUID) (dbgen.TaskExecution, error)
	UpdateTaskExecutionStatus(context.Context, dbgen.UpdateTaskExecutionStatusParams) error
}

type PostgresExecutionStore struct {
	queries ExecutionQueries
}

func NewPostgresExecutionStore(queries ExecutionQueries) *PostgresExecutionStore {
	return &PostgresExecutionStore{queries: queries}
}

func (store *PostgresExecutionStore) CreateExecution(ctx context.Context, execution NewExecution) (Execution, error) {
	definitionID, err := nullableUUID(execution.DefinitionID)
	if err != nil {
		return Execution{}, err
	}
	row, err := store.queries.CreateTaskExecution(ctx, dbgen.CreateTaskExecutionParams{
		DefinitionID:   definitionID,
		TaskType:       execution.TaskType,
		QueueID:        nullableText(execution.QueueID),
		IdempotencyKey: nullableText(execution.IdempotencyKey),
		Payload:        execution.Payload,
	})
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return Execution{}, fmt.Errorf("%w: %s", ErrDuplicateSubmission, execution.IdempotencyKey)
		}
		return Execution{}, err
	}
	return executionFromRow(row), nil
}

func (store *PostgresExecutionStore) GetExecution(ctx context.Context, id string) (Execution, error) {
	executionID, err := parseUUID(id)
	if err != nil {
		return Execution{}, err
	}
	row, err := store.queries.GetTaskExecution(ctx, executionID)
	if err != nil {
		return Execution{}, err
	}
	return executionFromRow(row), nil
}

func (store *PostgresExecutionStore) UpdateExecution(ctx context.Context, update ExecutionUpdate) error {
	executionID, err := parseUUID(update.ID)
	if err != nil {
		return err
	}
	return store.queries.UpdateTaskExecutionStatus(ctx, dbgen.UpdateTaskExecutionStatusParams{
		ID:            executionID,
		Status:        string(update.Status),
		StartedAt:     nullableTime(update.StartedAt),
		FinishedAt:    nullableTime(update.FinishedAt),
		ErrorSummary:  nullableText(update.ErrorSummary),
		ProcessedRows: update.ProcessedRows,
		Attempt:       int32(update.Attempt),
	})
}

type ScheduleQueries interface {
	ListEnabledTaskSchedules(context.Context) ([]dbgen.TaskSchedule, error)
}

type PostgresScheduleStore struct {
	queries ScheduleQueries
}

func NewPostgresScheduleStore(queries ScheduleQueries) *PostgresScheduleStore {
	return &PostgresScheduleStore{queries: queries}
}

func (store *PostgresScheduleStore) ListEnabledSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := store.queries.ListEnabledTaskSchedules(ctx)
	if err != nil {
		return nil, err
	}
	schedules := make([]Schedule, 0, len(rows))
	for _, row := range rows {
		schedules = append(schedules, Schedule{
			ID:             formatUUID(row.ID),
			DefinitionID:   formatUUID(row.DefinitionID),
			CronExpression: row.CronExpression,
			Timezone:       row.Timezone,
			Payload:        row.Payload,
			Enabled:        row.IsEnabled,
		})
	}
	return schedules, nil
}

func definitionFromRow(row dbgen.TaskDefinition) Definition {
	return Definition{
		ID:             formatUUID(row.ID),
		Name:           row.Name,
		TaskType:       row.TaskType,
		Description:    row.Description,
		DefaultPayload: row.DefaultPayload,
		MaxRetries:     int(row.MaxRetries),
		Timeout:        time.Duration(row.TimeoutSeconds) * time.Second,
		Active:         row.IsActive,
	}
}

func executionFromRow(row dbgen.TaskExecution) Execution {
	return Execution{
		ID:             formatUUID(row.ID),
		DefinitionID:   formatUUID(row.DefinitionID),
		TaskType:       row.TaskType,
		QueueID:        textValue(row.QueueID),
		IdempotencyKey: textValue(row.IdempotencyKey),
		Payload:        row.Payload,
		Status:         Status(row.Status),
		Attempt:        int(row.Attempt),
		ProcessedRows:  row.ProcessedRows,
		ErrorSummary:   textValue(row.ErrorSummary),
		StartedAt:      timeValue(row.StartedAt),
		FinishedAt:     timeValue(row.FinishedAt),
	}
}

func parseUUID(value string) (pgtype.UUID, error) {
	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID %q", value)
	}
	var bytes [16]byte
	copy(bytes[:], decoded)
	return pgtype.UUID{Bytes: bytes, Valid: true}, nil
}

func nullableUUID(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return parseUUID(value)
}

func formatUUID(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	hexValue := hex.EncodeToString(value.Bytes[:])
	return hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32]
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func timeValue(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
