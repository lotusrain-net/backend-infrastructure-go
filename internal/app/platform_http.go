package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/iam"
	taskmodule "backend-infrastructure-go/internal/modules/task"
	"backend-infrastructure-go/internal/shared/apperror"
	"backend-infrastructure-go/internal/shared/pagination"
	"backend-infrastructure-go/internal/shared/requestcontext"
	"backend-infrastructure-go/internal/shared/response"
	"github.com/go-chi/chi/v5"
)

type auditLister interface {
	List(context.Context, audit.Query) (pagination.Page[audit.Event], error)
}
type taskSubmitter interface {
	Submit(context.Context, taskmodule.Submission) (taskmodule.Execution, error)
}
type taskReader interface {
	GetExecution(context.Context, string) (taskmodule.Execution, error)
}
type PlatformRoutes struct {
	IAM        iam.Application
	JWT        *iam.JWTManager
	Audits     auditLister
	Tasks      taskSubmitter
	Executions taskReader
	Recorder   auditPreserver
}

func RegisterPlatformRoutes(router chi.Router, deps PlatformRoutes) {
	handler := platformHandler{deps: deps}
	router.Group(func(r chi.Router) {
		r.Use(iam.Authenticate(deps.JWT))
		r.With(iam.RequirePermission(deps.IAM, "audit:read")).Get("/api/v1/audit-logs", handler.auditLogs)
		r.With(iam.RequirePermission(deps.IAM, "tasks:write")).Post("/api/v1/task-executions", handler.submitTask)
		r.With(iam.RequirePermission(deps.IAM, "tasks:read")).Get("/api/v1/task-executions/{executionID}", handler.getExecution)
	})
}

type platformHandler struct{ deps PlatformRoutes }

func (h platformHandler) auditLogs(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	size := queryInt(r, "size", 20)
	query := r.URL.Query()
	result, err := h.deps.Audits.List(r.Context(), audit.Query{Filter: audit.Filter{
		RequestID: query.Get("request_id"), ActorID: query.Get("actor_id"), Action: query.Get("action"),
		Result: audit.Result(query.Get("result")), ResourceType: query.Get("resource_type"), ResourceID: query.Get("resource_id"),
		From: queryTime(query.Get("from")), To: queryTime(query.Get("to")),
	}, Page: page, Size: size})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.Write(w, http.StatusOK, result)
}
func (h platformHandler) submitTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DefinitionID   string          `json:"definition_id"`
		TaskType       string          `json:"task_type"`
		Payload        json.RawMessage `json:"payload"`
		IdempotencyKey string          `json:"idempotency_key"`
		MaxRetries     int             `json:"max_retries"`
		TimeoutSeconds int             `json:"timeout_seconds"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		response.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
		return
	}
	if in.TaskType != "" && in.TaskType != taskmodule.SystemTestTaskType {
		response.WriteError(w, apperror.Validation(map[string]string{"task_type": "only system.test is supported"}))
		return
	}
	if len(in.Payload) == 0 {
		in.Payload = json.RawMessage(`{}`)
	}
	execution, err := h.deps.Tasks.Submit(r.Context(), taskmodule.Submission{DefinitionID: in.DefinitionID, TaskType: taskmodule.SystemTestTaskType, Payload: in.Payload, IdempotencyKey: in.IdempotencyKey, MaxRetries: in.MaxRetries, Timeout: time.Duration(in.TimeoutSeconds) * time.Second})
	h.audit(r.Context(), "task.submit", "task_execution", execution.ID, err)
	if err != nil {
		if errors.Is(err, taskmodule.ErrDuplicateSubmission) {
			response.WriteError(w, apperror.New(http.StatusConflict, "duplicate task submission", http.StatusConflict, err))
			return
		}
		response.WriteError(w, err)
		return
	}
	response.Write(w, http.StatusAccepted, execution)
}
func (h platformHandler) getExecution(w http.ResponseWriter, r *http.Request) {
	execution, err := h.deps.Executions.GetExecution(r.Context(), chi.URLParam(r, "executionID"))
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.Write(w, http.StatusOK, execution)
}
func (h platformHandler) audit(ctx context.Context, action, resourceType, resourceID string, primary error) {
	if h.deps.Recorder == nil {
		return
	}
	result := audit.ResultSuccess
	if primary != nil {
		result = audit.ResultFailure
	}
	_ = h.deps.Recorder.Preserve(ctx, audit.NewEvent{RequestID: requestcontext.RequestID(ctx), Action: action, Result: result, ResourceType: resourceType, ResourceID: resourceID}, primary)
}
func queryInt(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func queryTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}
