package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"backend-infrastructure-go/internal/config"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/platform/database"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	queueplatform "backend-infrastructure-go/internal/platform/queue"
	schedulerplatform "backend-infrastructure-go/internal/platform/scheduler"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type workerRuntime struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	client *asynq.Client
	pool   *pgxpool.Pool
	redis  *redis.Client
}

func (r *workerRuntime) Run(context.Context) error { return r.server.Run(r.mux) }
func (r *workerRuntime) Shutdown(context.Context) error {
	r.server.Shutdown()
	var err error
	if r.client != nil {
		err = errors.Join(err, r.client.Close())
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
	queries := dbgen.New(pool)
	store := taskmodule.NewPostgresExecutionStore(queries)
	registry := taskmodule.NewRegistry()
	if err := registry.Register(taskmodule.SystemTestTaskType, taskmodule.SystemTestHandler{}); err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	processor := taskmodule.NewProcessor(store, registry, time.Now)
	handler := queueplatform.NewAsynqHandler(processor)
	failures := queueplatform.NewFailureHandler(processor)
	client := asynq.NewClientFromRedisClient(redisClient)
	publisher := queueplatform.NewAsynqPublisher(client, cfg.AsynqQueue)
	submissions := taskmodule.NewSubmissionService(store, publisher, newUUID)
	dispatcher := taskmodule.NewScheduledSubmissionHandler(submissions)
	server := asynq.NewServerFromRedisClient(redisClient, asynq.Config{Concurrency: cfg.AsynqConcurrency, Queues: map[string]int{cfg.AsynqQueue: 1}, ShutdownTimeout: cfg.ShutdownTimeout, ErrorHandler: asynq.ErrorHandlerFunc(failures.HandleError), Logger: asynqLogger{logger: logger}})
	mux := asynq.NewServeMux()
	mux.Handle(taskmodule.SystemTestTaskType, handler)
	mux.Handle(taskmodule.ScheduledDispatchTaskType, scheduledAsynqHandler{handler: dispatcher})
	return &workerRuntime{server: server, mux: mux, client: client, pool: pool, redis: redisClient}, nil
}

type schedulerRuntime struct {
	scheduler *asynq.Scheduler
	refresher *schedulerplatform.Refresher
	interval  time.Duration
	pool      *pgxpool.Pool
	redis     *redis.Client
}

func (r *schedulerRuntime) Run(ctx context.Context) error {
	if err := r.refresher.Refresh(ctx); err != nil {
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
			if err := r.refresher.Refresh(ctx); err != nil {
				return err
			}
		}
	}
}
func (r *schedulerRuntime) Shutdown(context.Context) error {
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
	pool, redisClient, err := openTaskDependencies(ctx, cfg)
	if err != nil {
		return nil, err
	}
	queries := dbgen.New(pool)
	source := taskmodule.NewScheduleService(taskmodule.NewPostgresScheduleStore(queries))
	scheduler := asynq.NewSchedulerFromRedisClient(redisClient, &asynq.SchedulerOpts{Logger: asynqLogger{logger: logger}})
	registrar := schedulerplatform.NewAsynqRegistrar(scheduler, func(schedule taskmodule.Schedule) (*asynq.Task, []asynq.Option, error) {
		return buildScheduledTask(schedule, cfg.AsynqQueue)
	})
	return &schedulerRuntime{scheduler: scheduler, refresher: schedulerplatform.NewRefresher(source, registrar), interval: cfg.SchedulerRefreshInterval, pool: pool, redis: redisClient}, nil
}

func openTaskDependencies(ctx context.Context, cfg config.Config) (*pgxpool.Pool, *redis.Client, error) {
	pool, err := database.Open(ctx, database.Config{URL: cfg.DatabaseURL, MinConns: cfg.DatabaseMinConns, MaxConns: cfg.DatabaseMaxConns, HealthCheckPeriod: 30 * time.Second})
	if err != nil {
		return nil, nil, err
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
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
