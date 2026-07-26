package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"backend-infrastructure-go/internal/config"
	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/httpserver"
	"backend-infrastructure-go/internal/platform/httpserver/requestmeta"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type dependencyObserverStub struct{ calls []string }

func (stub *dependencyObserverStub) ObserveDependency(dependency, outcome string, _ time.Duration) {
	stub.calls = append(stub.calls, dependency+":"+outcome)
}

type keyCapturingLimiter struct{ key string }

func (limiter *keyCapturingLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (httpserver.RateLimitDecision, error) {
	limiter.key = key
	return httpserver.RateLimitDecision{Allowed: true, Remaining: limit - 1}, nil
}

type readyFunc func(context.Context) error

func (fn readyFunc) Ready(ctx context.Context) error { return fn(ctx) }

func TestNewAPIRouterMountsHealthMetricsAndIAMRegistrar(t *testing.T) {
	registered := false
	router, err := NewAPIRouter(APIRouterOptions{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness: readyFunc(func(context.Context) error { return nil }),
		Metrics:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }),
		RegisterIAM: func(r chi.Router) {
			registered = true
			r.Get("/api/v1/test-iam", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("IAM registrar was not invoked")
	}
	for path, want := range map[string]int{"/health/ready": http.StatusOK, "/metrics": http.StatusAccepted, "/api/v1/test-iam": http.StatusNoContent} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s status=%d want=%d", path, rec.Code, want)
		}
	}
}

func TestNewHTTPServerAppliesConnectionLimits(t *testing.T) {
	server := newHTTPServer(config.Config{HTTPAddr: ":9090"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if server.Addr != ":9090" || server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts are incomplete: %+v", server)
	}
	if server.MaxHeaderBytes < 1024 || server.ReadTimeout > 30*time.Second || server.WriteTimeout > 30*time.Second {
		t.Fatalf("server limits are not bounded: %+v", server)
	}
}

func TestNewAPIRouterAppliesConfiguredCORS(t *testing.T) {
	router, err := NewAPIRouter(APIRouterOptions{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness: readyFunc(func(context.Context) error { return nil }),
		Metrics:   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		CORS: httpserver.CORSConfig{
			AllowedOrigins:   []string{"https://app.example.com"},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost},
			AllowedHeaders:   []string{"Authorization", "Content-Type"},
			AllowCredentials: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/test", nil)
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestDependencyReadinessObservesPostgresAndRedisChecks(t *testing.T) {
	observer := &dependencyObserverStub{}
	readiness := dependencyReadiness{
		postgresPing: func(context.Context) error { return nil },
		redisPing:    func(context.Context) error { return nil },
		observer:     observer,
		now:          time.Now,
	}
	if err := readiness.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(observer.calls, ","); got != "postgres:success,redis:success" {
		t.Fatalf("dependency observations = %q", got)
	}
}

func TestDependencyReadinessObservesFailureAndSkipsDependentCheck(t *testing.T) {
	observer := &dependencyObserverStub{}
	redisCalled := false
	readiness := dependencyReadiness{
		postgresPing: func(context.Context) error { return errors.New("postgres unavailable") },
		redisPing: func(context.Context) error {
			redisCalled = true
			return nil
		},
		observer: observer,
		now:      time.Now,
	}
	if err := readiness.Ready(context.Background()); err == nil {
		t.Fatal("Ready() error = nil")
	}
	if redisCalled || strings.Join(observer.calls, ",") != "postgres:failure" {
		t.Fatalf("redisCalled=%v observations=%v", redisCalled, observer.calls)
	}
}

func TestKnownAPIRoutesCoverHealthIAMAdministrationAuditAndTasks(t *testing.T) {
	want := []string{
		"/health/live", "/health/ready", "/metrics", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout",
		"/api/v1/users/me", "/api/v1/users", "/api/v1/users/{userID}/active", "/api/v1/roles", "/api/v1/permissions",
		"/api/v1/users/{userID}/roles/{roleID}", "/api/v1/roles/{roleID}/permissions/{permissionID}",
		"/api/v1/audit-logs", "/api/v1/task-executions", "/api/v1/task-executions/{executionID}", "/api/v1/task-types",
	}
	for _, route := range want {
		if !slices.Contains(knownAPIRoutes, route) {
			t.Errorf("knownAPIRoutes missing %q", route)
		}
	}
}

func TestAPIRateLimitUsesClientIPExtractedBehindTrustedProxy(t *testing.T) {
	limiter := &keyCapturingLimiter{}
	router, err := NewAPIRouter(APIRouterOptions{
		Logger:                    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness:                 readyFunc(func(context.Context) error { return nil }),
		Metrics:                   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		RequestMetadataMiddleware: requestmeta.Middleware(requestmeta.Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}),
		RateLimit:                 &httpserver.RateLimitConfig{Limiter: limiter, Limit: 10, Window: time.Minute, Key: auditClientKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	request.RemoteAddr = "10.0.0.1:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	router.ServeHTTP(httptest.NewRecorder(), request)
	if limiter.key != "203.0.113.9" {
		t.Fatalf("rate-limit key = %q", limiter.key)
	}
}

func TestAPICriticalRateLimitRoutesIncludeAuthAuditAndTaskControlPlane(t *testing.T) {
	for _, path := range []string{
		"/api/v1/auth/login",
		"/api/v1/audit-logs",
		"/api/v1/task-executions",
	} {
		if !isCriticalRateLimitPath(path) {
			t.Errorf("critical rate-limit route %q is fail-open", path)
		}
	}
	if isCriticalRateLimitPath("/health/live") {
		t.Fatal("health endpoint must remain fail-open when the distributed limiter is unavailable")
	}
}

func TestRedisCacheAdapterMapsMissingKeysWithoutHidingDependencyFailures(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	adapter := redisCacheAdapter{client: client}
	if _, err := adapter.Get(context.Background(), "missing"); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("Get() error = %v, want IAM not found", err)
	}
	server.Close()
	if _, err := adapter.Get(context.Background(), "dependency-failure"); err == nil || errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("Get() error = %v, want dependency failure", err)
	}
}
