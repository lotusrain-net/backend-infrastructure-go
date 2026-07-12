package task

import (
	"context"
	"encoding/json"
	"time"
)

type NewDefinition struct {
	Name           string
	TaskType       string
	Description    string
	DefaultPayload json.RawMessage
	MaxRetries     int
	Timeout        time.Duration
}

type DefinitionStore interface {
	CreateDefinition(context.Context, NewDefinition) (Definition, error)
}

type DefinitionService struct {
	store DefinitionStore
}

func NewDefinitionService(store DefinitionStore) *DefinitionService {
	return &DefinitionService{store: store}
}

func (service *DefinitionService) Create(ctx context.Context, definition NewDefinition) (Definition, error) {
	return service.store.CreateDefinition(ctx, definition)
}

type ScheduleStore interface {
	ListEnabledSchedules(context.Context) ([]Schedule, error)
}

type ScheduleService struct {
	store ScheduleStore
}

func NewScheduleService(store ScheduleStore) *ScheduleService {
	return &ScheduleService{store: store}
}

func (service *ScheduleService) Enabled(ctx context.Context) ([]Schedule, error) {
	return service.store.ListEnabledSchedules(ctx)
}
