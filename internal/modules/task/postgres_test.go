package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"backend-infrastructure-go/internal/platform/database/dbgen"

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
	updateParams dbgen.UpdateTaskExecutionStatusParams
}

func (queries *executionQueriesStub) CreateTaskExecution(context.Context, dbgen.CreateTaskExecutionParams) (dbgen.TaskExecution, error) {
	return dbgen.TaskExecution{}, queries.createErr
}

func (queries *executionQueriesStub) GetTaskExecution(context.Context, pgtype.UUID) (dbgen.TaskExecution, error) {
	return dbgen.TaskExecution{ID: testUUID, TaskType: "report.generate", Status: string(StatusRunning)}, nil
}

func (queries *executionQueriesStub) UpdateTaskExecutionStatus(_ context.Context, params dbgen.UpdateTaskExecutionStatusParams) error {
	queries.updateParams = params
	return nil
}

type scheduleQueriesStub struct {
	result []dbgen.TaskSchedule
}

func (queries *scheduleQueriesStub) ListEnabledTaskSchedules(context.Context) ([]dbgen.TaskSchedule, error) {
	return queries.result, nil
}

func TestPostgresDefinitionStoreMapsDefinitionFields(t *testing.T) {
	t.Parallel()

	queries := &definitionQueriesStub{result: dbgen.TaskDefinition{
		ID: testUUID, Name: "daily-report", TaskType: "report.generate", DefaultPayload: []byte(`{}`),
		MaxRetries: 4, TimeoutSeconds: 120, IsActive: true,
	}}
	store := NewPostgresDefinitionStore(queries)
	definition, err := store.CreateDefinition(context.Background(), NewDefinition{
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
	_, err := store.CreateExecution(context.Background(), NewExecution{
		TaskType: "report.generate", IdempotencyKey: "same-request", Payload: json.RawMessage(`{}`),
	})
	if !errors.Is(err, ErrDuplicateSubmission) {
		t.Fatalf("CreateExecution() error = %v, want duplicate submission", err)
	}
}

func TestPostgresExecutionStoreSynchronizesStatusFields(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{}
	store := NewPostgresExecutionStore(queries)
	startedAt := time.Unix(200, 0).UTC()
	finishedAt := time.Unix(201, 0).UTC()
	if err := store.UpdateExecution(context.Background(), ExecutionUpdate{
		ID: testUUIDString, Status: StatusSucceeded, StartedAt: &startedAt, FinishedAt: &finishedAt,
		ProcessedRows: 15, Attempt: 2,
	}); err != nil {
		t.Fatalf("UpdateExecution() error = %v", err)
	}
	params := queries.updateParams
	if params.ID != testUUID || params.Status != string(StatusSucceeded) || params.ProcessedRows != 15 || params.Attempt != 2 {
		t.Fatalf("UpdateTaskExecutionStatus params = %+v", params)
	}
	if !params.StartedAt.Valid || !params.FinishedAt.Valid {
		t.Fatalf("timestamp params = %+v, want valid timestamps", params)
	}
}

func TestPostgresScheduleStoreMapsEnabledSchedules(t *testing.T) {
	t.Parallel()

	queries := &scheduleQueriesStub{result: []dbgen.TaskSchedule{{
		ID: testUUID, DefinitionID: testUUID, CronExpression: "*/5 * * * *", Timezone: "UTC",
		Payload: []byte(`{"scope":"all"}`), IsEnabled: true,
	}}}
	store := NewPostgresScheduleStore(queries)
	schedules, err := store.ListEnabledSchedules(context.Background())
	if err != nil {
		t.Fatalf("ListEnabledSchedules() error = %v", err)
	}
	if len(schedules) != 1 || schedules[0].ID != testUUIDString || !schedules[0].Enabled {
		t.Fatalf("schedules = %+v", schedules)
	}
}
