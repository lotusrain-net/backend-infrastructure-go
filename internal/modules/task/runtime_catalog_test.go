package task

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

type catalogHandlerStub struct{}

func (catalogHandlerStub) Handle(context.Context, json.RawMessage) (Result, error) {
	return Result{}, nil
}

func TestTaskCatalogDrivesValidationAndRegistryRegistration(t *testing.T) {
	catalog, err := NewTaskCatalog(TaskRegistration{TaskType: "report.generate", Handler: catalogHandlerStub{}})
	if err != nil {
		t.Fatalf("NewTaskCatalog() error = %v", err)
	}
	if !catalog.Contains("report.generate") || !slices.Equal(catalog.Types(), []string{"report.generate"}) {
		t.Fatalf("catalog types = %v", catalog.Types())
	}
	registry := NewRegistry()
	if err := catalog.RegisterHandlers(registry); err != nil {
		t.Fatalf("RegisterHandlers() error = %v", err)
	}
	if _, err := registry.Resolve("report.generate"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestTaskCatalogRejectsInvalidRegistrations(t *testing.T) {
	tests := []struct {
		name          string
		registrations []TaskRegistration
	}{
		{name: "blank type", registrations: []TaskRegistration{{Handler: catalogHandlerStub{}}}},
		{name: "nil handler", registrations: []TaskRegistration{{TaskType: "report.generate"}}},
		{name: "duplicate", registrations: []TaskRegistration{{TaskType: "report.generate", Handler: catalogHandlerStub{}}, {TaskType: "report.generate", Handler: catalogHandlerStub{}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewTaskCatalog(test.registrations...); !errors.Is(err, ErrInvalidTaskCatalog) {
				t.Fatalf("NewTaskCatalog() error = %v, want invalid catalog", err)
			}
		})
	}
}
