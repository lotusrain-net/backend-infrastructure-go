package httpserver

import (
	"context"
	"fmt"
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
	Register  func(chi.Router)
}

func NewRouter(options RouterOptions) (chi.Router, error) {
	if options.Readiness == nil {
		return nil, fmt.Errorf("readiness checker is required")
	}
	if options.Metrics == nil {
		return nil, fmt.Errorf("metrics handler is required")
	}
	if err := options.CORS.Validate(); err != nil {
		return nil, err
	}
	if options.RateLimit != nil {
		if err := options.RateLimit.Validate(); err != nil {
			return nil, err
		}
	}
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
		if err := options.Readiness.Ready(request.Context()); err != nil {
			response.WriteError(writer, apperror.ServiceUnavailable("service not ready", err))
			return
		}
		response.Write(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Handle("/metrics", options.Metrics)
	if options.Register != nil {
		options.Register(router)
	}

	return router, nil
}
