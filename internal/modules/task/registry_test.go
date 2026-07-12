package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type handlerFunc func(context.Context, json.RawMessage) (Result, error)

func (fn handlerFunc) Handle(ctx context.Context, payload json.RawMessage) (Result, error) {
	return fn(ctx, payload)
}

func TestRegistryResolvesStringTaskHandler(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	want := Result{ProcessedRows: 7}
	handler := handlerFunc(func(context.Context, json.RawMessage) (Result, error) { return want, nil })
	if err := registry.Register("report.generate", handler); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := registry.Resolve("report.generate")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	result, err := got.Handle(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result != want {
		t.Fatalf("Handle() result = %+v, want %+v", result, want)
	}
}

func TestRegistryRejectsDuplicateAndMissingHandlers(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	handler := handlerFunc(func(context.Context, json.RawMessage) (Result, error) { return Result{}, nil })
	if err := registry.Register("report.generate", handler); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := registry.Register("report.generate", handler); !errors.Is(err, ErrHandlerRegistered) {
		t.Fatalf("duplicate Register() error = %v, want handler registered", err)
	}
	if _, err := registry.Resolve("report.missing"); !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("Resolve() error = %v, want handler not found", err)
	}
}
