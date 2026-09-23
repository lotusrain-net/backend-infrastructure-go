package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/config"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/audit"
	taskmodule "github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/task"
	cacheplatform "github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/cache"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/auditstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/taskstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/observability"
	queueplatform "github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/queue"
	schedulerplatform "github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/scheduler"
	database "github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
	"github.com/redis/go-redis/v9"
)

type workerRuntime struct {
	server         *asynq.Server
	mux            *asynq.ServeMux
	pool           *pgxpool.Pool
	redis          *redis.Client
	metrics        *http.Server
	outbox         pendingDispatcher
	outboxInterval time.Duration
	logger         *slog.Logger
	resources      *Stack
}

type pendingDispatcher interface {
	DispatchPending(context.Context, int) error
}

const (
	outboxDispatchBatchSize = 100
	outboxDispatchInterval  = 5 * time.Second
	outboxDispatchTimeout   = 10 * time.Second
)

func (r *workerRuntime) Run(ctx context.Context) error {
	if r.outbox != nil {
		interval := r.outboxInterval
		if interval <= 0 {
			interval = outboxDispatchInterval
		}
		go runOutboxDispatcher(ctx, r.logger, r.outbox, interval, outboxDispatchBatchSize)
	}
	results := make(chan error, 2)
	go func() { results <- r.server.Run(r.mux) }()
	go func() {
		err := r.metrics.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		results <- err
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-results:
		return err
	}
}

func runOutboxDispatcher(ctx context.Context, logger *slog.Logger, dispatcher pendingDispatcher, interval time.Duration, limit int) {
	if logger == nil {
		logger = slog.Default()
	}
	dispatch := func() {
		dispatchCtx, cancel := context.WithTimeout(ctx, outboxDispatchTimeout)
		defer cancel()
		if err := dispatcher.DispatchPending(dispatchCtx, limit); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "task outbox dispatch failed", "error", err)
		}
	}
	dispatch()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatch()
		}
	}
}
func (r *workerRuntime) Shutdown(ctx context.Context) error {
	if r.resources != nil {
		return r.resources.Close(ctx)
	}
	r.server.Shutdown()
	var err error
	if r.metrics != nil {
		err = errors.Join(err, r.metrics.Shutdown(ctx))
	}
	if r.redis != nil {
		err = errors.Join(err, r.redis.Close())
	}
	if r.pool != nil {
		r.pool.Close()
	}
	return err
}

func BuildWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) (*workerRuntime, error) {
	pool, redisClient, err := openTaskDependencies(ctx, cfg)
	if err != nil {
		return nil, err
	}
	resources := NewStack()
	resources.Add(resourceCloser(func(context.Context) error {
		pool.Close()
		return nil
	}))
	resources.Add(resourceCloser(func(context.Context) error { return redisClient.Close() }))
	queries := dbgen.New(pool)
	store := taskstore.NewPostgresExecutionStore(queries)
	registry, err := newTaskRegistry(runtimeTaskCatalog)
	if err != nil {
		return nil, closeBuildResources(ctx, resources, err)
	}
	metrics := observability.New(observability.Config{Namespace: "backend", KnownTaskTypes: RuntimeTaskTypes()})
	processor := taskmodule.NewProcessor(store, registry, time.Now, metrics)
	handler := queueplatform.NewAsynqHandler(processor)
	failures := queueplatform.NewFailureHandler(processor, logger)
	client := asynq.NewClientFromRedisClient(redisClient)
	publisher := queueplatform.NewAsynqPublisher(client, cfg.AsynqQueue)
	outbox := queueplatform.NewOutboxDispatcher(store, publisher)
	submissions := taskmodule.NewSubmissionService(store, publisher, newUUID)
	dispatcher := taskmodule.NewScheduledSubmissionHandler(submissions)
	server := asynq.NewServerFromRedisClient(redisClient, asynq.Config{Concurrency: cfg.AsynqConcurrency, Queues: map[string]int{cfg.AsynqQueue: 1}, ShutdownTimeout: cfg.ShutdownTimeout, ErrorHandler: asynq.ErrorHandlerFunc(failures.HandleError), Logger: asynqLogger{logger: logger}})
	metricsServer := newWorkerMetricsServer(cfg, metrics.Handler())
	resources.Add(resourceCloser(metricsServer.Shutdown))
	resources.Add(resourceCloser(func(context.Context) error {
		server.Shutdown()
		return nil
	}))
	mux := asynq.NewServeMux()
	for _, taskType := range RuntimeTaskTypes() {
		mux.Handle(taskType, handler)
	}
	mux.Handle(taskmodule.ScheduledDispatchTaskType, scheduledAsynqHandler{handler: dispatcher})
	return &workerRuntime{
		server:         server,
		mux:            mux,
		pool:           pool,
		redis:          redisClient,
		metrics:        metricsServer,
		outbox:         outbox,
		outboxInterval: outboxDispatchInterval,
		logger:         logger,
		resources:      resources,
	}, nil
}

func newWorkerMetricsServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.WorkerMetricsAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}

type schedulerRuntime struct {
	scheduler *asynq.Scheduler
	refresher scheduleRefresher
	audit     auditPreserver
	interval  time.Duration
	pool      *pgxpool.Pool
	redis     *redis.Client
	resources *Stack
}

type scheduleRefresher interface {
	Refresh(context.Context) error
}

