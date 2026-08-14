package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"

	"github.com/hibiken/asynq"
)

type enqueueClient interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

type AsynqPublisher struct {
	client enqueueClient
	queue  string
}

func NewAsynqPublisher(client enqueueClient, queue string) *AsynqPublisher {
	return &AsynqPublisher{client: client, queue: queue}
}

func (publisher *AsynqPublisher) Publish(ctx context.Context, message taskmodule.Message, options taskmodule.PublishOptions) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode task message: %w", err)
	}
	enqueueOptions := []asynq.Option{
		asynq.MaxRetry(options.MaxRetries),
	}
	if publisher.queue != "" {
		enqueueOptions = append(enqueueOptions, asynq.Queue(publisher.queue))
	}
	if options.QueueID != "" {
		enqueueOptions = append(enqueueOptions, asynq.TaskID(options.QueueID))
	}
	if options.Timeout > 0 {
		enqueueOptions = append(enqueueOptions, asynq.Timeout(options.Timeout))
	}
	if options.UniqueFor > 0 {
		enqueueOptions = append(enqueueOptions, asynq.Unique(options.UniqueFor))
	}
	if options.ProcessAfter > 0 {
		enqueueOptions = append(enqueueOptions, asynq.ProcessIn(options.ProcessAfter))
	}
	_, err = publisher.client.EnqueueContext(ctx, asynq.NewTask(message.TaskType, payload), enqueueOptions...)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return fmt.Errorf("%w: %s", taskmodule.ErrDuplicateSubmission, options.QueueID)
	}
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}
	return nil
}

type Processor interface {
	Process(context.Context, taskmodule.Message, int) error
	Fail(context.Context, taskmodule.Message, int, error) error
}

type AsynqHandler struct {
	processor Processor
}

func NewAsynqHandler(processor Processor) *AsynqHandler {
	return &AsynqHandler{processor: processor}
}

func (handler *AsynqHandler) ProcessTask(ctx context.Context, queuedTask *asynq.Task) error {
	message, err := decodeMessage(queuedTask)
	if err != nil {
		return errors.Join(err, asynq.SkipRetry)
	}
	retryCount, _ := asynq.GetRetryCount(ctx)
	if err := handler.processor.Process(ctx, message, retryCount+1); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return errors.Join(err, asynq.RevokeTask)
		}
		if errors.Is(err, taskmodule.ErrExecutionConflict) {
			return errors.Join(err, asynq.RevokeTask)
		}
		return err
	}
	return nil
}

type FailureHandler struct {
	processor Processor
	logger    *slog.Logger
}

func NewFailureHandler(processor Processor, loggers ...*slog.Logger) *FailureHandler {
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &FailureHandler{processor: processor, logger: logger}
}

func (handler *FailureHandler) HandleError(ctx context.Context, queuedTask *asynq.Task, processingErr error) {
	if errors.Is(processingErr, context.Canceled) ||
		errors.Is(processingErr, context.DeadlineExceeded) ||
		errors.Is(processingErr, asynq.RevokeTask) {
		return
	}
	retryCount, retryOK := asynq.GetRetryCount(ctx)
	maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
	if !retryOK || !maxRetryOK || retryCount < maxRetry {
		return
	}
	updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := handler.HandleExhausted(updateCtx, queuedTask, processingErr, retryCount, maxRetry); err != nil {
		handler.logger.ErrorContext(updateCtx, "task failure sync failed", "task_type", queuedTask.Type(), "error", err)
	}
}

func (handler *FailureHandler) HandleExhausted(
	ctx context.Context,
	queuedTask *asynq.Task,
	processingErr error,
	retryCount int,
	maxRetry int,
) error {
	if retryCount < maxRetry {
		return nil
	}
	message, err := decodeMessage(queuedTask)
	if err != nil {
		return err
	}
	return handler.processor.Fail(ctx, message, retryCount+1, processingErr)
}

type PendingPublishStore interface {
	ListPendingPublishes(context.Context, int) ([]taskmodule.PendingPublish, error)
	MarkPublishSucceeded(context.Context, string) error
}

type OutboxDispatcher struct {
	store     PendingPublishStore
	publisher taskmodule.Publisher
}

func NewOutboxDispatcher(store PendingPublishStore, publisher taskmodule.Publisher) *OutboxDispatcher {
	return &OutboxDispatcher{store: store, publisher: publisher}
}

func (dispatcher *OutboxDispatcher) DispatchPending(ctx context.Context, limit int) error {
	pending, err := dispatcher.store.ListPendingPublishes(ctx, limit)
	if err != nil {
		return fmt.Errorf("list pending task publishes: %w", err)
	}
	for _, publish := range pending {
		if publish.ExecutionStatus == taskmodule.StatusQueued {
			if err := dispatcher.publisher.Publish(ctx, publish.Message, publish.Options); err != nil {
				return fmt.Errorf("publish queued task %s: %w", publish.Options.QueueID, err)
			}
		}
		if err := dispatcher.store.MarkPublishSucceeded(ctx, publish.Options.QueueID); err != nil {
			return fmt.Errorf("mark queued task %s published: %w", publish.Options.QueueID, err)
		}
	}
	return nil
}

func decodeMessage(queuedTask *asynq.Task) (taskmodule.Message, error) {
	var message taskmodule.Message
	if err := json.Unmarshal(queuedTask.Payload(), &message); err != nil {
		return taskmodule.Message{}, fmt.Errorf("decode task message: %w", err)
	}
	if message.TaskType == "" {
		message.TaskType = queuedTask.Type()
	}
	return message, nil
}
