package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrDuplicateSubmission = errors.New("duplicate task submission")

type NewExecution struct {
	DefinitionID   string
	TaskType       string
	QueueID        string
	IdempotencyKey string
	Payload        json.RawMessage
}

type ExecutionUpdate struct {
	ID            string
	Status        Status
	StartedAt     *time.Time
	FinishedAt    *time.Time
	ErrorSummary  string
	ProcessedRows int64
	Attempt       int
}

type ExecutionStore interface {
	CreateExecution(context.Context, NewExecution) (Execution, error)
	GetExecution(context.Context, string) (Execution, error)
	UpdateExecution(context.Context, ExecutionUpdate) error
}

type Publisher interface {
	Publish(context.Context, Message, PublishOptions) error
}

type Submission struct {
	DefinitionID   string
	TaskType       string
	Payload        json.RawMessage
	IdempotencyKey string
	MaxRetries     int
	Timeout        time.Duration
	UniqueFor      time.Duration
	ProcessAfter   time.Duration
}

type SubmissionService struct {
	store     ExecutionStore
	publisher Publisher
	newID     func() string
}

func NewSubmissionService(store ExecutionStore, publisher Publisher, newID func() string) *SubmissionService {
	return &SubmissionService{store: store, publisher: publisher, newID: newID}
}

func (service *SubmissionService) Submit(ctx context.Context, submission Submission) (Execution, error) {
	queueID := service.newID()
	execution, err := service.store.CreateExecution(ctx, NewExecution{
		DefinitionID:   submission.DefinitionID,
		TaskType:       submission.TaskType,
		QueueID:        queueID,
		IdempotencyKey: submission.IdempotencyKey,
		Payload:        submission.Payload,
	})
	if err != nil {
		return Execution{}, err
	}
	message := Message{ExecutionID: execution.ID, TaskType: submission.TaskType, Payload: submission.Payload}
	options := PublishOptions{
		QueueID:      queueID,
		MaxRetries:   submission.MaxRetries,
		Timeout:      submission.Timeout,
		UniqueFor:    submission.UniqueFor,
		ProcessAfter: submission.ProcessAfter,
	}
	if err := service.publisher.Publish(ctx, message, options); err != nil {
		if transitionErr := ValidateTransition(execution.Status, StatusFailed); transitionErr != nil {
			return Execution{}, errors.Join(fmt.Errorf("publish task: %w", err), transitionErr)
		}
		finishedAt := time.Now()
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if updateErr := service.store.UpdateExecution(updateCtx, ExecutionUpdate{
			ID:           execution.ID,
			Status:       StatusFailed,
			FinishedAt:   &finishedAt,
			ErrorSummary: err.Error(),
		}); updateErr != nil {
			return Execution{}, errors.Join(fmt.Errorf("publish task: %w", err), fmt.Errorf("mark task failed: %w", updateErr))
		}
		return Execution{}, fmt.Errorf("publish task: %w", err)
	}
	return execution, nil
}

type Processor struct {
	store    ExecutionStore
	registry *Registry
	now      func() time.Time
}

func NewProcessor(store ExecutionStore, registry *Registry, now func() time.Time) *Processor {
	return &Processor{store: store, registry: registry, now: now}
}

func (processor *Processor) Process(ctx context.Context, message Message, attempt int) error {
	execution, err := processor.store.GetExecution(ctx, message.ExecutionID)
	if err != nil {
		return fmt.Errorf("get task execution: %w", err)
	}
	if err := ValidateTransition(execution.Status, StatusRunning); err != nil {
		return err
	}
	startedAt := processor.now()
	if err := processor.store.UpdateExecution(ctx, ExecutionUpdate{
		ID:        message.ExecutionID,
		Status:    StatusRunning,
		StartedAt: &startedAt,
		Attempt:   attempt,
	}); err != nil {
		return fmt.Errorf("mark task running: %w", err)
	}

	handler, err := processor.registry.Resolve(message.TaskType)
	if err != nil {
		return err
	}
	result, err := handler.Handle(ctx, message.Payload)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			finishedAt := processor.now()
			updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if updateErr := processor.store.UpdateExecution(updateCtx, ExecutionUpdate{
				ID:           message.ExecutionID,
				Status:       StatusCancelled,
				FinishedAt:   &finishedAt,
				ErrorSummary: err.Error(),
				Attempt:      attempt,
			}); updateErr != nil {
				return errors.Join(err, fmt.Errorf("mark task cancelled: %w", updateErr))
			}
		}
		return err
	}

	finishedAt := processor.now()
	if err := processor.store.UpdateExecution(ctx, ExecutionUpdate{
		ID:            message.ExecutionID,
		Status:        StatusSucceeded,
		StartedAt:     &startedAt,
		FinishedAt:    &finishedAt,
		ProcessedRows: result.ProcessedRows,
		Attempt:       attempt,
	}); err != nil {
		return fmt.Errorf("mark task succeeded: %w", err)
	}
	return nil
}

func (processor *Processor) Fail(ctx context.Context, message Message, attempt int, cause error) error {
	execution, err := processor.store.GetExecution(ctx, message.ExecutionID)
	if err != nil {
		return fmt.Errorf("get task execution: %w", err)
	}
	if err := ValidateTransition(execution.Status, StatusFailed); err != nil {
		return err
	}
	finishedAt := processor.now()
	if err := processor.store.UpdateExecution(ctx, ExecutionUpdate{
		ID:           message.ExecutionID,
		Status:       StatusFailed,
		FinishedAt:   &finishedAt,
		ErrorSummary: cause.Error(),
		Attempt:      attempt,
	}); err != nil {
		return fmt.Errorf("mark task failed: %w", err)
	}
	return nil
}
