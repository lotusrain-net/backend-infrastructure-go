package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type definitionStoreStub struct {
	created NewDefinition
	result  Definition
}

func (store *definitionStoreStub) CreateDefinition(_ context.Context, definition NewDefinition) (Definition, error) {
	store.created = definition
	return store.result, nil
}

type scheduleStoreStub struct {
	schedules []Schedule
}

func (store *scheduleStoreStub) ListEnabledSchedules(context.Context) ([]Schedule, error) {
	return store.schedules, nil
}

func TestDefinitionServiceCreatesReusableStringTaskDefinition(t *testing.T) {
	t.Parallel()

	store := &definitionStoreStub{result: Definition{ID: "definition-1", TaskType: "report.generate"}}
	service := NewDefinitionService(store)
	definition, err := service.Create(context.Background(), NewDefinition{
		Name:           "daily-report",
		TaskType:       "report.generate",
		DefaultPayload: json.RawMessage(`{}`),
		MaxRetries:     4,
		Timeout:        2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if definition.ID != "definition-1" || store.created.TaskType != "report.generate" {
		t.Fatalf("definition = %+v, created = %+v", definition, store.created)
	}
}

func TestScheduleServiceListsEnabledSchedules(t *testing.T) {
	t.Parallel()

	want := []Schedule{{ID: "schedule-1", CronExpression: "*/5 * * * *", Enabled: true}}
	service := NewScheduleService(&scheduleStoreStub{schedules: want})
	got, err := service.Enabled(context.Background())
	if err != nil {
		t.Fatalf("Enabled() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID {
		t.Fatalf("Enabled() = %+v, want %+v", got, want)
	}
}
