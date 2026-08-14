package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/audit"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/httpserver/iamhttp"
	apperror "github.com/jyysy/backend-infrastructure-go/pkg/apikit"
	requestcontext "github.com/jyysy/backend-infrastructure-go/pkg/httpkit"
	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
)

type auditLister interface {
	List(context.Context, audit.Query) (pagination.Page[audit.Event], error)
}
type taskSubmitter interface {
	Submit(context.Context, taskmodule.Submission) (taskmodule.Execution, error)
}
type taskReader interface {
	GetExecution(context.Context, string) (taskmodule.Execution, error)
	ListExecutions(context.Context, taskmodule.ExecutionQuery) (pagination.Page[taskmodule.Execution], error)
}
type taskObserver interface {
	ObserveTask(taskType, status string, duration time.Duration)
}
type taskCatalog interface {
	Contains(taskType string) bool
	Types() []string
}
type PlatformRoutes struct {
	IAM          iam.Application
	JWT          *iam.JWTManager
	Audits       auditLister
	Tasks        taskSubmitter
	Executions   taskReader
	Recorder     auditPreserver
	TaskObserver taskObserver
	TaskCatalog  taskCatalog
}

func RegisterPlatformRoutes(router chi.Router, deps PlatformRoutes) {
	if deps.TaskCatalog == nil {
		deps.TaskCatalog = runtimeTaskCatalog
	}
	handler := platformHandler{deps: deps}
	router.Group(func(r chi.Router) {
		r.Use(iamhttp.Authenticate(deps.JWT))
		r.With(iamhttp.RequirePermission(deps.IAM, "audit:read")).Get("/api/v1/audit-logs", handler.auditLogs)
		r.With(iamhttp.RequirePermission(deps.IAM, "tasks:write")).Post("/api/v1/task-executions", handler.submitTask)
		r.With(iamhttp.RequirePermission(deps.IAM, "tasks:read")).Get("/api/v1/task-executions", handler.listExecutions)
		r.With(iamhttp.RequirePermission(deps.IAM, "tasks:read")).Get("/api/v1/task-executions/{executionID}", handler.getExecution)
		r.With(iamhttp.RequirePermission(deps.IAM, "tasks:read")).Get("/api/v1/task-types", handler.listTaskTypes)
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
		apperror.WriteError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, result)
}
func (h platformHandler) submitTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DefinitionID        json.RawMessage `json:"definition_id"`
		TaskType            string          `json:"task_type"`
		Payload             json.RawMessage `json:"payload"`
		IdempotencyKey      json.RawMessage `json:"idempotency_key"`
		MaxRetries          json.RawMessage `json:"max_retries"`
		TimeoutSeconds      json.RawMessage `json:"timeout_seconds"`
		UniqueForSeconds    json.RawMessage `json:"unique_for_seconds"`
		ProcessAfterSeconds json.RawMessage `json:"process_after_seconds"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		apperror.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		apperror.WriteError(w, apperror.Validation(map[string]string{"body": "invalid JSON body"}))
		return
	}
	validationErrors := make(map[string]string)
	if in.TaskType == "" {
		validationErrors["task_type"] = "is required"
	} else if !h.deps.TaskCatalog.Contains(in.TaskType) {
		validationErrors["task_type"] = "is not a registered task type"
	}
	var payloadObject map[string]json.RawMessage
	if len(in.Payload) == 0 || json.Unmarshal(in.Payload, &payloadObject) != nil || payloadObject == nil {
		validationErrors["payload"] = "must be a JSON object"
	}
	definitionID := optionalString(in.DefinitionID, "definition_id", validationErrors)
	if definitionID != "" && uuid.Validate(definitionID) != nil {
		validationErrors["definition_id"] = "must be a UUID"
	}
	idempotencyKey := optionalString(in.IdempotencyKey, "idempotency_key", validationErrors)
	maxRetries := optionalInt(in.MaxRetries, 3, 0, taskmodule.MaxSubmissionRetries, "max_retries", validationErrors)
	timeoutSeconds := optionalInt(in.TimeoutSeconds, 300, 1, int(taskmodule.MaxSubmissionTimeout/time.Second), "timeout_seconds", validationErrors)
	uniqueForSeconds := optionalInt(in.UniqueForSeconds, 0, 0, int(taskmodule.MaxSubmissionUniqueFor/time.Second), "unique_for_seconds", validationErrors)
	processAfterSeconds := optionalInt(in.ProcessAfterSeconds, 0, 0, int(taskmodule.MaxSubmissionProcessAfter/time.Second), "process_after_seconds", validationErrors)
	if len(validationErrors) > 0 {
		apperror.WriteError(w, apperror.Validation(validationErrors))
		return
	}
	execution, err := h.deps.Tasks.Submit(r.Context(), taskmodule.Submission{
		DefinitionID: definitionID, TaskType: in.TaskType, Payload: in.Payload, IdempotencyKey: idempotencyKey,
		MaxRetries: maxRetries, Timeout: time.Duration(timeoutSeconds) * time.Second,
		UniqueFor: time.Duration(uniqueForSeconds) * time.Second, ProcessAfter: time.Duration(processAfterSeconds) * time.Second,
	})
	if h.deps.TaskObserver != nil {
		status := taskmodule.StatusQueued
		if err != nil {
			status = taskmodule.StatusFailed
		}
		h.deps.TaskObserver.ObserveTask(in.TaskType, string(status), 0)
	}
	h.audit(r.Context(), "task.submit", "task_execution", execution.ID, err)
	if err != nil {
		if errors.Is(err, taskmodule.ErrDefinitionNotFound) {
			apperror.WriteError(w, apperror.NotFound("task definition"))
			return
		}
		if errors.Is(err, taskmodule.ErrDuplicateSubmission) {
			apperror.WriteError(w, apperror.New(http.StatusConflict, "duplicate task submission", http.StatusConflict, err))
			return
		}
		if errors.Is(err, taskmodule.ErrInvalidSubmission) {
			apperror.WriteError(w, apperror.Validation(map[string]string{"body": "invalid task submission"}))
			return
		}
		apperror.WriteError(w, err)
		return
	}
	apperror.Write(w, http.StatusAccepted, execution)
}

func (h platformHandler) listExecutions(w http.ResponseWriter, r *http.Request) {
	query := taskmodule.ExecutionQuery{
		Filter: taskmodule.ExecutionFilter{TaskType: r.URL.Query().Get("task_type")},
		Page:   queryInt(r, "page", 1),
		Size:   queryInt(r, "size", pagination.DefaultSize),
	}
	if value := r.URL.Query().Get("status"); value != "" {
		status, ok := taskmodule.ParseStatus(value)
		if !ok {
			apperror.WriteError(w, apperror.Validation(map[string]string{"status": "must be one of queued, running, succeeded, failed, cancelled"}))
			return
		}
		query.Filter.Status = status
	}
	items, err := h.deps.Executions.ListExecutions(r.Context(), query)
	if err != nil {
		apperror.WriteError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, items)
}

func optionalString(raw json.RawMessage, field string, validationErrors map[string]string) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		validationErrors[field] = "must be a non-empty string"
		return ""
	}
	return value
}

func optionalInt(raw json.RawMessage, fallback, minimum, maximum int, field string, validationErrors map[string]string) int {
	if len(raw) == 0 {
		return fallback
	}
	var value int
	if string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
		validationErrors[field] = "must be an integer"
		return fallback
	}
	if value < minimum {
		validationErrors[field] = "must be greater than or equal to " + strconv.Itoa(minimum)
	} else if value > maximum {
		validationErrors[field] = "must be less than or equal to " + strconv.Itoa(maximum)
	}
	return value
}
func (h platformHandler) getExecution(w http.ResponseWriter, r *http.Request) {
	execution, err := h.deps.Executions.GetExecution(r.Context(), chi.URLParam(r, "executionID"))
	if err != nil {
		if errors.Is(err, taskmodule.ErrExecutionNotFound) {
			apperror.WriteError(w, apperror.NotFound("task execution"))
			return
		}
		apperror.WriteError(w, err)
		return
	}
	apperror.Write(w, http.StatusOK, execution)
}

func (h platformHandler) listTaskTypes(w http.ResponseWriter, _ *http.Request) {
	type taskTypeResponse struct {
		TaskType string `json:"task_type"`
	}
	items := make([]taskTypeResponse, 0, len(h.deps.TaskCatalog.Types()))
	for _, taskType := range h.deps.TaskCatalog.Types() {
		items = append(items, taskTypeResponse{TaskType: taskType})
	}
	apperror.Write(w, http.StatusOK, items)
}
func (h platformHandler) audit(ctx context.Context, action, resourceType, resourceID string, primary error) {
	if h.deps.Recorder == nil {
		return
	}
	result := audit.ResultSuccess
	if primary != nil {
		result = audit.ResultFailure
	}
	var actorID *string
	if subject := iam.Subject(ctx); subject != "" {
		actorID = &subject
	}
	metadata := audit.RequestMetadataFromContext(ctx)
	_ = h.deps.Recorder.Preserve(ctx, audit.NewEvent{
		RequestID: requestcontext.RequestIDFromContext(ctx), ActorID: actorID, Action: action, Result: result,
		ResourceType: resourceType, ResourceID: resourceID, IPAddress: metadata.IPAddress, UserAgent: metadata.UserAgent,
	}, primary)
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
