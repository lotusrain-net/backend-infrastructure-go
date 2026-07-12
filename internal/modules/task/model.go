package task

import (
	"encoding/json"
	"fmt"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var ErrInvalidTransition = fmt.Errorf("invalid task status transition")

func ValidateTransition(from, to Status) error {
	if from == to && from == StatusRunning {
		return nil
	}
	allowed := map[Status]map[Status]struct{}{
		StatusQueued: {
			StatusRunning:   {},
			StatusFailed:    {},
			StatusCancelled: {},
		},
		StatusRunning: {
			StatusSucceeded: {},
			StatusFailed:    {},
			StatusCancelled: {},
		},
	}
	if _, ok := allowed[from][to]; !ok {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}
	return nil
}

type Result struct {
	ProcessedRows int64
}

type Definition struct {
	ID             string
	Name           string
	TaskType       string
	Description    string
	DefaultPayload json.RawMessage
	MaxRetries     int
	Timeout        time.Duration
	Active         bool
}

type Execution struct {
	ID             string          `json:"id"`
	DefinitionID   string          `json:"definition_id,omitempty"`
	TaskType       string          `json:"task_type"`
	QueueID        string          `json:"queue_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Status         Status          `json:"status"`
	Attempt        int             `json:"attempt"`
	ProcessedRows  int64           `json:"processed_rows"`
	ErrorSummary   string          `json:"error_summary,omitempty"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	FinishedAt     *time.Time      `json:"finished_at,omitempty"`
}

type Schedule struct {
	ID             string
	DefinitionID   string
	CronExpression string
	Timezone       string
	Payload        json.RawMessage
	Enabled        bool
	TaskType       string
	MaxRetries     int
	Timeout        time.Duration
}

type Message struct {
	ExecutionID string          `json:"execution_id"`
	TaskType    string          `json:"task_type"`
	Payload     json.RawMessage `json:"payload"`
}

type PublishOptions struct {
	QueueID      string
	MaxRetries   int
	Timeout      time.Duration
	UniqueFor    time.Duration
	ProcessAfter time.Duration
}
