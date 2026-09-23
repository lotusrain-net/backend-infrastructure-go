package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const ScheduledDispatchTaskType = "system.schedule.dispatch"

type ScheduledSubmission struct {
	DefinitionID string          `json:"definition_id"`
	TaskType     string          `json:"task_type"`
	Payload      json.RawMessage `json:"payload"`
	MaxRetries   int             `json:"max_retries"`
	Timeout      time.Duration   `json:"timeout"`
}

type ScheduledSubmissionHandler struct{ service *SubmissionService }

func NewScheduledSubmissionHandler(service *SubmissionService) *ScheduledSubmissionHandler {
	return &ScheduledSubmissionHandler{service: service}
}
func (h *ScheduledSubmissionHandler) Handle(ctx context.Context, payload json.RawMessage) (Result, error) {
	var request ScheduledSubmission
	if err := json.Unmarshal(payload, &request); err != nil {
		return Result{}, err
	}
	if request.DefinitionID == "" || request.TaskType == "" {
		return Result{}, errors.New("scheduled submission requires definition and task type")
	}
	_, err := h.service.Submit(ctx, Submission{DefinitionID: request.DefinitionID, TaskType: request.TaskType, Payload: request.Payload, MaxRetries: request.MaxRetries, Timeout: request.Timeout})
	return Result{}, err
}