func (r *schedulerRuntime) Run(ctx context.Context) error {
	if err := r.refreshSchedules(ctx); err != nil {
		return err
	}
	if err := r.scheduler.Start(); err != nil {
		return err
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.refreshSchedules(ctx); err != nil {
				return err
			}
		}
	}
}

func (r *schedulerRuntime) refreshSchedules(ctx context.Context) error {
	err := r.refresher.Refresh(ctx)
	if r.audit == nil {
		return err
	}
	result := audit.ResultSuccess
	if err != nil {
		result = audit.ResultFailure
	}
	return r.audit.Preserve(ctx, audit.NewEvent{
		RequestID:    "scheduler-" + newUUID(),
		Action:       "scheduling.refresh",
		Result:       result,
		ResourceType: "task_schedule",
	}, err)
}
func (r *schedulerRuntime) Shutdown(ctx context.Context) error {
	if r.resources != nil {
		return r.resources.Close(ctx)
	}
	r.scheduler.Shutdown()
	var err error
	if r.redis != nil {
		err = r.redis.Close()
	}
	if r.pool != nil {
		r.pool.Close()
	}
	return err
}

func BuildScheduler(ctx context.Context, cfg config.Config, logger *slog.Logger) (*schedulerRuntime, error) {
	pool, err := database.Open(ctx, database.Config{URL: cfg.DatabaseURL, MinConns: cfg.DatabaseMinConns, MaxConns: cfg.DatabaseMaxConns, HealthCheckPeriod: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	resources := NewStack()
	resources.Add(resourceCloser(func(context.Context) error {
		pool.Close()
		return nil
	}))
	queries := dbgen.New(pool)
	source := taskmodule.NewScheduleService(taskstore.NewPostgresScheduleStore(queries))
	scheduler, err := newAsynqScheduler(cfg, logger)
	if err != nil {
		return nil, closeBuildResources(ctx, resources, err)
	}
	resources.Add(resourceCloser(func(context.Context) error {
		scheduler.Shutdown()
		return nil
	}))
	registrar := schedulerplatform.NewAsynqRegistrar(scheduler, func(schedule taskmodule.Schedule) (*asynq.Task, []asynq.Option, error) {
		return buildScheduledTask(schedule, cfg.AsynqQueue)
	})
	auditRecorder := audit.NewBestEffortRecorder(audit.NewService(auditstore.New(queries)), logger, nil)
	return &schedulerRuntime{scheduler: scheduler, refresher: schedulerplatform.NewRefresher(source, registrar), audit: auditRecorder, interval: cfg.SchedulerRefreshInterval, pool: pool, resources: resources}, nil
}

func newAsynqScheduler(cfg config.Config, logger *slog.Logger) (*asynq.Scheduler, error) {
	redisTLS, err := cacheplatform.NewTLSConfig(cfg.RedisTLS, cfg.RedisTLSServerName, cfg.RedisTLSCAFile)
	if err != nil {
		return nil, err
	}
	return asynq.NewScheduler(asynq.RedisClientOpt{
		Addr:      cfg.RedisAddr,
		Password:  cfg.RedisPassword,
		DB:        cfg.RedisDB,
		TLSConfig: redisTLS,
	}, &asynq.SchedulerOpts{Logger: asynqLogger{logger: logger}}), nil
}

func openTaskDependencies(ctx context.Context, cfg config.Config) (*pgxpool.Pool, *redis.Client, error) {
	pool, err := database.Open(ctx, database.Config{URL: cfg.DatabaseURL, MinConns: cfg.DatabaseMinConns, MaxConns: cfg.DatabaseMaxConns, HealthCheckPeriod: 30 * time.Second})
	if err != nil {
		return nil, nil, err
	}
	redisTLS, err := cacheplatform.NewTLSConfig(cfg.RedisTLS, cfg.RedisTLSServerName, cfg.RedisTLSCAFile)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, TLSConfig: redisTLS})
	if err := client.Ping(ctx).Err(); err != nil {
		pool.Close()
		_ = client.Close()
		return nil, nil, err
	}
	return pool, client, nil
}

type asynqLogger struct{ logger *slog.Logger }

func (l asynqLogger) Debug(args ...interface{}) { l.logger.Debug("asynq", "args", args) }
func (l asynqLogger) Info(args ...interface{})  { l.logger.Info("asynq", "args", args) }
func (l asynqLogger) Warn(args ...interface{})  { l.logger.Warn("asynq", "args", args) }
func (l asynqLogger) Error(args ...interface{}) { l.logger.Error("asynq", "args", args) }
func (l asynqLogger) Fatal(args ...interface{}) { l.logger.Error("asynq fatal", "args", args) }

type scheduledAsynqHandler struct{ handler taskmodule.Handler }

func (h scheduledAsynqHandler) ProcessTask(ctx context.Context, queued *asynq.Task) error {
	_, err := h.handler.Handle(ctx, queued.Payload())
	return err
}
func buildScheduledTask(schedule taskmodule.Schedule, queue string) (*asynq.Task, []asynq.Option, error) {
	payload, err := json.Marshal(taskmodule.ScheduledSubmission{DefinitionID: schedule.DefinitionID, TaskType: schedule.TaskType, Payload: schedule.Payload, MaxRetries: schedule.MaxRetries, Timeout: schedule.Timeout})
	if err != nil {
		return nil, nil, err
	}
	return asynq.NewTask(taskmodule.ScheduledDispatchTaskType, payload), []asynq.Option{asynq.Queue(queue), asynq.MaxRetry(3), asynq.Timeout(30 * time.Second)}, nil
}
func newUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
