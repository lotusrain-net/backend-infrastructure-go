package task

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidTaskCatalog = errors.New("invalid task catalog")

type TaskRegistration struct {
	TaskType string
	Handler  Handler
}

type TaskCatalog struct {
	registrations []TaskRegistration
	types         map[string]struct{}
}

func NewTaskCatalog(registrations ...TaskRegistration) (*TaskCatalog, error) {
	catalog := &TaskCatalog{
		registrations: make([]TaskRegistration, 0, len(registrations)),
		types:         make(map[string]struct{}, len(registrations)),
	}
	for _, registration := range registrations {
		taskType := strings.TrimSpace(registration.TaskType)
		if taskType == "" || registration.Handler == nil {
			return nil, ErrInvalidTaskCatalog
		}
		if _, exists := catalog.types[taskType]; exists {
			return nil, fmt.Errorf("%w: duplicate task type %s", ErrInvalidTaskCatalog, taskType)
		}
		registration.TaskType = taskType
		catalog.registrations = append(catalog.registrations, registration)
		catalog.types[taskType] = struct{}{}
	}
	return catalog, nil
}

func (catalog *TaskCatalog) Contains(taskType string) bool {
	if catalog == nil {
		return false
	}
	_, exists := catalog.types[taskType]
	return exists
}

func (catalog *TaskCatalog) Types() []string {
	if catalog == nil {
		return nil
	}
	types := make([]string, 0, len(catalog.registrations))
	for _, registration := range catalog.registrations {
		types = append(types, registration.TaskType)
	}
	return types
}

func (catalog *TaskCatalog) RegisterHandlers(registry *Registry) error {
	if catalog == nil || registry == nil {
		return ErrInvalidTaskCatalog
	}
	for _, registration := range catalog.registrations {
		if err := registry.Register(registration.TaskType, registration.Handler); err != nil {
			return err
		}
	}
	return nil
}
