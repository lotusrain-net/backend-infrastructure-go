package taskstore

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/platform/database/dbgen"

	"github.com/jackc/pgx/v5"
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

func (store *PostgresDefinitionStore) CreateDefinition(ctx context.Context, definition taskmodule.NewDefinition) (taskmodule.Definition, error) {
	row, err := store.queries.CreateTaskDefinition(ctx, dbgen.CreateTaskDefinitionParams{
		Name:           definition.Name,
		TaskType:       definition.TaskType,
		Description:    definition.Description,
		DefaultPayload: definition.DefaultPayload,
		MaxRetries:     int32(definition.MaxRetries),
		TimeoutSeconds: int32(definition.Timeout / time.Second),
	})
	if err != nil {
		return taskmodule.Definition{}, err
	}
	return definitionFromRow(row), nil
}

type ExecutionQueries interface {
	CreateTaskExecution(context.Context, dbgen.CreateTaskExecutionParams) (dbgen.TaskExecution, error)
	CreateTaskExecutionWithPendingPublish(context.Context, dbgen.CreateTaskExecutionWithPendingPublishParams) (dbgen.CreateTaskExecutionWithPendingPublishRow, error)
	GetTaskExecution(context.Context, pgtype.UUID) (dbgen.TaskExecution, error)
	ClaimTaskExecution(context.Context, dbgen.ClaimTaskExecutionParams) (int64, error)
	UpdateTaskExecutionStatus(context.Context, dbgen.UpdateTaskExecutionStatusParams) (int64, error)
	ListPendingTaskOutboxMessages(context.Context, int32) ([]dbgen.ListPendingTaskOutboxMessagesRow, error)
	MarkTaskOutboxMessagePublished(context.Context, string) error
}

type PostgresExecutionStore struct {
	queries ExecutionQueries
}

func NewPostgresExecutionStore(queries ExecutionQueries) *PostgresExecutionStore {
	return &PostgresExecutionStore{queries: queries}
}

func (store *PostgresExecutionStore) CreateExecution(ctx context.Context, execution taskmodule.NewExecution) (taskmodule.Execution, error) {
	params, err := createExecutionParams(execution)
	if err != nil {
		return taskmodule.Execution{}, err
	}
	row, err := store.queries.CreateTaskExecution(ctx, params)
	if err != nil {
		return taskmodule.Execution{}, mapExecutionWriteError(err, execution.IdempotencyKey)
	}
	return executionFromRow(row), nil
}

func executionFromPendingPublishRow(row dbgen.CreateTaskExecutionWithPendingPublishRow) taskmodule.Execution {
	return executionFromRow(dbgen.TaskExecution(row))
}

func (store *PostgresExecutionStore) CreateExecutionWithPendingPublish(
	ctx context.Context,
	execution taskmodule.NewExecution,
	message taskmodule.Message,
	options taskmodule.PublishOptions,
) (taskmodule.Execution, error) {
	definitionID, err := nullableUUID(execution.DefinitionID)
	if err != nil {
		return taskmodule.Execution{}, err
	}
	row, err := store.queries.CreateTaskExecutionWithPendingPublish(ctx, dbgen.CreateTaskExecutionWithPendingPublishParams{
		DefinitionID:        definitionID,
		TaskType:            execution.TaskType,
		QueueID:             nullableText(options.QueueID),
		IdempotencyKey:      nullableText(execution.IdempotencyKey),
		Payload:             execution.Payload,
		MaxRetries:          int32(options.MaxRetries),
		TimeoutSeconds:      int32(options.Timeout / time.Second),
		UniqueForSeconds:    int32(options.UniqueFor / time.Second),
		ProcessAfterSeconds: int32(options.ProcessAfter / time.Second),
	})
	if err != nil {
		return taskmodule.Execution{}, mapExecutionWriteError(err, execution.IdempotencyKey)
	}
	return executionFromPendingPublishRow(row), nil
}

func (store *PostgresExecutionStore) GetExecution(ctx context.Context, id string) (taskmodule.Execution, error) {
	executionID, err := parseUUID(id)
	if err != nil {
		return taskmodule.Execution{}, taskmodule.ErrExecutionNotFound
	}
	row, err := store.queries.GetTaskExecution(ctx, executionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return taskmodule.Execution{}, taskmodule.ErrExecutionNotFound
		}
		return taskmodule.Execution{}, err
	}
	return executionFromRow(row), nil
}

