package task

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

type scheduledStore struct{ created []NewExecution }

func (s *scheduledStore) CreateExecution(_ context.Context, in NewExecution) (Execution, error) {
	s.created = append(s.created, in)
	return Execution{ID: fmt.Sprintf("execution-%d", len(s.created)), Status: StatusQueued}, nil
}
func (*scheduledStore) GetExecution(context.Context, string) (Execution, error) {
	return Execution{}, nil
}
func (*scheduledStore) ClaimExecution(context.Context, string, int, time.Time) error { return nil }
func (*scheduledStore) UpdateExecution(context.Context, ExecutionUpdate) error       { return nil }

type scheduledPublisher struct{ messages []Message }

func (p *scheduledPublisher) Publish(_ context.Context, m Message, _ PublishOptions) error {
	p.messages = append(p.messages, m)
	return nil
}

func TestScheduledSubmissionCreatesFreshExecutionOnEveryTrigger(t *testing.T) {
	store := &scheduledStore{}
	publisher := &scheduledPublisher{}
	sequence := 0
	service := NewSubmissionService(store, publisher, func() string { sequence++; return fmt.Sprintf("queue-%d", sequence) })
	handler := NewScheduledSubmissionHandler(service)
	payload, _ := json.Marshal(ScheduledSubmission{DefinitionID: "def-1", TaskType: SystemTestTaskType, Payload: json.RawMessage(`{"processed_rows":2}`), MaxRetries: 3, Timeout: time.Minute})
	for range 2 {
		if _, err := handler.Handle(context.Background(), payload); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.created) != 2 || store.created[0].QueueID == store.created[1].QueueID {
		t.Fatalf("created=%+v", store.created)
	}
	if len(publisher.messages) != 2 || publisher.messages[0].ExecutionID == publisher.messages[1].ExecutionID {
		t.Fatalf("messages=%+v", publisher.messages)
	}
}
