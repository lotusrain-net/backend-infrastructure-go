package httpkit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jyysy/backend-infrastructure-go/pkg/apikit"
)

// RequestIDHeader is the HTTP header used to propagate request identifiers.
const RequestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type requestIDKey struct{}

// WithRequestID returns a child context carrying requestID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestIDFromContext returns the request identifier stored in ctx.
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// RequestID accepts valid caller identifiers or generates a new identifier.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}
		writer.Header().Set(RequestIDHeader, requestID)
		ctx := WithRequestID(request.Context(), requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value[:])
}

const recoveryBufferLimit = 64 << 10

// Recovery converts panics before response commitment to stable 500 responses
// and records the panic. Flushed or larger responses switch to streaming; a
// later panic aborts the connection because their status can no longer change.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	logger = defaultLogger(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			buffered := newRecoveryWriter(writer)
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered == http.ErrAbortHandler {
						panic(recovered)
					}
					logger.ErrorContext(request.Context(), "panic recovered",
						"request_id", RequestIDFromContext(request.Context()),
						"panic", fmt.Sprint(recovered),
					)
					if buffered.committed {
						panic(http.ErrAbortHandler)
					}
					apikit.WriteError(writer, apikit.New(
						http.StatusInternalServerError, "internal server error", http.StatusInternalServerError, nil,
					))
					return
				}
				_ = buffered.commit()
			}()
			next.ServeHTTP(buffered, request)
		})
	}
}

type recoveryWriter struct {
	destination http.ResponseWriter
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
	committed   bool
}

func newRecoveryWriter(destination http.ResponseWriter) *recoveryWriter {
	return &recoveryWriter{destination: destination, header: destination.Header().Clone()}
}

func (writer *recoveryWriter) Header() http.Header {
	if writer.committed {
		return writer.destination.Header()
	}
	return writer.header
}

func (writer *recoveryWriter) WriteHeader(status int) {
	if writer.committed {
		writer.destination.WriteHeader(status)
		return
	}
	if writer.wroteHeader {
		return
	}
	writer.wroteHeader = true
	writer.status = status
}

func (writer *recoveryWriter) Write(value []byte) (int, error) {
	if writer.committed {
		return writer.destination.Write(value)
	}
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	if len(value) <= recoveryBufferLimit-writer.body.Len() {
		return writer.body.Write(value)
	}
	if err := writer.commit(); err != nil {
		return 0, err
	}
	return writer.destination.Write(value)
}

// Flush commits buffered data and switches subsequent writes to streaming.
func (writer *recoveryWriter) Flush() {
	_ = writer.FlushError()
}

