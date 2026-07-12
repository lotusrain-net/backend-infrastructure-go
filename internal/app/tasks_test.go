package app

import (
	"encoding/json"
	"testing"
	"time"

	taskmodule "backend-infrastructure-go/internal/modules/task"
)

func TestBuildScheduledTaskCarriesDefinitionExecutionPolicy(t *testing.T) {
	schedule := taskmodule.Schedule{ID: "schedule-1", DefinitionID: "definition-1", TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{"processed_rows":4}`), MaxRetries: 5, Timeout: 2 * time.Minute, Enabled: true}
	queued, options, err := buildScheduledTask(schedule, "critical")
	if err != nil {
		t.Fatal(err)
	}
	if queued.Type() != taskmodule.ScheduledDispatchTaskType || len(options) == 0 {
		t.Fatalf("task=%s options=%d", queued.Type(), len(options))
	}
	var payload taskmodule.ScheduledSubmission
	if err := json.Unmarshal(queued.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.DefinitionID != schedule.DefinitionID || payload.TaskType != schedule.TaskType || payload.MaxRetries != 5 || payload.Timeout != 2*time.Minute {
		t.Fatalf("payload=%+v", payload)
	}
}
