// Package httpserver is a compatibility adapter around pkg/httpkit.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
)

const RequestIDHeader = httpkit.RequestIDHeader

type CORSConfig = httpkit.CORSConfig
type RateLimiter = httpkit.RateLimiter
type RateLimitDecision = httpkit.RateLimitDecision
type RateLimitConfig = httpkit.RateLimitConfig
type MemoryRateLimiter = httpkit.MemoryRateLimiter

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return httpkit.WithRequestID(ctx, requestID)
}
func RequestIDFromContext(ctx context.Context) string               { return httpkit.RequestIDFromContext(ctx) }
func RequestID(next http.Handler) http.Handler                      { return httpkit.RequestID(next) }
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler  { return httpkit.Recovery(logger) }
func CORS(config CORSConfig) func(http.Handler) http.Handler        { return httpkit.CORS(config) }
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler { return httpkit.AccessLog(logger) }
func RoutePattern(request *http.Request) string                     { return httpkit.RoutePattern(request) }
func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	return httpkit.RateLimit(config)
}
func NewMemoryRateLimiter(now func() time.Time) *MemoryRateLimiter {
	return httpkit.NewMemoryRateLimiter(now)
}
