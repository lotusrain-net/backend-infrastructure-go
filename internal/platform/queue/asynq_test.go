package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type processorStub struct {
	mu             sync.Mutex
	processMessage taskmodule.Message
	processAttempt int
	processErr     error
	failMessage    taskmodule.Message
	failAttempt    int
	failCause      error
	failed         chan struct{}
}

func (processor *processorStub) Process(_ context.Context, message taskmodule.Message, attempt int) error {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	processor.processMessage = message
	processor.processAttempt = attempt
	return processor.processErr
}

func (processor *processorStub) Fail(_ context.Context, message taskmodule.Message, attempt int, cause error) error {
	processor.mu.Lock()
	defer processor.mu.Unlock()
	processor.failMessage = message
	processor.failAttempt = attempt
	processor.failCause = cause
	if processor.failed != nil {
		select {
		case <-processor.failed:
		default:
			close(processor.failed)
		}
	}
	return nil
}

func TestAsynqPublisherAppliesQueuePolicyAndRejectsUniqueDuplicate(t *testing.T) {
	t.Parallel()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	client := asynq.NewClientFromRedisClient(redisClient)
	publisher := NewAsynqPublisher(client, "default")
	message := taskmodule.Message{
		ExecutionID: "execution-1",
		TaskType:    "report.generate",
		Payload:     json.RawMessage(`{"report_id":7}`),
	}
	options := taskmodule.PublishOptions{
		QueueID: "queue-1", MaxRetries: 2, Timeout: time.Minute,
		UniqueFor: time.Minute, ProcessAfter: time.Minute,
	}

	if err := publisher.Publish(context.Background(), message, options); err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	duplicateOptions := options
	duplicateOptions.QueueID = "queue-2"
	if err := publisher.Publish(context.Background(), message, duplicateOptions); !errors.Is(err, taskmodule.ErrDuplicateSubmission) {
		t.Fatalf("duplicate Publish() error = %v, want duplicate submission", err)
	}

	inspector := asynq.NewInspectorFromRedisClient(redisClient)
	info, err := inspector.GetTaskInfo("default", "queue-1")
	if err != nil {
		t.Fatalf("GetTaskInfo() error = %v", err)
	}
	if info.State != asynq.TaskStateScheduled || info.MaxRetry != 2 || info.Timeout != time.Minute {
		t.Fatalf("task info = %+v, want scheduled retry=2 timeout=1m", info)
	}
}

func TestAsynqHandlerForwardsStringTaskAndAttempt(t *testing.T) {
	t.Parallel()

	processor := &processorStub{}
	handler := NewAsynqHandler(processor)
	message := taskmodule.Message{
		ExecutionID: "execution-1",
		TaskType:    "report.generate",
		Payload:     json.RawMessage(`{"report_id":7}`),
	}
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if err := handler.ProcessTask(context.Background(), asynq.NewTask(message.TaskType, payload)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.processMessage.TaskType != message.TaskType || processor.processAttempt != 1 {
		t.Fatalf("processor call = %+v attempt=%d", processor.processMessage, processor.processAttempt)
	}
}

func TestFailureHandlerSynchronizesRetryExhaustion(t *testing.T) {
	t.Parallel()

	processor := &processorStub{}
	handler := NewFailureHandler(processor)
	message := taskmodule.Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}
	payload, _ := json.Marshal(message)
	wantErr := errors.New("dependency failed")

	if err := handler.HandleExhausted(context.Background(), asynq.NewTask(message.TaskType, payload), wantErr, 3, 3); err != nil {
		t.Fatalf("HandleExhausted() error = %v", err)
	}
	if processor.failAttempt != 4 || !errors.Is(processor.failCause, wantErr) {
		t.Fatalf("Fail() attempt=%d cause=%v", processor.failAttempt, processor.failCause)
	}
}

func TestAsynqArchivesTaskAfterRetryExhaustion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	client := asynq.NewClientFromRedisClient(redisClient)
	publisher := NewAsynqPublisher(client, "default")
	processor := &processorStub{processErr: errors.New("dependency failed"), failed: make(chan struct{})}
	failureHandler := NewFailureHandler(processor)
	server := asynq.NewServerFromRedisClient(redisClient, asynq.Config{
		Concurrency:  1,
		Queues:       map[string]int{"default": 1},
		ErrorHandler: failureHandler,
	})
	if err := server.Start(NewAsynqHandler(processor)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(server.Shutdown)

	message := taskmodule.Message{ExecutionID: "execution-1", TaskType: "report.generate", Payload: json.RawMessage(`{}`)}
	if err := publisher.Publish(context.Background(), message, taskmodule.PublishOptions{QueueID: "queue-archive", MaxRetries: 0}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case <-processor.failed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retry exhaustion status sync")
	}
	inspector := asynq.NewInspectorFromRedisClient(redisClient)
	deadline := time.Now().Add(5 * time.Second)
	for {
		archived, err := inspector.ListArchivedTasks("default")
		if err == nil && len(archived) == 1 && archived[0].ID == "queue-archive" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("archived tasks = %+v, error = %v", archived, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
