package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"

	"github.com/hibiken/asynq"
)

type periodicSchedulerStub struct {
	registered   []string
	unregistered []string
}

func (scheduler *periodicSchedulerStub) Register(spec string, _ *asynq.Task, _ ...asynq.Option) (string, error) {
	scheduler.registered = append(scheduler.registered, spec)
	return "entry-" + spec, nil
}

func (scheduler *periodicSchedulerStub) Unregister(entryID string) error {
	scheduler.unregistered = append(scheduler.unregistered, entryID)
	return nil
}

func TestAsynqRegistrarRefreshesChangedAndRemovedSchedules(t *testing.T) {
	t.Parallel()

	backend := &periodicSchedulerStub{}
	factory := func(schedule taskmodule.Schedule) (*asynq.Task, []asynq.Option, error) {
		payload, err := json.Marshal(schedule)
		return asynq.NewTask("system.schedule", payload), nil, err
	}
	registrar := NewAsynqRegistrar(backend, factory)
	initial := taskmodule.Schedule{ID: "schedule-1", CronExpression: "*/5 * * * *", Timezone: "Asia/Shanghai", Enabled: true}

	if err := registrar.Replace(context.Background(), []taskmodule.Schedule{initial}); err != nil {
		t.Fatalf("first Replace() error = %v", err)
	}
	if err := registrar.Replace(context.Background(), []taskmodule.Schedule{initial}); err != nil {
		t.Fatalf("unchanged Replace() error = %v", err)
	}
	changed := initial
	changed.CronExpression = "*/10 * * * *"
	if err := registrar.Replace(context.Background(), []taskmodule.Schedule{changed}); err != nil {
		t.Fatalf("changed Replace() error = %v", err)
	}
	if err := registrar.Replace(context.Background(), nil); err != nil {
		t.Fatalf("removal Replace() error = %v", err)
	}

	if len(backend.registered) != 2 {
		t.Fatalf("register count = %d, want 2", len(backend.registered))
	}
	if backend.registered[0] != "CRON_TZ=Asia/Shanghai */5 * * * *" {
		t.Fatalf("first spec = %q", backend.registered[0])
	}
	if len(backend.unregistered) != 2 {
		t.Fatalf("unregister count = %d, want 2", len(backend.unregistered))
	}
}

func TestScheduleSpecPreservesExplicitUTC(t *testing.T) {
	t.Parallel()

	got := scheduleSpec(taskmodule.Schedule{CronExpression: "0 * * * *", Timezone: "UTC"})
	if got != "CRON_TZ=UTC 0 * * * *" {
		t.Fatalf("scheduleSpec() = %q, want explicit UTC", got)
	}
}

func TestAsynqRegistrarRefreshesWhenExecutionSettingsChange(t *testing.T) {
	t.Parallel()

	backend := &periodicSchedulerStub{}
	factory := func(schedule taskmodule.Schedule) (*asynq.Task, []asynq.Option, error) {
		return asynq.NewTask(schedule.TaskType, schedule.Payload), nil, nil
	}
	registrar := NewAsynqRegistrar(backend, factory)
	schedule := taskmodule.Schedule{
		ID: "schedule-1", DefinitionID: "definition-1", CronExpression: "0 * * * *", Timezone: "UTC",
		Payload: json.RawMessage(`{"scope":"all"}`), Enabled: true, TaskType: "report.generate", MaxRetries: 3, Timeout: time.Minute,
	}

	variants := []taskmodule.Schedule{schedule}
	changedTaskType := schedule
	changedTaskType.TaskType = "report.refresh"
	variants = append(variants, changedTaskType)
	changedRetries := changedTaskType
	changedRetries.MaxRetries = 5
	variants = append(variants, changedRetries)
	changedTimeout := changedRetries
	changedTimeout.Timeout = 2 * time.Minute
	variants = append(variants, changedTimeout)

	for _, variant := range variants {
		if err := registrar.Replace(context.Background(), []taskmodule.Schedule{variant}); err != nil {
			t.Fatalf("Replace() error = %v", err)
		}
	}
	if len(backend.registered) != len(variants) {
		t.Fatalf("register count = %d, want %d", len(backend.registered), len(variants))
	}
}
