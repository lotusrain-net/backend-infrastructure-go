package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/audit"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
)

type auditListStub struct{ query audit.Query }

func (stub *auditListStub) List(_ context.Context, query audit.Query) (pagination.Page[audit.Event], error) {
	stub.query = query
	return pagination.New([]audit.Event{}, 1, 20, 0), nil
}

type taskAPIStub struct {
	submitted  int
	submission taskmodule.Submission
	execution  taskmodule.Execution
	listed     pagination.Page[taskmodule.Execution]
	query      taskmodule.ExecutionQuery
	getErr     error
	submitErr  error
	listErr    error
}

type taskObserverStub struct{ taskType, status string }

func (stub *taskObserverStub) ObserveTask(taskType, status string, _ time.Duration) {
	stub.taskType, stub.status = taskType, status
}

type permissionRecordingIAMStub struct {
	iamStub
	permissions []string
}

func (stub *permissionRecordingIAMStub) Authorize(_ context.Context, _ string, permission string) error {
	stub.permissions = append(stub.permissions, permission)
	return iam.ErrPermissionDenied
}

func (s *taskAPIStub) Submit(_ context.Context, submission taskmodule.Submission) (taskmodule.Execution, error) {
	s.submitted++
	s.submission = submission
	if s.submitErr != nil {
		return taskmodule.Execution{}, s.submitErr
	}
	if s.execution.ID != "" {
		return s.execution, nil
	}
	return taskmodule.Execution{ID: "execution-1", TaskType: submission.TaskType, Payload: submission.Payload, Status: taskmodule.StatusQueued}, nil
}

