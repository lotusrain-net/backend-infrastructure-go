package task

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"
)

type observerStub struct{ statuses []Status }

func (stub *observerStub) ObserveTask(_ string, status string, _ time.Duration) {
	stub.statuses = append(stub.statuses, Status(status))
}

type executionStoreStub struct {
	execution Execution
	createErr error
	updates   []ExecutionUpdate
	outbox    []PendingPublish
	published []string
	claimErr  error
}

func (store *executionStoreStub) CreateExecution(context.Context, NewExecution) (Execution, error) {
	if store.createErr != nil {
		return Execution{}, store.createErr
	}
	return store.execution, nil
}

func (store *executionStoreStub) GetExecution(context.Context, string) (Execution, error) {
	return store.execution, nil
}

func (store *executionStoreStub) ClaimExecution(_ context.Context, id string, attempt int, startedAt time.Time) error {
	if store.claimErr != nil {
		return store.claimErr
	}
	store.updates = append(store.updates, ExecutionUpdate{
		ID:              id,
		Status:          StatusRunning,
		StartedAt:       &startedAt,
		Attempt:         attempt,
		ExpectedStatus:  store.execution.Status,
		ExpectedAttempt: store.execution.Attempt,
	})
	store.execution.Status = StatusRunning
	store.execution.Attempt = attempt
	store.execution.StartedAt = &startedAt
	return nil
}

func (store *executionStoreStub) UpdateExecution(_ context.Context, update ExecutionUpdate) error {
	store.updates = append(store.updates, update)
	store.execution.Status = update.Status
	store.execution.Attempt = update.Attempt
	return nil
}

func (store *executionStoreStub) CreateExecutionWithPendingPublish(_ context.Context, execution NewExecution, message Message, options PublishOptions) (Execution, error) {
	store.outbox = append(store.outbox, PendingPublish{Message: message, Options: options})
	return store.CreateExecution(context.Background(), execution)
}

func (store *executionStoreStub) ListPendingPublishes(context.Context, int) ([]PendingPublish, error) {
	return append([]PendingPublish(nil), store.outbox...), nil
}

func (store *executionStoreStub) MarkPublishSucceeded(_ context.Context, queueID string) error {
	store.published = append(store.published, queueID)
	return nil
}

type publisherStub struct {
	called  bool
	message Message
	options PublishOptions
	err     error
}

func (publisher *publisherStub) Publish(_ context.Context, message Message, options PublishOptions) error {
	publisher.called = true
	publisher.message = message
	publisher.options = options
	return publisher.err
}

type directExecutionStoreStub struct {
	execution Execution
	createErr error
	updates   []ExecutionUpdate
}

func (store *directExecutionStoreStub) CreateExecution(context.Context, NewExecution) (Execution, error) {
	if store.createErr != nil {
		return Execution{}, store.createErr
	}
	return store.execution, nil
}

func (store *directExecutionStoreStub) GetExecution(context.Context, string) (Execution, error) {
	return store.execution, nil
}

func (*directExecutionStoreStub) ClaimExecution(context.Context, string, int, time.Time) error {
	return nil
}

func (store *directExecutionStoreStub) UpdateExecution(_ context.Context, update ExecutionUpdate) error {
	store.updates = append(store.updates, update)
	store.execution.Status = update.Status
	store.execution.Attempt = update.Attempt
	return nil
}

func TestSubmissionServiceRejectsDuplicateBeforePublishing(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{createErr: ErrDuplicateSubmission}
	publisher := &publisherStub{}
	service := NewSubmissionService(store, publisher, func() string { return "queue-1" })

	_, err := service.Submit(context.Background(), Submission{
		TaskType:       "report.generate",
		Payload:        json.RawMessage(`{"report_id":7}`),
		IdempotencyKey: "same-request",
		MaxRetries:     3,
		Timeout:        time.Minute,
	})
	if !errors.Is(err, ErrDuplicateSubmission) {
		t.Fatalf("Submit() error = %v, want duplicate submission", err)
	}
	if publisher.called {
		t.Fatal("publisher called for duplicate submission")
	}
}

