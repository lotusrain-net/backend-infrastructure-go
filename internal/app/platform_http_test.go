package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/iam"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/shared/pagination"
	"github.com/go-chi/chi/v5"
)

type auditListStub struct{}

func (auditListStub) List(context.Context, audit.Query) (pagination.Page[audit.Event], error) {
	return pagination.New([]audit.Event{}, 1, 20, 0), nil
}

type taskAPIStub struct{ submitted int }

func (s *taskAPIStub) Submit(context.Context, taskmodule.Submission) (taskmodule.Execution, error) {
	s.submitted++
	return taskmodule.Execution{ID: "execution-1", Status: taskmodule.StatusQueued}, nil
}
func (s *taskAPIStub) GetExecution(context.Context, string) (taskmodule.Execution, error) {
	return taskmodule.Execution{ID: "execution-1", Status: taskmodule.StatusQueued}, nil
}

func TestPlatformRoutesUseAuthenticationRBACAndSharedEndpoints(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: auditListStub{}, Tasks: tasks, Executions: tasks})
	tests := []struct {
		method, path, body string
		want               int
	}{{http.MethodGet, "/api/v1/audit-logs", "", http.StatusOK}, {http.MethodPost, "/api/v1/task-executions", `{"task_type":"system.test","payload":{"processed_rows":1}}`, http.StatusCreated}, {http.MethodGet, "/api/v1/task-executions/execution-1", "", http.StatusOK}}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tt.want {
			t.Fatalf("%s %s status=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
	if tasks.submitted != 1 {
		t.Fatalf("submitted=%d", tasks.submitted)
	}
}
