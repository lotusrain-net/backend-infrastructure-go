package httpkit_test

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

	"github.com/jyysy/backend-infrastructure-go/pkg/httpkit"
)

func TestRequestIDAcceptsValidIncomingID(t *testing.T) {
	handler := httpkit.RequestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := httpkit.RequestIDFromContext(request.Context()); got != "client-request-42" {
			t.Fatalf("context request id = %q", got)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(httpkit.RequestIDHeader, "client-request-42")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get(httpkit.RequestIDHeader); got != "client-request-42" {
		t.Fatalf("response request id = %q", got)
	}
}

func TestRequestIDReplacesInvalidIncomingID(t *testing.T) {
	handler := httpkit.RequestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(httpkit.RequestIDHeader, "contains spaces and is invalid")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	got := recorder.Header().Get(httpkit.RequestIDHeader)
	if got == "" || got == request.Header.Get(httpkit.RequestIDHeader) {
		t.Fatalf("generated request id = %q", got)
	}
}

func TestRecoveryReturnsStableInternalError(t *testing.T) {
	handler := httpkit.Recovery(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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

func TestRecoveryDiscardsPartialResponseAfterPanic(t *testing.T) {
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("partial response"))
		panic("secret panic")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "partial response") || strings.Contains(recorder.Body.String(), "secret panic") {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestRecoveryFlushesStreamingResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	visibleBeforeReturn := ""
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("data: ready\n\n"))
		if err := http.NewResponseController(writer).Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
		visibleBeforeReturn = recorder.Body.String()
	}))

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if visibleBeforeReturn != "data: ready\n\n" || !recorder.Flushed {
		t.Fatalf("visible body = %q, flushed = %v", visibleBeforeReturn, recorder.Flushed)
	}
}

func TestRecoveryExposesWorkingHTTPFlusher(t *testing.T) {
	recorder := httptest.NewRecorder()
	visibleBeforeReturn := ""
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		flusher, ok := writer.(http.Flusher)
		if !ok {
			t.Fatal("recovery writer does not implement http.Flusher")
		}
		_, _ = writer.Write([]byte("data: direct\n\n"))
		flusher.Flush()
		visibleBeforeReturn = recorder.Body.String()
	}))

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if visibleBeforeReturn != "data: direct\n\n" || !recorder.Flushed {
		t.Fatalf("visible body = %q, flushed = %v", visibleBeforeReturn, recorder.Flushed)
	}
}

func TestRecoveryStreamsResponsesLargerThanItsBoundedBuffer(t *testing.T) {
	recorder := httptest.NewRecorder()
	visibleBeforeReturn := 0
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		payload := strings.Repeat("x", 256<<10)
		if _, err := writer.Write([]byte(payload)); err != nil {
			t.Fatalf("write: %v", err)
		}
		visibleBeforeReturn = recorder.Body.Len()
	}))

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/large", nil))

	if visibleBeforeReturn == 0 || recorder.Body.Len() != 256<<10 {
		t.Fatalf("visible bytes = %d, final bytes = %d", visibleBeforeReturn, recorder.Body.Len())
	}
}

func TestRecoveryAbortsAfterFlushedResponsePanics(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("partial stream"))
		if err := http.NewResponseController(writer).Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
		panic("stream failed")
	}))

	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("panic = %#v", recovered)
		}
		if recorder.Body.String() != "partial stream" {
			t.Fatalf("body = %q", recorder.Body.String())
		}
	}()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))
	t.Fatal("expected connection abort panic")
}

func TestRecoveryUnwrapsResponseControllerWriteDeadline(t *testing.T) {
	underlying := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := httpkit.Recovery(slog.Default())(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(writer).SetWriteDeadline(time.Time{}); err != nil {
			t.Fatalf("set write deadline: %v", err)
		}
	}))

	handler.ServeHTTP(underlying, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if !underlying.writeDeadlineSet {
		t.Fatal("write deadline did not reach the underlying writer")
	}
}

