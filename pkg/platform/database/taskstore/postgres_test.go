package taskstore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	taskmodule "github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/task"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"

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
	createErr        error
	getErr           error
	claimErr         error
	claimRows        int64
	updateRows       int64
	total            int64
	createParams     dbgen.CreateTaskExecutionParams
	createOutbox     dbgen.CreateTaskExecutionWithPendingPublishParams
	updateParams     dbgen.UpdateTaskExecutionStatusParams
	claimParams      dbgen.ClaimTaskExecutionParams
	listParams       dbgen.ListTaskExecutionsParams
	countParams      dbgen.CountTaskExecutionsParams
	listRows         []dbgen.TaskExecution
	outboxList       []dbgen.ListPendingTaskOutboxMessagesRow
	listPendingLimit int32
	markPublishedID  string
}

func (queries *executionQueriesStub) CreateTaskExecution(_ context.Context, params dbgen.CreateTaskExecutionParams) (dbgen.TaskExecution, error) {
	queries.createParams = params
	return dbgen.TaskExecution{ID: testUUID, DefinitionID: params.DefinitionID, TaskType: params.TaskType, Status: string(taskmodule.StatusQueued)}, queries.createErr
}

func (queries *executionQueriesStub) GetTaskExecution(context.Context, pgtype.UUID) (dbgen.TaskExecution, error) {
	return dbgen.TaskExecution{ID: testUUID, TaskType: "report.generate", Status: string(taskmodule.StatusRunning)}, queries.getErr
}

func (queries *executionQueriesStub) ListTaskExecutions(_ context.Context, params dbgen.ListTaskExecutionsParams) ([]dbgen.TaskExecution, error) {
	queries.listParams = params
	return queries.listRows, nil
}

func (queries *executionQueriesStub) CountTaskExecutions(_ context.Context, params dbgen.CountTaskExecutionsParams) (int64, error) {
	queries.countParams = params
	return queries.total, nil
}

func (queries *executionQueriesStub) ClaimTaskExecution(_ context.Context, params dbgen.ClaimTaskExecutionParams) (int64, error) {
	queries.claimParams = params
	if queries.claimErr != nil {
		return 0, queries.claimErr
	}
	if queries.claimRows != 0 {
		return queries.claimRows, nil
	}
	return 1, nil
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

func (queries *executionQueriesStub) UpdateTaskExecutionStatus(_ context.Context, params dbgen.UpdateTaskExecutionStatusParams) (int64, error) {
	queries.updateParams = params
	if queries.updateRows != 0 {
		return queries.updateRows, nil
	}
	return 1, nil
}

func (queries *executionQueriesStub) CreateTaskExecutionWithPendingPublish(_ context.Context, params dbgen.CreateTaskExecutionWithPendingPublishParams) (dbgen.CreateTaskExecutionWithPendingPublishRow, error) {
	queries.createOutbox = params
	return dbgen.CreateTaskExecutionWithPendingPublishRow{ID: testUUID, DefinitionID: params.DefinitionID, TaskType: params.TaskType, Status: string(taskmodule.StatusQueued)}, queries.createErr
}

func (queries *executionQueriesStub) ListPendingTaskOutboxMessages(_ context.Context, limit int32) ([]dbgen.ListPendingTaskOutboxMessagesRow, error) {
	queries.listPendingLimit = limit
	return queries.outboxList, nil
}

func (queries *executionQueriesStub) MarkTaskOutboxMessagePublished(_ context.Context, queueID string) error {
	queries.markPublishedID = queueID
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
		ProcessedRows: 15, Attempt: 2, ExpectedStatus: taskmodule.StatusRunning, ExpectedAttempt: 2,
	}); err != nil {
		t.Fatalf("UpdateExecution() error = %v", err)
	}
	params := queries.updateParams
	if params.ID != testUUID || params.Status != string(taskmodule.StatusSucceeded) || params.ProcessedRows != 15 || params.Attempt != 2 || params.ExpectedStatus != string(taskmodule.StatusRunning) || params.ExpectedAttempt != 2 {
		t.Fatalf("UpdateTaskExecutionStatus params = %+v", params)
	}
	if !params.StartedAt.Valid || !params.FinishedAt.Valid {
		t.Fatalf("timestamp params = %+v, want valid timestamps", params)
	}
}

func TestPostgresExecutionStoreClaimsAttemptWithCAS(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{}
	store := NewPostgresExecutionStore(queries)
	startedAt := time.Unix(200, 0).UTC()

	if err := store.ClaimExecution(context.Background(), testUUIDString, 2, startedAt); err != nil {
		t.Fatalf("ClaimExecution() error = %v", err)
	}
	if queries.claimParams.ID != testUUID || queries.claimParams.Attempt != 2 || !queries.claimParams.StartedAt.Valid {
		t.Fatalf("ClaimTaskExecution params = %+v", queries.claimParams)
	}
}

