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
	ID             string
	DefinitionID   string
	TaskType       string
	QueueID        string
	IdempotencyKey string
	Payload        json.RawMessage
	Status         Status
	Attempt        int
	ProcessedRows  int64
	ErrorSummary   string
	StartedAt      *time.Time
	FinishedAt     *time.Time
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
