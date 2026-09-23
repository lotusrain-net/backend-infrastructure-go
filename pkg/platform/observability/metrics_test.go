package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/observability"
)

func TestMetricsExposeHTTPDependencyAuditAndTaskMeasurements(t *testing.T) {
	metrics := observability.New(observability.Config{
		Namespace:   "backend",
		KnownRoutes: []string{"/users/{id}"}, KnownDependencies: []string{"postgres"}, KnownTaskTypes: []string{"system.test"},
	})
	metrics.ObserveHTTP("GET", "/users/42", http.StatusOK, 25*time.Millisecond)
	metrics.ObserveDependency("postgres", "success", 10*time.Millisecond)
	metrics.ObserveAudit("auth.login", "success")
	metrics.AuditWriteFailed("auth.login")
	metrics.ObserveTask("system.test", "succeeded", time.Second)

	body := scrape(t, metrics)
	for _, want := range []string{
		`backend_http_requests_total{method="GET",route="/users/{id}",status_class="2xx"} 1`,
		`backend_dependency_operations_total{dependency="postgres",outcome="success"} 1`,
		`backend_audit_records_total{category="auth",result="success"} 1`,
		`backend_audit_write_failures_total{category="auth"} 1`,
		`backend_task_executions_total{status="succeeded",task_type="system.test"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q\n%s", want, body)
		}
	}
}

func TestMetricsCollapseUnknownLabelsToBoundCardinality(t *testing.T) {
	metrics := observability.New(observability.Config{Namespace: "backend"})
	metrics.ObserveHTTP("BREW", "/customers/unique-person-123", 799, time.Millisecond)
	metrics.ObserveDependency("tenant-database-123", "unexpected", time.Millisecond)
	metrics.ObserveTask("tenant.task.123", "invented", time.Millisecond)

	body := scrape(t, metrics)
	for _, want := range []string{
		`method="OTHER",route="/unmatched",status_class="other"`,
		`dependency="other",outcome="other"`,
		`status="other",task_type="other"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q\n%s", want, body)
		}
	}
}

func TestNormalizePathReplacesDynamicIdentifiers(t *testing.T) {
	for input, want := range map[string]string{
		"/users/42": "/users/{id}",
		"/jobs/8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66": "/jobs/{id}",
		"/users/{id}":                    "/users/{id}",
		"/users/{userID}/roles/{roleID}": "/users/{id}/roles/{id}",
		"":                               "/unmatched",
	} {
		if got := observability.NormalizePath(input); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHTTPMiddlewareUsesRoutePatternInsteadOfRawIdentifier(t *testing.T) {
	metrics := observability.New(observability.Config{Namespace: "backend", KnownRoutes: []string{"/users/{id}"}})
	router := chi.NewRouter()
	router.Use(metrics.HTTPMiddleware)
	router.Get("/users/{id}", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusAccepted) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/users/customer-unique-value", nil))

	body := scrape(t, metrics)
	if !strings.Contains(body, `route="/users/{id}",status_class="2xx"`) || strings.Contains(body, "customer-unique-value") {
		t.Fatalf("metrics contain an unbounded raw path\n%s", body)
	}
}

func TestHTTPMiddlewareOutsideRouterUsesRegisteredRawPath(t *testing.T) {
	metrics := observability.New(observability.Config{Namespace: "backend", KnownRoutes: []string{"/health/ready"}})
	router := chi.NewRouter()
	router.Get("/health/ready", func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) })
	handler := metrics.HTTPMiddleware(router)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	body := scrape(t, metrics)
	if !strings.Contains(body, `route="/health/ready",status_class="2xx"`) || strings.Contains(body, `route="/unmatched",status_class="2xx"`) {
		t.Fatalf("registered route was not observed\n%s", body)
	}
}

func scrape(t *testing.T, metrics *observability.Metrics) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", recorder.Code)
	}
	return recorder.Body.String()
}
