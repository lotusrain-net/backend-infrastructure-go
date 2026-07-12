package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"

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
	if errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict) {
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
		return err
	}
	return nil
}

type FailureHandler struct {
	processor Processor
}

func NewFailureHandler(processor Processor) *FailureHandler {
	return &FailureHandler{processor: processor}
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
	_ = handler.HandleExhausted(updateCtx, queuedTask, processingErr, retryCount, maxRetry)
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
