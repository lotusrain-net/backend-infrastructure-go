package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/requestcontext"
	"backend-infrastructure-go/internal/shared/response"
)

const RequestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}
		writer.Header().Set(RequestIDHeader, requestID)
		ctx := requestcontext.WithRequestID(request.Context(), requestID)
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

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	logger = defaultLogger(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(request.Context(), "panic recovered",
						"request_id", requestcontext.RequestID(request.Context()),
						"panic", fmt.Sprint(recovered),
					)
					response.WriteError(writer, apperror.New(
						http.StatusInternalServerError, "internal server error", http.StatusInternalServerError, nil,
					))
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func CORS(config CORSConfig) func(http.Handler) http.Handler {
	origins := stringSet(config.AllowedOrigins)
	methods := strings.Join(config.AllowedMethods, ", ")
	headers := strings.Join(config.AllowedHeaders, ", ")
	exposed := strings.Join(config.ExposedHeaders, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
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

func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	logger = defaultLogger(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			wrapped := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(wrapped, request)
			logger.InfoContext(request.Context(), "http request",
				"request_id", requestcontext.RequestID(request.Context()),
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
	status int
	bytes  int
}

func (writer *statusWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(value []byte) (int, error) {
	written, err := writer.ResponseWriter.Write(value)
	writer.bytes += written
	return written, err
}

func RoutePattern(request *http.Request) string {
	if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "/unmatched"
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error)
}

type RateLimitDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type RateLimitConfig struct {
	Limiter  RateLimiter
	Limit    int
	Window   time.Duration
	Key      func(*http.Request) string
	Critical func(*http.Request) bool
}

func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	key := config.Key
	if key == nil {
		key = clientKey
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if config.Limiter == nil || config.Limit <= 0 || config.Window <= 0 {
				next.ServeHTTP(writer, request)
				return
			}
			decision, err := config.Limiter.Allow(request.Context(), key(request), config.Limit, config.Window)
			if err != nil {
				if config.Critical != nil && config.Critical(request) {
					response.WriteError(writer, apperror.ServiceUnavailable("rate limiter unavailable", err))
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
				response.WriteError(writer, apperror.TooManyRequests())
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

type MemoryRateLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	attempts map[string][]time.Time
}

// NewMemoryRateLimiter creates a deterministic reference backend for tests and local development.
// Production deployments should inject a distributed Redis implementation of RateLimiter.
func NewMemoryRateLimiter(now func() time.Time) *MemoryRateLimiter {
	if now == nil {
		now = time.Now
	}
	return &MemoryRateLimiter{now: now, attempts: make(map[string][]time.Time)}
}

func (limiter *MemoryRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

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
