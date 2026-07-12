package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"backend-infrastructure-go/internal/config"
	"backend-infrastructure-go/internal/modules/audit"
	auditpostgres "backend-infrastructure-go/internal/modules/audit/postgres"
	"backend-infrastructure-go/internal/modules/iam"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/platform/database"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	"backend-infrastructure-go/internal/platform/httpserver"
	"backend-infrastructure-go/internal/platform/observability"
	queueplatform "backend-infrastructure-go/internal/platform/queue"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type APIRouterOptions struct {
	Logger            *slog.Logger
	Readiness         httpserver.ReadinessChecker
	Metrics           http.Handler
	MetricsMiddleware func(http.Handler) http.Handler
	RateLimit         *httpserver.RateLimitConfig
	RegisterIAM       func(chi.Router)
}

func NewAPIRouter(options APIRouterOptions) (http.Handler, error) {
	router, err := httpserver.NewRouter(httpserver.RouterOptions{Logger: options.Logger, Readiness: options.Readiness, Metrics: options.Metrics, RateLimit: options.RateLimit, Register: options.RegisterIAM})
	if err != nil {
		return nil, err
	}
	if options.MetricsMiddleware != nil {
		return options.MetricsMiddleware(router), nil
	}
	return router, nil
}

type apiRuntime struct {
	server *http.Server
	pool   *pgxpool.Pool
	redis  *redis.Client
}

func (runtime *apiRuntime) Run(context.Context) error {
	err := runtime.server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (runtime *apiRuntime) Shutdown(ctx context.Context) error {
	var result error
	if runtime.server != nil {
		result = errors.Join(result, runtime.server.Shutdown(ctx))
	}
	if runtime.redis != nil {
		result = errors.Join(result, runtime.redis.Close())
	}
	if runtime.pool != nil {
		runtime.pool.Close()
	}
	return result
}

func BuildAPI(ctx context.Context, cfg config.Config, logger *slog.Logger) (*apiRuntime, error) {
	if err := cfg.ValidateAPI(); err != nil {
		return nil, err
	}
	pool, err := database.Open(ctx, database.Config{URL: cfg.DatabaseURL, MinConns: cfg.DatabaseMinConns, MaxConns: cfg.DatabaseMaxConns, HealthCheckPeriod: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	queries := dbgen.New(pool)
	if err := BootstrapAdmin(ctx, queries, iam.NewPasswordHasher(iam.DefaultArgon2Params()), AdminBootstrap{Email: cfg.AdminEmail, Username: cfg.AdminUsername, Password: cfg.AdminPassword}); err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	iamRepo := iam.NewSQLCRepository(queries)
	jwt, err := iam.NewJWTManager([]byte(cfg.JWTSecret), cfg.JWTIssuer, cfg.AccessTokenTTL)
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	refresh := iam.NewRefreshStore(redisCacheAdapter{client: redisClient}, "iam:"+cfg.Environment, cfg.RefreshTokenTTL)
	iamService := iam.NewService(iamRepo, iamRepo, iam.NewPasswordHasher(iam.DefaultArgon2Params()), jwt, refresh)
	auditService := audit.NewService(auditpostgres.New(queries))
	metrics := observability.New(observability.Config{Namespace: "backend", KnownDependencies: []string{"postgres", "redis"}, KnownTaskTypes: []string{"system.test"}})
	auditRecorder := audit.NewBestEffortRecorder(auditService, logger, metrics)
	executionStore := taskmodule.NewPostgresExecutionStore(queries)
	queueClient := asynq.NewClientFromRedisClient(redisClient)
	submissions := taskmodule.NewSubmissionService(executionStore, queueplatform.NewAsynqPublisher(queueClient, cfg.AsynqQueue), newUUID)
	readiness := dependencyReadiness{pool: pool, redis: redisClient}
	limiter := httpserver.NewRedisRateLimiter(redisClient, "ratelimit:"+cfg.Environment, time.Now)
	handler, err := NewAPIRouter(APIRouterOptions{Logger: logger, Readiness: readiness, Metrics: metrics.Handler(), MetricsMiddleware: metrics.HTTPMiddleware, RateLimit: &httpserver.RateLimitConfig{Limiter: limiter, Limit: cfg.RateLimit, Window: cfg.RateLimitWindow, Critical: func(r *http.Request) bool { return stringsHasAuthPrefix(r.URL.Path) }}, RegisterIAM: func(router chi.Router) {
		audited := newAuditedIAM(iamService, auditRecorder)
		iam.RegisterRoutes(router, audited, jwt, iam.HTTPConfig{SecureCookies: cfg.SecureCookies, RefreshTTL: cfg.RefreshTokenTTL})
		RegisterPlatformRoutes(router, PlatformRoutes{IAM: audited, JWT: jwt, Audits: auditService, Tasks: submissions, Executions: executionStore, Recorder: auditRecorder})
	}})
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	return &apiRuntime{server: &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}, pool: pool, redis: redisClient}, nil
}

type redisCacheAdapter struct{ client *redis.Client }

func (a redisCacheAdapter) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return a.client.Set(ctx, key, value, ttl).Err()
}
func (a redisCacheAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}
func (a redisCacheAdapter) Delete(ctx context.Context, keys ...string) (int64, error) {
	return a.client.Del(ctx, keys...).Result()
}

type dependencyReadiness struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

func (r dependencyReadiness) Ready(ctx context.Context) error {
	if err := r.pool.Ping(ctx); err != nil {
		return err
	}
	return r.redis.Ping(ctx).Err()
}
func stringsHasAuthPrefix(path string) bool { return len(path) >= 12 && path[:12] == "/api/v1/auth" }