// FlushError commits buffered data and reports errors from the underlying flush.
func (writer *recoveryWriter) FlushError() error {
	if err := writer.commit(); err != nil {
		return err
	}
	return http.NewResponseController(writer.destination).Flush()
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (writer *recoveryWriter) Unwrap() http.ResponseWriter {
	return writer.destination
}

func (writer *recoveryWriter) commit() error {
	if writer.committed {
		return nil
	}
	writer.committed = true
	for key := range writer.destination.Header() {
		delete(writer.destination.Header(), key)
	}
	for key, values := range writer.header {
		writer.destination.Header()[key] = append([]string(nil), values...)
	}
	if writer.wroteHeader {
		writer.destination.WriteHeader(writer.status)
	}
	if writer.body.Len() > 0 {
		_, err := writer.destination.Write(writer.body.Bytes())
		writer.body.Reset()
		return err
	}
	return nil
}

// CORSConfig describes allowed origins, methods, headers, and credentials.
type CORSConfig struct {
	// AllowedOrigins lists exact origins or the wildcard origin.
	AllowedOrigins []string
	// AllowedMethods lists methods accepted during preflight.
	AllowedMethods []string
	// AllowedHeaders lists headers accepted during preflight.
	AllowedHeaders []string
	// ExposedHeaders lists response headers visible to browser clients.
	ExposedHeaders []string
	// AllowCredentials permits browser credentials for exact origins.
	AllowCredentials bool
	// MaxAge is the browser preflight cache duration.
	MaxAge time.Duration
}

// Validate verifies that the CORS settings are safe and coherent.
func (config CORSConfig) Validate() error {
	if config.AllowCredentials {
		for _, origin := range config.AllowedOrigins {
			if origin == "*" {
				return fmt.Errorf("CORS wildcard origin cannot be combined with credentials")
			}
		}
	}
	return nil
}

// CORS applies the configured cross-origin response policy. Invalid settings
// disable cross-origin response headers so direct use remains fail-closed.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	valid := config.Validate() == nil
	origins := stringSet(config.AllowedOrigins)
	methods := strings.Join(config.AllowedMethods, ", ")
	headers := strings.Join(config.AllowedHeaders, ", ")
	exposed := strings.Join(config.ExposedHeaders, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if !valid {
				next.ServeHTTP(writer, request)
				return
			}
			origin := request.Header.Get("Origin")
			writer.Header().Add("Vary", "Origin")
			if origin == "" || (!origins[origin] && !origins["*"]) {
				next.ServeHTTP(writer, request)
				return
			}
			allowedOrigin := origin
			if origins["*"] && !config.AllowCredentials {
				allowedOrigin = "*"
			}
			writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			if config.AllowCredentials {
				writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if exposed != "" {
				writer.Header().Set("Access-Control-Expose-Headers", exposed)
			}
			if request.Method == http.MethodOptions && request.Header.Get("Access-Control-Request-Method") != "" {
				writer.Header().Add("Vary", "Access-Control-Request-Method")
				writer.Header().Add("Vary", "Access-Control-Request-Headers")
				writer.Header().Set("Access-Control-Allow-Methods", methods)
				writer.Header().Set("Access-Control-Allow-Headers", headers)
				if config.MaxAge > 0 {
					writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(config.MaxAge.Seconds())))
				}
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

// AccessLog records normalized route, status, size, duration, and request ID.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	logger = defaultLogger(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			wrapped := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(wrapped, request)
			logger.InfoContext(request.Context(), "http request",
				"request_id", RequestIDFromContext(request.Context()),
				"method", request.Method,
				"route", RoutePattern(request),
				"status", wrapped.status,
				"bytes", wrapped.bytes,
				"duration_ms", float64(time.Since(started).Microseconds())/1000,
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (writer *statusWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *statusWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := writer.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

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
	written, err := writer.ResponseWriter.Write(value)
	writer.bytes += written
	return written, err
}

// RoutePattern returns the matched Chi route pattern or /unmatched.
func RoutePattern(request *http.Request) string {
	if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "/unmatched"
}

// RateLimiter decides whether a key may proceed within a time window.
type RateLimiter interface {
	// Allow decides whether key may proceed within the requested boundary.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error)
}

// RateLimitDecision describes a rate-limit backend decision.
type RateLimitDecision struct {
	// Allowed reports whether the request is within the configured limit.
	Allowed bool
	// Remaining is the number of requests left in the current window.
	Remaining int
	// RetryAfter is the delay before a denied request may be retried.
	RetryAfter time.Duration
}

// RateLimitConfig configures the generic rate-limit middleware.
type RateLimitConfig struct {
	// Limiter is the injected rate-limit backend.
	Limiter RateLimiter
	// Limit is the maximum number of requests in Window.
	Limit int
	// Window is the duration over which Limit applies.
	Window time.Duration
	// Key derives the backend key; nil uses the remote client address.
	Key func(*http.Request) string
	// Critical identifies routes that fail closed when limiting is unavailable.
	Critical func(*http.Request) bool
}

// Validate verifies that rate limiting has a backend and positive boundaries.
func (config RateLimitConfig) Validate() error {
	if config.Limiter == nil {
		return fmt.Errorf("rate limiter is required")
	}
	if config.Limit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}
	if config.Window <= 0 {
		return fmt.Errorf("rate limit window must be positive")
	}
	return nil
}

// RateLimit applies fail-open or critical-route fail-closed limiting.
func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	key := config.Key
	if key == nil {
		key = clientKey
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if config.Limiter == nil || config.Limit <= 0 || config.Window <= 0 {
				if config.Critical != nil && config.Critical(request) {
					apikit.WriteError(writer, apikit.ServiceUnavailable("rate limiter unavailable", nil))
					return
				}
				next.ServeHTTP(writer, request)
				return
			}
			decision, err := config.Limiter.Allow(request.Context(), key(request), config.Limit, config.Window)
			if err != nil {
				if config.Critical != nil && config.Critical(request) {
					apikit.WriteError(writer, apikit.ServiceUnavailable("rate limiter unavailable", err))
					return
				}
				next.ServeHTTP(writer, request)
				return
			}
			writer.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.Limit))
			writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
			if !decision.Allowed {
				retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				writer.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				apikit.WriteError(writer, apikit.TooManyRequests())
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

// MemoryRateLimiter is an in-process sliding-window limiter.
type MemoryRateLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	attempts map[string][]time.Time
}

// NewMemoryRateLimiter creates a deterministic reference backend for tests and local development.
// Production deployments should inject a distributed implementation of RateLimiter.
func NewMemoryRateLimiter(now func() time.Time) *MemoryRateLimiter {
	if now == nil {
		now = time.Now
	}
	return &MemoryRateLimiter{now: now, attempts: make(map[string][]time.Time)}
}

// Allow implements RateLimiter with an in-process sliding window.
func (limiter *MemoryRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error) {
	if limiter == nil {
		return RateLimitDecision{}, errors.New("memory rate limiter is nil")
	}
	if limit <= 0 {
		return RateLimitDecision{}, errors.New("rate limit must be positive")
	}
	if window <= 0 {
		return RateLimitDecision{}, errors.New("rate limit window must be positive")
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if limiter.now == nil {
		limiter.now = time.Now
	}
	if limiter.attempts == nil {
		limiter.attempts = make(map[string][]time.Time)
	}
	now := limiter.now()
	cutoff := now.Add(-window)
	attempts := limiter.attempts[key]
	firstActive := 0
	for firstActive < len(attempts) && !attempts[firstActive].After(cutoff) {
		firstActive++
	}
	attempts = attempts[firstActive:]
	if len(attempts) >= limit {
		limiter.attempts[key] = attempts
		return RateLimitDecision{Allowed: false, Remaining: 0, RetryAfter: attempts[0].Add(window).Sub(now)}, nil
	}
	attempts = append(attempts, now)
	limiter.attempts[key] = attempts
	return RateLimitDecision{Allowed: true, Remaining: limit - len(attempts)}, nil
}

func defaultLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}
