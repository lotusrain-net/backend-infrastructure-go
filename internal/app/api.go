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
	"backend-infrastructure-go/internal/modules/audit/requestmeta"
	"backend-infrastructure-go/internal/modules/iam"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/platform/database"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	"backend-infrastructure-go/internal/platform/database/iamstore"
	"backend-infrastructure-go/internal/platform/database/taskstore"
	"backend-infrastructure-go/internal/platform/httpserver"
	"backend-infrastructure-go/internal/platform/observability"
	queueplatform "backend-infrastructure-go/internal/platform/queue"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type APIRouterOptions struct {
	Logger                    *slog.Logger
	Readiness                 httpserver.ReadinessChecker
	Metrics                   http.Handler
	MetricsMiddleware         func(http.Handler) http.Handler
	RequestMetadataMiddleware func(http.Handler) http.Handler
	RateLimit                 *httpserver.RateLimitConfig
	CORS                      httpserver.CORSConfig
	RegisterIAM               func(chi.Router)
}

func NewAPIRouter(options APIRouterOptions) (http.Handler, error) {
	router, err := httpserver.NewRouter(httpserver.RouterOptions{Logger: options.Logger, Readiness: options.Readiness, Metrics: options.Metrics, CORS: options.CORS, RateLimit: options.RateLimit, Register: options.RegisterIAM})
	if err != nil {
		return nil, err
	}
	var handler http.Handler = router
	if options.RequestMetadataMiddleware != nil {
		handler = options.RequestMetadataMiddleware(handler)
	}
	if options.MetricsMiddleware != nil {
		handler = options.MetricsMiddleware(handler)
	}
	return handler, nil
}

var knownAPIRoutes = []string{
	"/health/live", "/health/ready", "/metrics",
	"/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout",
	"/api/v1/users/me", "/api/v1/users", "/api/v1/users/{userID}/active",
	"/api/v1/roles", "/api/v1/permissions", "/api/v1/users/{userID}/roles/{roleID}",
	"/api/v1/roles/{roleID}/permissions/{permissionID}", "/api/v1/audit-logs",
	"/api/v1/task-executions", "/api/v1/task-executions/{executionID}",
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
	iamRepo := iamstore.New(queries)
	jwt, err := iam.NewJWTManager([]byte(cfg.JWTSecret), cfg.JWTIssuer, cfg.AccessTokenTTL)
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	refresh := iam.NewRefreshStore(redisCacheAdapter{client: redisClient}, "iam:"+cfg.Environment, cfg.RefreshTokenTTL)
	iamService := iam.NewService(iamRepo, iamRepo, iam.NewPasswordHasher(iam.DefaultArgon2Params()), jwt, refresh)
	auditService := audit.NewService(auditpostgres.New(queries))
	metrics := observability.New(observability.Config{Namespace: "backend", KnownRoutes: knownAPIRoutes, KnownDependencies: []string{"postgres", "redis"}, KnownTaskTypes: []string{"system.test"}})
	auditRecorder := audit.NewBestEffortRecorder(auditService, logger, metrics)
	executionStore := taskstore.NewPostgresExecutionStore(queries)
	queueClient := asynq.NewClientFromRedisClient(redisClient)
	submissions := taskmodule.NewSubmissionService(executionStore, queueplatform.NewAsynqPublisher(queueClient, cfg.AsynqQueue), newUUID)
	readiness := dependencyReadiness{
		postgresPing: pool.Ping,
		redisPing:    func(ctx context.Context) error { return redisClient.Ping(ctx).Err() },
		observer:     metrics,
		now:          time.Now,
	}
	limiter := httpserver.NewRedisRateLimiter(redisClient, "ratelimit:"+cfg.Environment, time.Now)
	handler, err := NewAPIRouter(APIRouterOptions{Logger: logger, Readiness: readiness, Metrics: metrics.Handler(), MetricsMiddleware: metrics.HTTPMiddleware, RequestMetadataMiddleware: requestmeta.Middleware(requestmeta.Config{TrustedProxies: cfg.TrustedProxyCIDRs}), CORS: httpserver.CORSConfig{AllowedOrigins: cfg.CORSAllowedOrigins, AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}, AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-ID"}, ExposedHeaders: []string{"X-Request-ID"}, AllowCredentials: cfg.CORSAllowCredentials, MaxAge: 10 * time.Minute}, RateLimit: &httpserver.RateLimitConfig{Limiter: limiter, Limit: cfg.RateLimit, Window: cfg.RateLimitWindow, Key: auditClientKey, Critical: func(r *http.Request) bool { return stringsHasAuthPrefix(r.URL.Path) }}, RegisterIAM: func(router chi.Router) {
		audited := newAuditedIAM(iamService, auditRecorder)
		iam.RegisterRoutes(router, audited, jwt, iam.HTTPConfig{SecureCookies: cfg.SecureCookies, RefreshTTL: cfg.RefreshTokenTTL})
		RegisterPlatformRoutes(router, PlatformRoutes{IAM: audited, JWT: jwt, Audits: auditService, Tasks: submissions, Executions: executionStore, Recorder: auditRecorder, TaskObserver: metrics})
	}})
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	return &apiRuntime{server: newHTTPServer(cfg, handler), pool: pool, redis: redisClient}, nil
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

type redisCacheAdapter struct{ client *redis.Client }

func (a redisCacheAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}
func (a redisCacheAdapter) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

type dependencyReadiness struct {
	postgresPing func(context.Context) error
	redisPing    func(context.Context) error
	observer     dependencyObserver
	now          func() time.Time
}

type dependencyObserver interface {
	ObserveDependency(dependency, outcome string, duration time.Duration)
}

func (r dependencyReadiness) Ready(ctx context.Context) error {
	if err := r.check(ctx, "postgres", r.postgresPing); err != nil {
		return err
	}
	return r.check(ctx, "redis", r.redisPing)
}

func (r dependencyReadiness) check(ctx context.Context, name string, ping func(context.Context) error) error {
	now := r.now
	if now == nil {
		now = time.Now
	}
	started := now()
	err := ping(ctx)
	if r.observer != nil {
		outcome := "success"
		if err != nil {
			outcome = "failure"
			if errors.Is(err, context.DeadlineExceeded) {
				outcome = "timeout"
			}
		}
		r.observer.ObserveDependency(name, outcome, now().Sub(started))
	}
	return err
}
func stringsHasAuthPrefix(path string) bool { return len(path) >= 12 && path[:12] == "/api/v1/auth" }

func auditClientKey(request *http.Request) string {
	metadata := audit.RequestMetadataFromContext(request.Context())
	if metadata.IPAddress != nil {
		return metadata.IPAddress.String()
	}
	return request.RemoteAddr
}