func (store *PostgresExecutionStore) ClaimExecution(ctx context.Context, id string, attempt int, startedAt time.Time) error {
	executionID, err := parseUUID(id)
	if err != nil {
		return err
	}
	rows, err := store.queries.ClaimTaskExecution(ctx, dbgen.ClaimTaskExecutionParams{
		ID:        executionID,
		Attempt:   int32(attempt),
		StartedAt: nullableTime(&startedAt),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return taskmodule.ErrExecutionConflict
	}
	return nil
}

func (store *PostgresExecutionStore) UpdateExecution(ctx context.Context, update taskmodule.ExecutionUpdate) error {
	executionID, err := parseUUID(update.ID)
	if err != nil {
		return err
	}
	rows, err := store.queries.UpdateTaskExecutionStatus(ctx, dbgen.UpdateTaskExecutionStatusParams{
		ID:              executionID,
		Status:          string(update.Status),
		StartedAt:       nullableTime(update.StartedAt),
		FinishedAt:      nullableTime(update.FinishedAt),
		ErrorSummary:    nullableText(update.ErrorSummary),
		ProcessedRows:   update.ProcessedRows,
		Attempt:         int32(update.Attempt),
		ExpectedStatus:  string(update.ExpectedStatus),
		ExpectedAttempt: int32(update.ExpectedAttempt),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return taskmodule.ErrExecutionConflict
	}
	return nil
}

func (store *PostgresExecutionStore) ListPendingPublishes(ctx context.Context, limit int) ([]taskmodule.PendingPublish, error) {
	rows, err := store.queries.ListPendingTaskOutboxMessages(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	pending := make([]taskmodule.PendingPublish, 0, len(rows))
	for _, row := range rows {
		pending = append(pending, taskmodule.PendingPublish{
			ExecutionStatus: taskmodule.Status(row.ExecutionStatus),
			Message: taskmodule.Message{
				ExecutionID: formatUUID(row.ExecutionID),
				TaskType:    row.TaskType,
				Payload:     row.Payload,
			},
			Options: taskmodule.PublishOptions{
				QueueID:      row.QueueID,
				MaxRetries:   int(row.MaxRetries),
				Timeout:      time.Duration(row.TimeoutSeconds) * time.Second,
				UniqueFor:    time.Duration(row.UniqueForSeconds) * time.Second,
				ProcessAfter: time.Duration(row.ProcessAfterSeconds) * time.Second,
			},
		})
	}
	return pending, nil
}

func (store *PostgresExecutionStore) MarkPublishSucceeded(ctx context.Context, queueID string) error {
	return store.queries.MarkTaskOutboxMessagePublished(ctx, queueID)
}

type ScheduleQueries interface {
	ListEnabledTaskSchedules(context.Context) ([]dbgen.ListEnabledTaskSchedulesRow, error)
}

type PostgresScheduleStore struct {
	queries ScheduleQueries
}

func NewPostgresScheduleStore(queries ScheduleQueries) *PostgresScheduleStore {
	return &PostgresScheduleStore{queries: queries}
}

func (store *PostgresScheduleStore) ListEnabledSchedules(ctx context.Context) ([]taskmodule.Schedule, error) {
	rows, err := store.queries.ListEnabledTaskSchedules(ctx)
	if err != nil {
		return nil, err
	}
	schedules := make([]taskmodule.Schedule, 0, len(rows))
	for _, row := range rows {
		schedules = append(schedules, taskmodule.Schedule{
			ID:             formatUUID(row.ID),
			DefinitionID:   formatUUID(row.DefinitionID),
			CronExpression: row.CronExpression,
			Timezone:       row.Timezone,
			Payload:        row.Payload,
			Enabled:        row.IsEnabled,
			TaskType:       row.TaskType,
			MaxRetries:     int(row.MaxRetries),
			Timeout:        time.Duration(row.TimeoutSeconds) * time.Second,
		})
	}
	return schedules, nil
}

func definitionFromRow(row dbgen.TaskDefinition) taskmodule.Definition {
	return taskmodule.Definition{
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

func executionFromRow(row dbgen.TaskExecution) taskmodule.Execution {
	return taskmodule.Execution{
		ID:             formatUUID(row.ID),
		DefinitionID:   formatUUID(row.DefinitionID),
		TaskType:       row.TaskType,
		QueueID:        textValue(row.QueueID),
		IdempotencyKey: textValue(row.IdempotencyKey),
		Payload:        row.Payload,
		Status:         taskmodule.Status(row.Status),
		Attempt:        int(row.Attempt),
		ProcessedRows:  row.ProcessedRows,
		ErrorSummary:   textValue(row.ErrorSummary),
		StartedAt:      timeValue(row.StartedAt),
		FinishedAt:     timeValue(row.FinishedAt),
	}
}

func createExecutionParams(execution taskmodule.NewExecution) (dbgen.CreateTaskExecutionParams, error) {
	definitionID, err := nullableUUID(execution.DefinitionID)
	if err != nil {
		return dbgen.CreateTaskExecutionParams{}, err
	}
	return dbgen.CreateTaskExecutionParams{
		DefinitionID:   definitionID,
		TaskType:       execution.TaskType,
		QueueID:        nullableText(execution.QueueID),
		IdempotencyKey: nullableText(execution.IdempotencyKey),
		Payload:        execution.Payload,
	}, nil
}

func mapExecutionWriteError(err error, idempotencyKey string) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			return fmt.Errorf("%w: %s", taskmodule.ErrDuplicateSubmission, idempotencyKey)
		case "23503":
			return taskmodule.ErrDefinitionNotFound
		}
	}
	return err
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