func TestCORSHandlesAllowedPreflight(t *testing.T) {
	handler := httpkit.CORS(httpkit.CORSConfig{
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
	handler := httpkit.CORS(httpkit.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
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
	router.Use(httpkit.RequestID, httpkit.AccessLog(logger))
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
	handler := httpkit.AccessLog(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
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
	handler := httpkit.AccessLog(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
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
	handler := httpkit.AccessLog(slog.New(slog.NewJSONHandler(&output, nil)))(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
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
	if got := httpkit.RoutePattern(request); got != "/unmatched" {
		t.Fatalf("route pattern = %q", got)
	}
}

func TestMemoryRateLimiterEnforcesBoundary(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpkit.NewMemoryRateLimiter(func() time.Time { return now })
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
	limiter := httpkit.NewMemoryRateLimiter(func() time.Time { return now })
	if decision, err := limiter.Allow(context.Background(), "client", 1, time.Minute); err != nil || !decision.Allowed {
		t.Fatalf("first decision=%#v err=%v", decision, err)
	}
	now = now.Add(time.Minute)
	if decision, err := limiter.Allow(context.Background(), "client", 1, time.Minute); err != nil || !decision.Allowed {
		t.Fatalf("post-window decision=%#v err=%v", decision, err)
	}
}

func TestMemoryRateLimiterRejectsInvalidBoundariesWithoutPanicking(t *testing.T) {
	limiter := httpkit.NewMemoryRateLimiter(nil)
	for _, test := range []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{name: "zero limit", limit: 0, window: time.Minute},
		{name: "negative limit", limit: -1, window: time.Minute},
		{name: "zero window", limit: 1, window: 0},
		{name: "negative window", limit: 1, window: -time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := limiter.Allow(context.Background(), "client", test.limit, test.window); err == nil {
				t.Fatal("Allow() error = nil")
			}
		})
	}
}

func TestMemoryRateLimiterZeroValueIsUsable(t *testing.T) {
	var limiter httpkit.MemoryRateLimiter
	decision, err := limiter.Allow(context.Background(), "client", 1, time.Minute)
	if err != nil || !decision.Allowed || decision.Remaining != 0 {
		t.Fatalf("decision=%#v err=%v", decision, err)
	}
}

func TestRateLimitReturnsRetryHeadersWhenDenied(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpkit.NewMemoryRateLimiter(func() time.Time { return now })
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
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
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
		Limiter: fixedLimiter{decision: httpkit.RateLimitDecision{Allowed: false, RetryAfter: 1100 * time.Millisecond}},
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
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
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
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
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
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
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
	err := (httpkit.CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true}).Validate()
	if err == nil {
		t.Fatal("wildcard origin with credentials must be rejected")
	}
}

func TestCORSDirectUseFailsClosedForInvalidConfig(t *testing.T) {
	handler := httpkit.CORS(httpkit.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("allow credentials = %q", got)
	}
}

func TestRateLimitInvalidBoundariesFailClosedForCriticalRoute(t *testing.T) {
	handler := httpkit.RateLimit(httpkit.RateLimitConfig{
		Limiter:  fixedLimiter{decision: httpkit.RateLimitDecision{Allowed: true}},
		Limit:    0,
		Window:   time.Minute,
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

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string, int, time.Duration) (httpkit.RateLimitDecision, error) {
	return httpkit.RateLimitDecision{}, errors.New("redis unavailable")
}

type fixedLimiter struct {
	decision httpkit.RateLimitDecision
}

type pushRecorder struct {
	*httptest.ResponseRecorder
	pushed string
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	writeDeadlineSet bool
}

func (recorder *deadlineRecorder) SetWriteDeadline(time.Time) error {
	recorder.writeDeadlineSet = true
	return nil
}

func (recorder *pushRecorder) Push(target string, _ *http.PushOptions) error {
	recorder.pushed = target
	return nil
}

func (limiter fixedLimiter) Allow(context.Context, string, int, time.Duration) (httpkit.RateLimitDecision, error) {
	return limiter.decision, nil
}
