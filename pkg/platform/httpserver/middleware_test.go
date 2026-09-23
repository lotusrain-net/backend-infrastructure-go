package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	requestcontext "github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver"
)

func TestRequestIDAcceptsValidIncomingID(t *testing.T) {
	handler := httpserver.RequestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := requestcontext.RequestIDFromContext(request.Context()); got != "client-request-42" {
			t.Fatalf("context request id = %q", got)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(httpserver.RequestIDHeader, "client-request-42")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(httpserver.RequestIDHeader); got != "client-request-42" {
		t.Fatalf("response request id = %q", got)
	}
}

func TestRequestIDReplacesInvalidIncomingID(t *testing.T) {
	handler := httpserver.RequestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(httpserver.RequestIDHeader, "contains spaces and is invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	got := recorder.Header().Get(httpserver.RequestIDHeader)
	if got == "" || got == request.Header.Get(httpserver.RequestIDHeader) {
		t.Fatalf("generated request id = %q", got)
	}
}

func TestRecoveryReturnsStableInternalError(t *testing.T) {
	handler := httpserver.Recovery(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret panic")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret panic") || !strings.Contains(recorder.Body.String(), `"code":500`) {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestCORSHandlesAllowedPreflight(t *testing.T) {
	handler := httpserver.CORS(httpserver.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight must not call downstream handler")
	}))
	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := recorder.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q", got)
	}
}

func TestCORSDoesNotGrantUnknownOrigin(t *testing.T) {
	handler := httpserver.CORS(httpserver.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q", got)
	}
}

func TestAccessLogIncludesRequestIDRouteStatusAndDuration(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := chi.NewRouter()
	router.Use(httpserver.RequestID, httpserver.AccessLog(logger))
	router.Get("/users/{id}", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/42", nil))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["request_id"] == "" || record["route"] != "/users/{id}" || record["status"] != float64(http.StatusAccepted) {
		t.Fatalf("log record = %#v", record)
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Fatalf("duration missing: %#v", record)
	}
}

func TestResponseControllerFlushTraversesAccessLogWriter(t *testing.T) {
	downstreamSawFlusher := false
	handler := httpserver.AccessLog(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		controller := http.NewResponseController(writer)
		if err := controller.Flush(); err != nil {
			t.Fatalf("wrapped writer does not expose flush: %v", err)
		}
		downstreamSawFlusher = true
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/stream", nil))

	if !downstreamSawFlusher {
		t.Fatal("downstream handler was not invoked")
	}
}

func TestAccessLogDelegatesHTTP2Push(t *testing.T) {
	underlying := &pushRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := httpserver.AccessLog(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		pusher, ok := writer.(http.Pusher)
		if !ok {
			t.Fatal("wrapped writer does not implement http.Pusher")
		}
		if err := pusher.Push("/asset.js", nil); err != nil {
			t.Fatal(err)
		}
	}))

	handler.ServeHTTP(underlying, httptest.NewRequest(http.MethodGet, "/", nil))

	if underlying.pushed != "/asset.js" {
		t.Fatalf("pushed = %q", underlying.pushed)
	}
}

func TestAccessLogKeepsFirstCommittedStatus(t *testing.T) {
	var output bytes.Buffer
	handler := httpserver.AccessLog(slog.New(slog.NewJSONHandler(&output, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		writer.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["status"] != float64(http.StatusCreated) {
		t.Fatalf("logged status = %#v", record["status"])
	}
}

func TestRoutePatternNormalizesUnmatchedPaths(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/unknown/123?secret=value", nil)
	if got := httpserver.RoutePattern(request); got != "/unmatched" {
		t.Fatalf("route pattern = %q", got)
	}
}

func TestMemoryRateLimiterEnforcesBoundary(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpserver.NewMemoryRateLimiter(func() time.Time { return now })
	for attempt := 1; attempt <= 2; attempt++ {
		decision, err := limiter.Allow(context.Background(), "client", 2, time.Minute)
		if err != nil || !decision.Allowed {
			t.Fatalf("attempt %d: decision=%#v err=%v", attempt, decision, err)
		}
	}
	decision, err := limiter.Allow(context.Background(), "client", 2, time.Minute)
	if err != nil || decision.Allowed || decision.RetryAfter <= 0 {
		t.Fatalf("third attempt: decision=%#v err=%v", decision, err)
	}
}

func TestMemoryRateLimiterReleasesAttemptsAfterWindow(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpserver.NewMemoryRateLimiter(func() time.Time { return now })
	if decision, err := limiter.Allow(context.Background(), "client", 1, time.Minute); err != nil || !decision.Allowed {
		t.Fatalf("first decision=%#v err=%v", decision, err)
	}
	now = now.Add(time.Minute)
	if decision, err := limiter.Allow(context.Background(), "client", 1, time.Minute); err != nil || !decision.Allowed {
		t.Fatalf("post-window decision=%#v err=%v", decision, err)
	}
}

func TestRateLimitReturnsRetryHeadersWhenDenied(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpserver.NewMemoryRateLimiter(func() time.Time { return now })
	handler := httpserver.RateLimit(httpserver.RateLimitConfig{
		Limiter: limiter, Limit: 1, Window: time.Minute,
	})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" || recorder.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("headers = %#v", recorder.Header())
	}
}

func TestRateLimitRoundsRetryAfterUpToWholeSeconds(t *testing.T) {
	handler := httpserver.RateLimit(httpserver.RateLimitConfig{
		Limiter: fixedLimiter{decision: httpserver.RateLimitDecision{Allowed: false, RetryAfter: 1100 * time.Millisecond}},
		Limit:   1,
		Window:  time.Minute,
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("denied request must not reach downstream handler")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := recorder.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After = %q, want 2", got)
	}
}

func TestRateLimitFailsOpenForOrdinaryRoute(t *testing.T) {
	handler := httpserver.RateLimit(httpserver.RateLimitConfig{
		Limiter: failingLimiter{}, Limit: 10, Window: time.Minute,
	})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ordinary", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestRateLimitFailsClosedForCriticalRoute(t *testing.T) {
	handler := httpserver.RateLimit(httpserver.RateLimitConfig{
		Limiter: failingLimiter{}, Limit: 10, Window: time.Minute,
		Critical: func(request *http.Request) bool { return request.URL.Path == "/login" },
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("critical request must not reach downstream handler")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/login", nil))

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":503`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRateLimitWithNilLimiterFailsClosedForCriticalRoute(t *testing.T) {
	handler := httpserver.RateLimit(httpserver.RateLimitConfig{
		Limit: 10, Window: time.Minute,
		Critical: func(*http.Request) bool { return true },
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("critical request must not reach downstream handler")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/login", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestCORSValidateRejectsWildcardCredentials(t *testing.T) {
	err := (httpserver.CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true}).Validate()
	if err == nil {
		t.Fatal("wildcard origin with credentials must be rejected")
	}
}

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string, int, time.Duration) (httpserver.RateLimitDecision, error) {
	return httpserver.RateLimitDecision{}, errors.New("redis unavailable")
}

type fixedLimiter struct {
	decision httpserver.RateLimitDecision
}

type pushRecorder struct {
	*httptest.ResponseRecorder
	pushed string
}

func (recorder *pushRecorder) Push(target string, _ *http.PushOptions) error {
	recorder.pushed = target
	return nil
}

func (limiter fixedLimiter) Allow(context.Context, string, int, time.Duration) (httpserver.RateLimitDecision, error) {
	return limiter.decision, nil
}
