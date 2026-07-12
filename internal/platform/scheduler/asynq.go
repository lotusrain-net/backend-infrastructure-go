package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	taskmodule "backend-infrastructure-go/internal/modules/task"

	"github.com/hibiken/asynq"
)

type PeriodicScheduler interface {
	Register(string, *asynq.Task, ...asynq.Option) (string, error)
	Unregister(string) error
}

type TaskFactory func(taskmodule.Schedule) (*asynq.Task, []asynq.Option, error)

type scheduleEntry struct {
	entryID     string
	fingerprint string
}

type AsynqRegistrar struct {
	mu        sync.Mutex
	scheduler PeriodicScheduler
	factory   TaskFactory
	entries   map[string]scheduleEntry
}

func NewAsynqRegistrar(scheduler PeriodicScheduler, factory TaskFactory) *AsynqRegistrar {
	return &AsynqRegistrar{
		scheduler: scheduler,
		factory:   factory,
		entries:   make(map[string]scheduleEntry),
	}
}

func (registrar *AsynqRegistrar) Replace(ctx context.Context, schedules []taskmodule.Schedule) error {
	registrar.mu.Lock()
	defer registrar.mu.Unlock()

	desired := make(map[string]struct{}, len(schedules))
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		desired[schedule.ID] = struct{}{}
		fingerprint, err := scheduleFingerprint(schedule)
		if err != nil {
			return err
		}
		current, exists := registrar.entries[schedule.ID]
		if exists && current.fingerprint == fingerprint {
			continue
		}
		queuedTask, options, err := registrar.factory(schedule)
		if err != nil {
			return fmt.Errorf("build schedule %s task: %w", schedule.ID, err)
		}
		entryID, err := registrar.scheduler.Register(scheduleSpec(schedule), queuedTask, options...)
		if err != nil {
			return fmt.Errorf("register schedule %s: %w", schedule.ID, err)
		}
		if exists {
			if err := registrar.scheduler.Unregister(current.entryID); err != nil {
				_ = registrar.scheduler.Unregister(entryID)
				return fmt.Errorf("unregister replaced schedule %s: %w", schedule.ID, err)
			}
		}
		registrar.entries[schedule.ID] = scheduleEntry{entryID: entryID, fingerprint: fingerprint}
	}

	for scheduleID, entry := range registrar.entries {
		if _, exists := desired[scheduleID]; exists {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := registrar.scheduler.Unregister(entry.entryID); err != nil {
			return fmt.Errorf("unregister removed schedule %s: %w", scheduleID, err)
		}
		delete(registrar.entries, scheduleID)
	}
	return nil
}

func scheduleSpec(schedule taskmodule.Schedule) string {
	if schedule.Timezone == "" {
		return schedule.CronExpression
	}
	return "CRON_TZ=" + schedule.Timezone + " " + schedule.CronExpression
}

func scheduleFingerprint(schedule taskmodule.Schedule) (string, error) {
	encoded, err := json.Marshal(struct {
		DefinitionID   string          `json:"definition_id"`
		CronExpression string          `json:"cron_expression"`
		Timezone       string          `json:"timezone"`
		Payload        json.RawMessage `json:"payload"`
		TaskType       string          `json:"task_type"`
		MaxRetries     int             `json:"max_retries"`
		Timeout        int64           `json:"timeout_nanoseconds"`
	}{
		DefinitionID:   schedule.DefinitionID,
		CronExpression: schedule.CronExpression,
		Timezone:       schedule.Timezone,
		Payload:        schedule.Payload,
		TaskType:       schedule.TaskType,
		MaxRetries:     schedule.MaxRetries,
		Timeout:        int64(schedule.Timeout),
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint schedule %s: %w", schedule.ID, err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