func TestPostgresExecutionStoreCreatesExecutionWithPendingPublish(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{}
	store := NewPostgresExecutionStore(queries)

	execution, err := store.CreateExecutionWithPendingPublish(context.Background(),
		taskmodule.NewExecution{TaskType: "report.generate", QueueID: "queue-1", Payload: json.RawMessage(`{"report_id":7}`)},
		taskmodule.Message{ExecutionID: testUUIDString, TaskType: "report.generate", Payload: json.RawMessage(`{"report_id":7}`)},
		taskmodule.PublishOptions{QueueID: "queue-1", MaxRetries: 3, Timeout: time.Minute, UniqueFor: 2 * time.Minute, ProcessAfter: 30 * time.Second},
	)
	if err != nil {
		t.Fatalf("CreateExecutionWithPendingPublish() error = %v", err)
	}
	if execution.ID != testUUIDString {
		t.Fatalf("execution = %+v", execution)
	}
	if queries.createOutbox.QueueID.String != "queue-1" || queries.createOutbox.TaskType != "report.generate" || queries.createOutbox.MaxRetries != 3 || queries.createOutbox.TimeoutSeconds != 60 || queries.createOutbox.UniqueForSeconds != 120 || queries.createOutbox.ProcessAfterSeconds != 30 {
		t.Fatalf("CreateTaskExecutionWithPendingPublish params = %+v", queries.createOutbox)
	}
}

func TestPostgresExecutionStoreListsAndMarksPendingPublishes(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{outboxList: []dbgen.ListPendingTaskOutboxMessagesRow{{
		ExecutionID:         testUUID,
		ExecutionStatus:     string(taskmodule.StatusQueued),
		QueueID:             "queue-1",
		TaskType:            "report.generate",
		Payload:             []byte(`{"report_id":7}`),
		MaxRetries:          3,
		TimeoutSeconds:      60,
		UniqueForSeconds:    120,
		ProcessAfterSeconds: 30,
	}}}
	store := NewPostgresExecutionStore(queries)

	pending, err := store.ListPendingPublishes(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListPendingPublishes() error = %v", err)
	}
	if queries.listPendingLimit != 10 {
		t.Fatalf("limit = %d, want 10", queries.listPendingLimit)
	}
	if len(pending) != 1 || pending[0].ExecutionStatus != taskmodule.StatusQueued || pending[0].Message.ExecutionID != testUUIDString || pending[0].Options.QueueID != "queue-1" || pending[0].Options.Timeout != time.Minute || pending[0].Options.UniqueFor != 2*time.Minute || pending[0].Options.ProcessAfter != 30*time.Second {
		t.Fatalf("pending = %+v", pending)
	}
	if err := store.MarkPublishSucceeded(context.Background(), "queue-1"); err != nil {
		t.Fatalf("MarkPublishSucceeded() error = %v", err)
	}
	if queries.markPublishedID != "queue-1" {
		t.Fatalf("markPublishedID = %q", queries.markPublishedID)
	}
}

func TestPostgresExecutionStoreListsExecutionsNewestFirstPage(t *testing.T) {
	t.Parallel()

	queries := &executionQueriesStub{
		listRows: []dbgen.TaskExecution{{ID: testUUID, TaskType: "report.generate", Status: string(taskmodule.StatusSucceeded)}},
		total:    8,
	}
	store := NewPostgresExecutionStore(queries)

	page, err := store.ListExecutions(context.Background(), taskmodule.ExecutionQuery{
		Filter: taskmodule.ExecutionFilter{TaskType: "report.generate", Status: taskmodule.StatusSucceeded},
		Page:   2,
		Size:   5,
	})
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if queries.listParams.TaskType.String != "report.generate" || queries.listParams.Status.String != string(taskmodule.StatusSucceeded) || queries.listParams.Offset != 5 || queries.listParams.Limit != 5 {
		t.Fatalf("list params = %+v", queries.listParams)
	}
	if queries.countParams.TaskType.String != "report.generate" || queries.countParams.Status.String != string(taskmodule.StatusSucceeded) {
		t.Fatalf("count params = %+v", queries.countParams)
	}
	if len(page.Items) != 1 || page.Items[0].ID != testUUIDString || page.Meta.Page != 2 || page.Meta.Size != 5 || page.Meta.Total != 8 {
		t.Fatalf("page = %+v", page)
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
