package observability

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Namespace         string
	KnownRoutes       []string
	KnownDependencies []string
	KnownTaskTypes    []string
}

type Metrics struct {
	registry       *prometheus.Registry
	routes         map[string]struct{}
	dependencies   map[string]struct{}
	taskTypes      map[string]struct{}
	httpRequests   *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	dependencyOps  *prometheus.CounterVec
	dependencyTime *prometheus.HistogramVec
	auditRecords   *prometheus.CounterVec
	auditFailures  *prometheus.CounterVec
	taskExecutions *prometheus.CounterVec
	taskDuration   *prometheus.HistogramVec
}

func New(config Config) *Metrics {
	namespace := strings.TrimSpace(config.Namespace)
	if namespace == "" {
		namespace = "backend"
	}
	metrics := &Metrics{
		registry:       prometheus.NewRegistry(),
		routes:         routeSet(config.KnownRoutes),
		dependencies:   set(config.KnownDependencies),
		taskTypes:      set(config.KnownTaskTypes),
		httpRequests:   prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "http_requests_total", Help: "HTTP requests."}, []string{"method", "route", "status_class"}),
		httpDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: "http_request_duration_seconds", Help: "HTTP request latency."}, []string{"method", "route", "status_class"}),
		dependencyOps:  prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "dependency_operations_total", Help: "Dependency operations."}, []string{"dependency", "outcome"}),
		dependencyTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: "dependency_operation_duration_seconds", Help: "Dependency operation latency."}, []string{"dependency", "outcome"}),
		auditRecords:   prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "audit_records_total", Help: "Audit records attempted successfully."}, []string{"category", "result"}),
		auditFailures:  prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "audit_write_failures_total", Help: "Audit backend write failures."}, []string{"category"}),
		taskExecutions: prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "task_executions_total", Help: "Task executions."}, []string{"task_type", "status"}),
		taskDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: "task_execution_duration_seconds", Help: "Task execution latency."}, []string{"task_type", "status"}),
	}
	metrics.registry.MustRegister(metrics.httpRequests, metrics.httpDuration, metrics.dependencyOps, metrics.dependencyTime, metrics.auditRecords, metrics.auditFailures, metrics.taskExecutions, metrics.taskDuration)
	return metrics
}

func (metrics *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

func (metrics *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(wrapped, request)
		route := NormalizePath(request.URL.Path)
		if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
			if pattern := routeContext.RoutePattern(); pattern != "" {
				route = pattern
			}
		}
		metrics.ObserveHTTP(request.Method, route, wrapped.status, time.Since(started))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *statusWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.wroteHeader = true
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(value []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(value)
}

func (metrics *Metrics) ObserveHTTP(method, path string, status int, duration time.Duration) {
	method = normalizeMethod(method)
	route := NormalizePath(path)
	if _, known := metrics.routes[route]; !known {
		route = "/unmatched"
	}
	class := statusClass(status)
	metrics.httpRequests.WithLabelValues(method, route, class).Inc()
	metrics.httpDuration.WithLabelValues(method, route, class).Observe(duration.Seconds())
}

func (metrics *Metrics) ObserveDependency(dependency, outcome string, duration time.Duration) {
	dependency = knownOrOther(dependency, metrics.dependencies)
	outcome = enum(outcome, "success", "failure", "timeout")
	metrics.dependencyOps.WithLabelValues(dependency, outcome).Inc()
	metrics.dependencyTime.WithLabelValues(dependency, outcome).Observe(duration.Seconds())
}

func (metrics *Metrics) ObserveAudit(action, result string) {
	metrics.auditRecords.WithLabelValues(actionCategory(action), enum(result, "success", "failure")).Inc()
}

func (metrics *Metrics) AuditWriteFailed(action string) {
	metrics.auditFailures.WithLabelValues(actionCategory(action)).Inc()
}

func (metrics *Metrics) ObserveTask(taskType, status string, duration time.Duration) {
	taskType = knownOrOther(taskType, metrics.taskTypes)
	status = enum(status, "queued", "running", "succeeded", "failed", "cancelled")
	metrics.taskExecutions.WithLabelValues(taskType, status).Inc()
	metrics.taskDuration.WithLabelValues(taskType, status).Observe(duration.Seconds())
}

var (
	numericID = regexp.MustCompile(`^[0-9]+$`)
	uuidID    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	hexID     = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
)

func NormalizePath(path string) string {
	path = strings.TrimSpace(strings.SplitN(path, "?", 2)[0])
	if path == "" || path[0] != '/' {
		return "/unmatched"
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		return "/"
	}
	for index, segment := range segments {
		if (strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")) || numericID.MatchString(segment) || uuidID.MatchString(segment) || hexID.MatchString(segment) {
			segments[index] = "{id}"
		}
	}
	return "/" + strings.Join(segments, "/")
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(method)
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	default:
		return "OTHER"
	}
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "other"
	}
	return strconv.Itoa(status/100) + "xx"
}

func actionCategory(action string) string {
	prefix, _, _ := strings.Cut(strings.ToLower(action), ".")
	switch prefix {
	case "auth", "authorization", "administration", "task", "scheduling":
		return prefix
	default:
		return "other"
	}
}

func enum(value string, allowed ...string) string {
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return "other"
}

func knownOrOther(value string, known map[string]struct{}) string {
	if _, ok := known[value]; ok {
		return value
	}
	return "other"
}

func set(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func routeSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[NormalizePath(value)] = struct{}{}
	}
	return result
}
