package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type executionStoreStub struct {
	execution Execution
	createErr error
	updates   []ExecutionUpdate
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

func (store *executionStoreStub) UpdateExecution(_ context.Context, update ExecutionUpdate) error {
	store.updates = append(store.updates, update)
	store.execution.Status = update.Status
	store.execution.Attempt = update.Attempt
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

	store := &executionStoreStub{execution: Execution{ID: "execution-1", Status: StatusQueued}}
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

func TestProcessorSynchronizesRunningAndSucceededStatuses(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusQueued}}
	registry := NewRegistry()
	if err := registry.Register("report.generate", handlerFunc(func(context.Context, json.RawMessage) (Result, error) {
		return Result{ProcessedRows: 9}, nil
	})); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	processor := NewProcessor(store, registry, func() time.Time { return time.Unix(200, 0) })

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
	processor := NewProcessor(store, registry, func() time.Time { return time.Unix(200, 0) })

	err := processor.Process(ctx, Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Process() error = %v, want context canceled", err)
	}
	if got := store.updates[len(store.updates)-1].Status; got != StatusCancelled {
		t.Fatalf("final status = %s, want cancelled", got)
	}
}

func TestProcessorMarksRetryExhaustionFailed(t *testing.T) {
	t.Parallel()

	store := &executionStoreStub{execution: Execution{ID: "execution-1", TaskType: "report.generate", Status: StatusRunning}}
	processor := NewProcessor(store, NewRegistry(), func() time.Time { return time.Unix(200, 0) })
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
}
