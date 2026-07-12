package httpserver_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend-infrastructure-go/internal/platform/httpserver"
)

func TestRouterExposesLiveness(t *testing.T) {
	router := httpserver.NewRouter(httpserver.RouterOptions{Logger: discardLogger()})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRouterReadinessChecksDependencies(t *testing.T) {
	router := httpserver.NewRouter(httpserver.RouterOptions{
		Logger:    discardLogger(),
		Readiness: readinessFunc(func(context.Context) error { return errors.New("postgres unavailable") }),
	})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":503`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRouterDelegatesMetricsEndpoint(t *testing.T) {
	metrics := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("http_requests_total 1\n"))
	})
	router := httpserver.NewRouter(httpserver.RouterOptions{Logger: discardLogger(), Metrics: metrics})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "http_requests_total 1\n" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error { return fn(ctx) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(ioDiscard{}, nil))
}

type ioDiscard struct{}

func (ioDiscard) Write(value []byte) (int, error) { return len(value), nil }
