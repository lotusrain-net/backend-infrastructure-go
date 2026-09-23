package httpkit_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
)

func TestNewRouterRequiresReadiness(t *testing.T) {
	_, err := httpkit.NewRouter(httpkit.RouterOptions{Logger: discardLogger(), Metrics: metricsHandler()})
	if err == nil {
		t.Fatal("missing readiness checker must fail construction")
	}
}

func TestNewRouterRequiresMetricsHandler(t *testing.T) {
	_, err := httpkit.NewRouter(httpkit.RouterOptions{Logger: discardLogger(), Readiness: readyChecker()})
	if err == nil {
		t.Fatal("missing metrics handler must fail construction")
	}
}

func TestNewRouterRejectsInvalidCORS(t *testing.T) {
	_, err := httpkit.NewRouter(httpkit.RouterOptions{
		Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler(),
		CORS: httpkit.CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true},
	})
	if err == nil {
		t.Fatal("invalid CORS must fail construction")
	}
}

func TestNewRouterRejectsEnabledRateLimitWithoutBackend(t *testing.T) {
	_, err := httpkit.NewRouter(httpkit.RouterOptions{
		Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler(),
		RateLimit: &httpkit.RateLimitConfig{Limit: 10, Window: time.Minute},
	})
	if err == nil {
		t.Fatal("enabled rate limiting without a backend must fail construction")
	}
}

func TestRouterExposesLiveness(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRouterReadinessChecksDependencies(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{
		Logger: discardLogger(), Metrics: metricsHandler(),
		Readiness: readinessFunc(func(context.Context) error { return errors.New("postgres unavailable") }),
	})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":503`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRouterDelegatesMetricsEndpoint(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "http_requests_total 1\n" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRegistrarRoutesInheritMiddleware(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{
		Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler(),
		Register: func(router chi.Router) {
			router.Get("/api/v1/example", func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNoContent)
			})
		},
	})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/example", nil))

	if recorder.Code != http.StatusNoContent || recorder.Header().Get(httpkit.RequestIDHeader) == "" {
		t.Fatalf("status=%d request-id=%q", recorder.Code, recorder.Header().Get(httpkit.RequestIDHeader))
	}
}

func TestRouterReturnsStableNotFoundEnvelope(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if recorder.Code != http.StatusNotFound || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"code":404`) || !strings.Contains(body, `"msg":"resource not found"`) || !strings.Contains(body, `"data":null`) {
		t.Fatalf("body=%q", body)
	}
}

func TestRouterReturnsStableMethodNotAllowedEnvelope(t *testing.T) {
	router := mustRouter(t, httpkit.RouterOptions{Logger: discardLogger(), Readiness: readyChecker(), Metrics: metricsHandler()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/health/live", nil))

	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"code":405`) || !strings.Contains(body, `"msg":"method not allowed"`) || !strings.Contains(body, `"data":null`) {
		t.Fatalf("body=%q", body)
	}
}

func mustRouter(t *testing.T, options httpkit.RouterOptions) chi.Router {
	t.Helper()
	router, err := httpkit.NewRouter(options)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error { return fn(ctx) }

func readyChecker() readinessFunc {
	return func(context.Context) error { return nil }
}

func metricsHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("http_requests_total 1\n"))
	})
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(ioDiscard{}, nil))
}

type ioDiscard struct{}

func (ioDiscard) Write(value []byte) (int, error) { return len(value), nil }
