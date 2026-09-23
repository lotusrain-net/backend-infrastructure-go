package httpkit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/apikit"
)

// ReadinessChecker reports whether service dependencies are ready.
type ReadinessChecker interface {
	// Ready reports whether service dependencies can serve traffic.
	Ready(context.Context) error
}

// RouterOptions configures the standard infrastructure service router.
type RouterOptions struct {
	// Logger receives access and recovery logs; nil uses slog.Default.
	Logger *slog.Logger
	// Readiness checks dependencies for the readiness endpoint.
	Readiness ReadinessChecker
	// Metrics serves the metrics endpoint.
	Metrics http.Handler
	// CORS configures cross-origin response headers.
	CORS CORSConfig
	// RateLimit optionally configures request limiting.
	RateLimit *RateLimitConfig
	// Register adds application routes after the standard routes.
	Register func(chi.Router)
}

// NewRouter builds a Chi router with health, metrics, and common middleware.
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
	// Chi's defaults write plain text. Keep unmatched and unsupported routes on
	// the same {code,msg,data} contract as application handlers.
	router.NotFound(func(writer http.ResponseWriter, _ *http.Request) {
		apikit.WriteError(writer, apikit.NotFound("resource"))
	})
	router.MethodNotAllowed(func(writer http.ResponseWriter, _ *http.Request) {
		apikit.WriteError(writer, apikit.MethodNotAllowed())
	})

	router.Get("/health/live", func(writer http.ResponseWriter, _ *http.Request) {
		apikit.Write(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(writer http.ResponseWriter, request *http.Request) {
		if err := options.Readiness.Ready(request.Context()); err != nil {
			apikit.WriteError(writer, apikit.ServiceUnavailable("service not ready", err))
			return
		}
		apikit.Write(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Handle("/metrics", options.Metrics)
	if options.Register != nil {
		options.Register(router)
	}

	return router, nil
}
