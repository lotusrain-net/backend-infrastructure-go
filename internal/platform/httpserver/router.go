package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/response"
)

type ReadinessChecker interface {
	Ready(context.Context) error
}

type RouterOptions struct {
	Logger    *slog.Logger
	Readiness ReadinessChecker
	Metrics   http.Handler
	CORS      CORSConfig
	RateLimit *RateLimitConfig
}

func NewRouter(options RouterOptions) http.Handler {
	router := chi.NewRouter()
	router.Use(RequestID)
	router.Use(AccessLog(options.Logger))
	router.Use(Recovery(options.Logger))
	if len(options.CORS.AllowedOrigins) > 0 {
		router.Use(CORS(options.CORS))
	}
	if options.RateLimit != nil {
		router.Use(RateLimit(*options.RateLimit))
	}

	router.Get("/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		response.Write(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		if options.Readiness != nil {
			if err := options.Readiness.Ready(request.Context()); err != nil {
				response.WriteError(writer, apperror.ServiceUnavailable("service not ready", err))
				return
			}
		}
		response.Write(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	if options.Metrics != nil {
		router.Handle("/metrics", options.Metrics)
	}

	return router
}
