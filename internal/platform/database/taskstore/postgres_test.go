package taskstore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/platform/database/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var testUUID = pgtype.UUID{
	Bytes: [16]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
	Valid: true,
}

const testUUIDString = "00112233-4455-6677-8899-aabbccddeeff"

type definitionQueriesStub struct {
	params dbgen.CreateTaskDefinitionParams
	result dbgen.TaskDefinition
}

func (queries *definitionQueriesStub) CreateTaskDefinition(_ context.Context, params dbgen.CreateTaskDefinitionParams) (dbgen.TaskDefinition, error) {
	queries.params = params
	return queries.result, nil
}

type executionQueriesStub struct {
	createErr    error
	getErr       error
	createParams dbgen.CreateTaskExecutionParams
	updateParams dbgen.UpdateTaskExecutionStatusParams
}

func (queries *executionQueriesStub) CreateTaskExecution(_ context.Context, params dbgen.CreateTaskExecutionParams) (dbgen.TaskExecution, error) {
	queries.createParams = params
	return dbgen.TaskExecution{ID: testUUID, DefinitionID: params.DefinitionID, TaskType: params.TaskType, Status: string(taskmodule.StatusQueued)}, queries.createErr
}

func (queries *executionQueriesStub) GetTaskExecution(context.Context, pgtype.UUID) (dbgen.TaskExecution, error) {
	return dbgen.TaskExecution{ID: testUUID, TaskType: "report.generate", Status: string(taskmodule.StatusRunning)}, queries.getErr
}

func TestPostgresExecutionStoreMapsMissingRowToExecutionNotFound(t *testing.T) {
	store := NewPostgresExecutionStore(&executionQueriesStub{getErr: pgx.ErrNoRows})
	_, err := store.GetExecution(context.Background(), testUUIDString)
	if !errors.Is(err, taskmodule.ErrExecutionNotFound) {
		t.Fatalf("GetExecution() error = %v, want ErrExecutionNotFound", err)
	}
}

func TestPostgresExecutionStoreTreatsInvalidIdentifierAsNotFound(t *testing.T) {
	store := NewPostgresExecutionStore(&executionQueriesStub{})
	_, err := store.GetExecution(context.Background(), "not-a-uuid")
	if !errors.Is(err, taskmodule.ErrExecutionNotFound) {
		t.Fatalf("GetExecution() error = %v, want ErrExecutionNotFound", err)
	}
}

func (queries *executionQueriesStub) UpdateTaskExecutionStatus(_ context.Context, params dbgen.UpdateTaskExecutionStatusParams) error {
	queries.updateParams = params
	return nil
}

type scheduleQueriesStub struct {
	result []dbgen.ListEnabledTaskSchedulesRow
}

func (queries *scheduleQueriesStub) ListEnabledTaskSchedules(context.Context) ([]dbgen.ListEnabledTaskSchedulesRow, error) {
	return queries.result, nil
}

func TestPostgresDefinitionStoreMapsDefinitionFields(t *testing.T) {
	t.Parallel()

	queries := &definitionQueriesStub{result: dbgen.TaskDefinition{
		ID: testUUID, Name: "daily-report", TaskType: "report.generate", DefaultPayload: []byte(`{}`),
		MaxRetries: 4, TimeoutSeconds: 120, IsActive: true,
	}}
	store := NewPostgresDefinitionStore(queries)
	definition, err := store.CreateDefinition(context.Background(), taskmodule.NewDefinition{
		Name: "daily-report", TaskType: "report.generate", DefaultPayload: json.RawMessage(`{}`),
		MaxRetries: 4, Timeout: 2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateDefinition() error = %v", err)
	}
	if queries.params.TimeoutSeconds != 120 || definition.ID != testUUIDString || !definition.Active {
		t.Fatalf("params = %+v, definition = %+v", queries.params, definition)
	}
}

func TestPostgresExecutionStoreMapsUniqueViolationToDuplicate(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{createErr: &pgconn.PgError{Code: "23505"}}
	store := NewPostgresExecutionStore(queries)
	_, err := store.CreateExecution(context.Background(), taskmodule.NewExecution{
		TaskType: "report.generate", IdempotencyKey: "same-request", Payload: json.RawMessage(`{}`),
	})
	if !errors.Is(err, taskmodule.ErrDuplicateSubmission) {
		t.Fatalf("CreateExecution() error = %v, want duplicate submission", err)
	}
}

func TestPostgresExecutionStoreMapsMissingDefinition(t *testing.T) {
	queries := &executionQueriesStub{createErr: &pgconn.PgError{Code: "23503"}}
	store := NewPostgresExecutionStore(queries)
	_, err := store.CreateExecution(context.Background(), taskmodule.NewExecution{
		DefinitionID: testUUIDString, TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{}`),
	})
	if !errors.Is(err, taskmodule.ErrDefinitionNotFound) {
		t.Fatalf("CreateExecution() error = %v", err)
	}
}

func TestPostgresExecutionStoreAllowsSubmissionWithoutDefinition(t *testing.T) {
	queries := &executionQueriesStub{}
	store := NewPostgresExecutionStore(queries)
	if _, err := store.CreateExecution(context.Background(), taskmodule.NewExecution{TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if queries.createParams.DefinitionID.Valid {
		t.Fatalf("definition_id=%+v, want SQL NULL", queries.createParams.DefinitionID)
	}
}

func TestPostgresExecutionStoreSynchronizesStatusFields(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{}
	store := NewPostgresExecutionStore(queries)
	startedAt := time.Unix(200, 0).UTC()
	finishedAt := time.Unix(201, 0).UTC()
	if err := store.UpdateExecution(context.Background(), taskmodule.ExecutionUpdate{
		ID: testUUIDString, Status: taskmodule.StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt,
		ProcessedRows: 15, Attempt: 2,
	}); err != nil {
		t.Fatalf("UpdateExecution() error = %v", err)
	}
	params := queries.updateParams
	if params.ID != testUUID || params.Status != string(taskmodule.StatusSucceeded) || params.ProcessedRows != 15 || params.Attempt != 2 {
		t.Fatalf("UpdateTaskExecutionStatus params = %+v", params)
	}
	if !params.StartedAt.Valid || !params.FinishedAt.Valid {
		t.Fatalf("timestamp params = %+v, want valid timestamps", params)
	}
}

func TestPostgresScheduleStoreMapsEnabledSchedules(t *testing.T) {
	t.Parallel()

	queries := &scheduleQueriesStub{result: []dbgen.ListEnabledTaskSchedulesRow{{
		ID: testUUID, DefinitionID: testUUID, CronExpression: "*/5 * * * *", Timezone: "UTC",
		Payload: []byte(`{"scope":"all"}`), IsEnabled: true, TaskType: taskmodule.SystemTestTaskType, MaxRetries: 4, TimeoutSeconds: 120,
	}}}
	store := NewPostgresScheduleStore(queries)
	schedules, err := store.ListEnabledSchedules(context.Background())
	if err != nil {
		t.Fatalf("ListEnabledSchedules() error = %v", err)
	}
	if len(schedules) != 1 || schedules[0].ID != testUUIDString || !schedules[0].Enabled || schedules[0].TaskType != taskmodule.SystemTestTaskType || schedules[0].MaxRetries != 4 || schedules[0].Timeout != 2*time.Minute {
		t.Fatalf("schedules = %+v", schedules)
	}
}
