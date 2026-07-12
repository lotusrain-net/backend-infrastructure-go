package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrHandlerRegistered = errors.New("task handler already registered")
	ErrHandlerNotFound   = errors.New("task handler not found")
)

type Handler interface {
	Handle(context.Context, json.RawMessage) (Result, error)
}

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

func (registry *Registry) Register(taskType string, handler Handler) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.handlers[taskType]; exists {
		return fmt.Errorf("%w: %s", ErrHandlerRegistered, taskType)
	}
	registry.handlers[taskType] = handler
	return nil
}

func (registry *Registry) Resolve(taskType string) (Handler, error) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	handler, exists := registry.handlers[taskType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotFound, taskType)
	}
	return handler, nil
}
