package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type readyFunc func(context.Context) error

func (fn readyFunc) Ready(ctx context.Context) error { return fn(ctx) }

func TestNewAPIRouterMountsHealthMetricsAndIAMRegistrar(t *testing.T) {
	registered := false
	router, err := NewAPIRouter(APIRouterOptions{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Readiness: readyFunc(func(context.Context) error { return nil }),
		Metrics:   http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }),
		RegisterIAM: func(r chi.Router) {
			registered = true
			r.Get("/api/v1/test-iam", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("IAM registrar was not invoked")
	}
	for path, want := range map[string]int{"/health/ready": http.StatusOK, "/metrics": http.StatusAccepted, "/api/v1/test-iam": http.StatusNoContent} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("%s status=%d want=%d", path, rec.Code, want)
		}
	}
}
