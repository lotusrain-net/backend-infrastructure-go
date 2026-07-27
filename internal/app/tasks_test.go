package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"backend-infrastructure-go/internal/config"
	"backend-infrastructure-go/internal/modules/audit"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	platformlogging "backend-infrastructure-go/internal/platform/logging"
	"backend-infrastructure-go/internal/platform/observability"
	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type outboxDispatcherStub struct {
	calls atomic.Int32
	err   error
}

func (stub *outboxDispatcherStub) DispatchPending(context.Context, int) error {
	stub.calls.Add(1)
	return stub.err
}

func TestRunOutboxDispatcherDrainsImmediatelyAndRetriesFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output bytes.Buffer
	logger := platformlogging.New(&output, slog.LevelInfo, "test")
	dispatcher := &outboxDispatcherStub{err: errors.New("redis unavailable")}
	done := make(chan struct{})

	go func() {
		runOutboxDispatcher(ctx, logger, dispatcher, 5*time.Millisecond, 10)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for dispatcher.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if calls := dispatcher.calls.Load(); calls < 2 {
		t.Fatalf("dispatcher calls = %d, want startup drain and retry", calls)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher loop did not stop after cancellation")
	}
	if !strings.Contains(output.String(), "task outbox dispatch failed") {
		t.Fatalf("logs = %q", output.String())
	}
}

func TestBuildScheduledTaskCarriesDefinitionExecutionPolicy(t *testing.T) {
	schedule := taskmodule.Schedule{ID: "schedule-1", DefinitionID: "definition-1", TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{"processed_rows":4}`), MaxRetries: 5, Timeout: 2 * time.Minute, Enabled: true}
	queued, options, err := buildScheduledTask(schedule, "critical")
	if err != nil {
		t.Fatal(err)
	}
	if queued.Type() != taskmodule.ScheduledDispatchTaskType || len(options) == 0 {
		t.Fatalf("task=%s options=%d", queued.Type(), len(options))
	}
	var payload taskmodule.ScheduledSubmission
	if err := json.Unmarshal(queued.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.DefinitionID != schedule.DefinitionID || payload.TaskType != schedule.TaskType || payload.MaxRetries != 5 || payload.Timeout != 2*time.Minute {
		t.Fatalf("payload=%+v", payload)
	}
}

type schedulerRefresherStub struct{ err error }

func (stub schedulerRefresherStub) Refresh(context.Context) error { return stub.err }

func TestSchedulerRefreshRecordsAuditOutcome(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		result audit.Result
	}{
		{name: "success", result: audit.ResultSuccess},
		{name: "failure", err: context.DeadlineExceeded, result: audit.ResultFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &preserveStub{}
			runtime := &schedulerRuntime{refresher: schedulerRefresherStub{err: test.err}, audit: recorder}
			err := runtime.refreshSchedules(context.Background())
			if !errors.Is(err, test.err) {
				t.Fatalf("refreshSchedules() error = %v", err)
			}
			if recorder.event.Action != "scheduling.refresh" || recorder.event.Result != test.result || recorder.event.ResourceType != "task_schedule" {
				t.Fatalf("audit event = %+v", recorder.event)
			}
		})
	}
}

func TestWorkerMetricsServerExposesTerminalTaskMetrics(t *testing.T) {
	metrics := observability.New(observability.Config{Namespace: "backend", KnownTaskTypes: []string{taskmodule.SystemTestTaskType}})
	metrics.ObserveTask(taskmodule.SystemTestTaskType, string(taskmodule.StatusSucceeded), time.Second)
	server := newWorkerMetricsServer(config.Config{WorkerMetricsAddr: ":9090"}, metrics.Handler())
	if server.Addr != ":9090" {
		t.Fatalf("metrics server address = %q", server.Addr)
	}
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `backend_task_executions_total{status="succeeded",task_type="system.test"} 1`) {
		t.Fatalf("metrics response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRuntimeTaskCatalogRegistersEveryWorkerHandler(t *testing.T) {
	registry, err := newTaskRegistry(runtimeTaskCatalog)
	if err != nil {
		t.Fatalf("newTaskRegistry() error = %v", err)
	}
	for _, taskType := range RuntimeTaskTypes() {
		if _, err := registry.Resolve(taskType); err != nil {
			t.Fatalf("Resolve(%q) error = %v", taskType, err)
		}
	}
}

func TestSchedulerOwnsItsAsynqRedisConnection(t *testing.T) {
	redisServer := miniredis.RunT(t)
	var output bytes.Buffer
	logger := platformlogging.New(&output, slog.LevelInfo, "test")
	scheduler, err := newAsynqScheduler(config.Config{RedisAddr: redisServer.Addr()}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Start(); err != nil {
		t.Fatal(err)
	}
	scheduler.Shutdown()
	if strings.Contains(output.String(), "redis connection is shared") {
		t.Fatalf("scheduler shutdown logs shared-connection error: %s", output.String())
	}
}

func TestWorkerShutdownDoesNotCloseSharedAsynqClient(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	runtime := &workerRuntime{
		server: asynq.NewServerFromRedisClient(redisClient, asynq.Config{}),
		redis:  redisClient,
	}

	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