func TestSubmissionServiceMarksExecutionFailedWhenPublishingFails(t *testing.T) {
	t.Parallel()

	store := &directExecutionStoreStub{execution: Execution{ID: "execution-1", Status: StatusQueued}}
	wantErr := errors.New("redis unavailable")
	publisher := &publisherStub{err: wantErr}
	service := NewSubmissionService(store, publisher, func() string { return "queue-1" })

	_, err := service.Submit(context.Background(), Submission{
		TaskType: "report.generate", Payload: json.RawMessage(`{}`), MaxRetries: 3, Timeout: time.Minute,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Submit() error = %v, want Redis error", err)
	}
	if len(store.updates) != 1 {
		t.Fatalf("update count = %d, want 1", len(store.updates))
	}
	if update := store.updates[0]; update.Status != StatusFailed || update.ErrorSummary != wantErr.Error() {
		t.Fatalf("failure update = %+v", update)
	}
}

func TestSubmissionServicePersistsOutboxWhenPublishingFails(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", Status: StatusQueued}}
	publisher := &publisherStub{err: errors.New("redis unavailable")}
	service := NewSubmissionService(store, publisher, func() string { return "queue-1" })

	execution, err := service.Submit(context.Background(), Submission{
		TaskType: "report.generate", Payload: json.RawMessage(`{"report_id":7}`), MaxRetries: 3, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("Submit() error = %v, want nil", err)
	}
	if execution.ID != "execution-1" {
		t.Fatalf("execution = %+v", execution)
	}
	if len(store.outbox) != 1 {
		t.Fatalf("outbox count = %d, want 1", len(store.outbox))
	}
	if len(store.updates) != 0 {
		t.Fatalf("updates = %+v, want no status update on publish failure", store.updates)
	}
	if len(store.published) != 0 {
		t.Fatalf("published markers = %v, want none", store.published)
	}
}

func TestSubmissionServiceRejectsSubmissionAboveConfiguredLimits(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", Status: StatusQueued}}
	publisher := &publisherStub{}
	service := NewSubmissionService(store, publisher, func() string { return "queue-1" })

	_, err := service.Submit(context.Background(), Submission{
		TaskType:     "report.generate",
		Payload:      json.RawMessage(`{}`),
		MaxRetries:   MaxSubmissionRetries + 1,
		Timeout:      MaxSubmissionTimeout + time.Second,
		UniqueFor:    MaxSubmissionUniqueFor + time.Second,
		ProcessAfter: MaxSubmissionProcessAfter + time.Second,
	})
	if err == nil {
		t.Fatal("Submit() error = nil, want validation error")
	}
	if store.outbox != nil || publisher.called {
		t.Fatalf("store/publisher invoked despite invalid submission: outbox=%v called=%v", store.outbox, publisher.called)
	}
}

func TestSubmissionServiceRejectsZeroTimeoutAtDomainBoundary(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", Status: StatusQueued}}
	publisher := &publisherStub{}
	service := NewSubmissionService(store, publisher, func() string { return "queue-1" })

	_, err := service.Submit(context.Background(), Submission{
		TaskType: "report.generate",
		Payload:  json.RawMessage(`{}`),
		Timeout:  0,
	})
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("Submit() error = %v, want invalid submission", err)
	}
	if store.outbox != nil || publisher.called {
		t.Fatalf("store/publisher invoked despite zero timeout: outbox=%v called=%v", store.outbox, publisher.called)
	}
}

func TestProcessorSynchronizesRunningAndSucceededStatuses(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusQueued}}
	registry := NewRegistry()
	if err := registry.Register("report.generate", handlerFunc(func(context.Context, json.RawMessage) (Result, error) {
		return Result{ProcessedRows: 9}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	observer := &observerStub{}
	processor := NewProcessor(store, registry, func() time.Time { return time.Unix(200, 0) }, observer)

	if err := processor.Process(context.Background(), Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}, 2); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if len(store.updates) != 2 {
		t.Fatalf("update count = %d, want 2", len(store.updates))
	}
	if store.updates[0].Status != StatusRunning || store.updates[0].Attempt != 2 {
		t.Fatalf("first update = %+v, want running attempt 2", store.updates[0])
	}
	if store.updates[1].Status != StatusSucceeded || store.updates[1].ProcessedRows != 9 {
		t.Fatalf("second update = %+v, want succeeded with 9 rows", store.updates[1])
	}
	if store.updates[1].ExpectedStatus != StatusRunning || store.updates[1].ExpectedAttempt != 2 {
		t.Fatalf("second update CAS = %+v, want expected running attempt 2", store.updates[1])
	}
	if got := observer.statuses; !slices.Equal(got, []Status{StatusRunning, StatusSucceeded}) {
		t.Fatalf("observed statuses = %v", got)
	}
}

func TestProcessorSkipsHandlerWhenClaimConflicts(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{
		execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusRunning, Attempt: 1},
		claimErr:  ErrExecutionConflict,
	}
	registry := NewRegistry()
	handled := false
	if err := registry.Register("report.generate", handlerFunc(func(context.Context, json.RawMessage) (Result, error) {
		handled = true
		return Result{}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	processor := NewProcessor(store, registry, func() time.Time { return time.Unix(200, 0) })
	err := processor.Process(context.Background(), Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}, 1)
	if !errors.Is(err, ErrExecutionConflict) {
		t.Fatalf("Process() error = %v, want ErrExecutionConflict", err)
	}
	if handled {
		t.Fatal("handler ran despite claim conflict")
	}
}

func TestProcessorMarksInterruptedHandlerCancelled(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusQueued}}
	registry := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	if err := registry.Register("report.generate", handlerFunc(func(context.Context, json.RawMessage) (Result, error) {
		cancel()
		return Result{}, ctx.Err()
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	observer := &observerStub{}
	processor := NewProcessor(store, registry, func() time.Time { return time.Unix(200, 0) }, observer)

	err := processor.Process(ctx, Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Process() error = %v, want context canceled", err)
	}
	if got := store.updates[len(store.updates)-1].Status; got != StatusCancelled {
		t.Fatalf("final status = %s, want cancelled", got)
	}
	if got := observer.statuses; !slices.Equal(got, []Status{StatusRunning, StatusCancelled}) {
		t.Fatalf("observed statuses = %v", got)
	}
}

func TestProcessorMarksRetryExhaustionFailed(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusRunning}}
	observer := &observerStub{}
	processor := NewProcessor(store, NewRegistry(), func() time.Time { return time.Unix(200, 0) }, observer)
	wantErr := errors.New("dependency failed")

	if err := processor.Fail(context.Background(), Message{ExecutionID: "execution-1"}, 4, wantErr); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if len(store.updates) != 1 {
		t.Fatalf("update count = %d, want 1", len(store.updates))
	}
	update := store.updates[0]
	if update.Status != StatusFailed || update.Attempt != 4 || update.ErrorSummary != wantErr.Error() {
		t.Fatalf("failure update = %+v", update)
	}
	if got := observer.statuses; !slices.Equal(got, []Status{StatusFailed}) {
		t.Fatalf("observed statuses = %v", got)
	}
}