func TestSubmitTaskMapsMissingDefinitionToNotFound(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{submitErr: taskmodule.ErrDefinitionNotFound}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"definition_id":"00112233-4455-6677-8899-aabbccddeeff","task_type":"system.test","payload":{}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSubmitTaskMapsDomainValidationFailureToUnprocessableEntity(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{submitErr: fmt.Errorf("validate submission: %w", taskmodule.ErrInvalidSubmission)}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"task_type":"system.test","payload":{}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
func (s *taskAPIStub) GetExecution(context.Context, string) (taskmodule.Execution, error) {
	if s.getErr != nil {
		return taskmodule.Execution{}, s.getErr
	}
	if s.execution.ID != "" {
		return s.execution, nil
	}
	return taskmodule.Execution{ID: "execution-1", TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{}`), Status: taskmodule.StatusQueued}, nil
}

func (s *taskAPIStub) ListExecutions(_ context.Context, query taskmodule.ExecutionQuery) (pagination.Page[taskmodule.Execution], error) {
	s.query = query
	if s.listErr != nil {
		return pagination.Page[taskmodule.Execution]{}, s.listErr
	}
	if s.listed.Items != nil || s.listed.Meta.Size != 0 || s.listed.Meta.Total != 0 {
		return s.listed, nil
	}
	return pagination.New([]taskmodule.Execution{}, 1, 20, 0), nil
}

func TestPlatformRoutesUseAuthenticationRBACAndSharedEndpoints(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	audits := &auditListStub{}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: audits, Tasks: tasks, Executions: tasks})
	tests := []struct {
		method, path, body string
		want               int
	}{{http.MethodGet, "/api/v1/audit-logs?request_id=req-1&actor_id=user-1&result=success&resource_id=execution-1", "", http.StatusOK}, {http.MethodPost, "/api/v1/task-executions", `{"task_type":"system.test","payload":{"processed_rows":1}}`, http.StatusAccepted}, {http.MethodGet, "/api/v1/task-executions", "", http.StatusOK}, {http.MethodGet, "/api/v1/task-executions/execution-1", "", http.StatusOK}, {http.MethodGet, "/api/v1/task-types", "", http.StatusOK}}
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
	if tasks.submission.DefinitionID != "" {
		t.Fatalf("definition_id=%q, want nullable", tasks.submission.DefinitionID)
	}
	filter := audits.query.Filter
	if filter.ResourceID != "execution-1" || filter.RequestID != "req-1" || filter.ActorID != "user-1" || filter.Result != audit.ResultSuccess {
		t.Fatalf("filter=%+v", filter)
	}
}

func TestSubmitTaskAppliesDocumentedDefaultsAndQueueOptions(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"task_type":"system.test","payload":{},"unique_for_seconds":60,"process_after_seconds":15}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if tasks.submission.TaskType != taskmodule.SystemTestTaskType || tasks.submission.MaxRetries != 3 || tasks.submission.Timeout != 5*time.Minute {
		t.Fatalf("submission defaults = %+v", tasks.submission)
	}
	if tasks.submission.UniqueFor != time.Minute || tasks.submission.ProcessAfter != 15*time.Second {
		t.Fatalf("queue options = %+v", tasks.submission)
	}
	if strings.Contains(rec.Body.String(), "TaskType") || !strings.Contains(rec.Body.String(), `"task_type":"system.test"`) {
		t.Fatalf("response does not use snake_case: %s", rec.Body.String())
	}
}

func TestSubmitTaskAuditsAuthenticatedActorAndObservesQueuedExecution(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	recorder := &preserveStub{}
	observer := &taskObserverStub{}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks, Recorder: recorder, TaskObserver: observer})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"task_type":"system.test","payload":{}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if recorder.event.ActorID == nil || *recorder.event.ActorID != "user-1" {
		t.Fatalf("audit actor = %v", recorder.event.ActorID)
	}
	if observer.taskType != taskmodule.SystemTestTaskType || observer.status != string(taskmodule.StatusQueued) {
		t.Fatalf("task metric = %q %q", observer.taskType, observer.status)
	}
}

func TestSubmitTaskPreservesExplicitZeroRetries(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"task_type":"system.test","payload":{},"max_retries":0,"timeout_seconds":10}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if tasks.submission.MaxRetries != 0 || tasks.submission.Timeout != 10*time.Second {
		t.Fatalf("explicit options = %+v", tasks.submission)
	}
}

func TestSubmitTaskUsesInjectedTaskCatalog(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{}
	catalog, err := taskmodule.NewTaskCatalog(taskmodule.TaskRegistration{TaskType: "report.generate", Handler: taskmodule.SystemTestHandler{}})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks, TaskCatalog: catalog})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(`{"task_type":"report.generate","payload":{}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || tasks.submission.TaskType != "report.generate" {
		t.Fatalf("status=%d body=%s submission=%+v", rec.Code, rec.Body.String(), tasks.submission)
	}
}

func TestSubmitTaskRejectsRequestsOutsideDocumentedContract(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tests := []string{
		`{"payload":{}}`,
		`{"task_type":"report.generate","payload":{}}`,
		`{"task_type":"system.test"}`,
		`{"task_type":"system.test","payload":[]}`,
		`{"task_type":"system.test","payload":{},"definition_id":"not-a-uuid"}`,
		`{"task_type":"system.test","payload":{},"definition_id":""}`,
		`{"task_type":"system.test","payload":{},"idempotency_key":""}`,
		`{"task_type":"system.test","payload":{},"max_retries":-1}`,
		`{"task_type":"system.test","payload":{},"max_retries":26}`,
		`{"task_type":"system.test","payload":{},"max_retries":null}`,
		`{"task_type":"system.test","payload":{},"timeout_seconds":0}`,
		`{"task_type":"system.test","payload":{},"timeout_seconds":86401}`,
		`{"task_type":"system.test","payload":{},"timeout_seconds":9223372036854775807}`,
		`{"task_type":"system.test","payload":{},"unique_for_seconds":-1}`,
		`{"task_type":"system.test","payload":{},"unique_for_seconds":604801}`,
		`{"task_type":"system.test","payload":{},"process_after_seconds":-1}`,
		`{"task_type":"system.test","payload":{},"process_after_seconds":2592001}`,
		`{"task_type":"system.test","payload":{}} {}`,
	}
	for _, body := range tests {
		t.Run(body, func(t *testing.T) {
			tasks := &taskAPIStub{}
			router := chi.NewRouter()
			RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/task-executions", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if tasks.submitted != 0 {
				t.Fatalf("Submit() called %d times", tasks.submitted)
			}
		})
	}
}

func TestGetExecutionMapsMissingExecutionToNotFound(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{getErr: fmt.Errorf("lookup: %w", taskmodule.ErrExecutionNotFound)}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-executions/00000000-0000-0000-0000-000000000001", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListExecutionsRejectsInvalidStatusAndReturnsPaginatedEnvelope(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tasks := &taskAPIStub{listed: pagination.New([]taskmodule.Execution{{
		ID: "execution-2", TaskType: "report.generate", Payload: json.RawMessage(`{}`), Status: taskmodule.StatusSucceeded,
	}}, 2, 5, 6)}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: tasks, Executions: tasks, TaskCatalog: runtimeTaskCatalog})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-executions?page=2&size=5&task_type=report.generate&status=succeeded", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if tasks.query.Filter.TaskType != "report.generate" || tasks.query.Filter.Status != taskmodule.StatusSucceeded || tasks.query.Page != 2 || tasks.query.Size != 5 {
		t.Fatalf("query=%+v", tasks.query)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/task-executions?status=bogus", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListTaskTypesReturnsDeterministicCatalogOrder(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	catalog, err := taskmodule.NewTaskCatalog(
		taskmodule.TaskRegistration{TaskType: "zeta.task", Handler: taskmodule.SystemTestHandler{}},
		taskmodule.TaskRegistration{TaskType: "alpha.task", Handler: taskmodule.SystemTestHandler{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterPlatformRoutes(router, PlatformRoutes{IAM: iamStub{}, JWT: jwt, Audits: &auditListStub{}, Tasks: &taskAPIStub{}, Executions: &taskAPIStub{}, TaskCatalog: catalog})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-types", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data []struct {
			TaskType string `json:"task_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data) != 2 || envelope.Data[0].TaskType != "alpha.task" || envelope.Data[1].TaskType != "zeta.task" {
		t.Fatalf("task types=%+v", envelope.Data)
	}
}

func TestPlatformReadRoutesRequireTasksReadPermission(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("user-1")
	tests := []struct {
		name string
		path string
	}{
		{name: "executions", path: "/api/v1/task-executions"},
		{name: "task types", path: "/api/v1/task-types"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &permissionRecordingIAMStub{}
			router := chi.NewRouter()
			RegisterPlatformRoutes(router, PlatformRoutes{IAM: app, JWT: jwt, Audits: &auditListStub{}, Tasks: &taskAPIStub{}, Executions: &taskAPIStub{}})

			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if len(app.permissions) != 1 || app.permissions[0] != "tasks:read" {
				t.Fatalf("requested permissions=%v", app.permissions)
			}
		})
	}
}
