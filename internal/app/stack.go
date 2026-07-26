package app

import (
	"context"
	"errors"
	"sync"
)

type Closer interface{ Close(context.Context) error }

type resourceCloser func(context.Context) error

func (closer resourceCloser) Close(ctx context.Context) error { return closer(ctx) }

type Stack struct {
	mu      sync.Mutex
	closers []Closer
	closed  bool
}
type Factory func(context.Context) (Closer, error)

func BuildResources(ctx context.Context, factories ...Factory) (*Stack, error) {
	stack := NewStack()
	for _, factory := range factories {
		resource, err := factory(ctx)
		if err != nil {
			_ = stack.Close(context.WithoutCancel(ctx))
			return nil, err
		}
		stack.Add(resource)
	}
	return stack, nil
}

func NewStack() *Stack { return &Stack{} }

func closeBuildResources(ctx context.Context, stack *Stack, buildErr error) error {
	if stack == nil {
		return buildErr
	}
	return errors.Join(buildErr, stack.Close(context.WithoutCancel(ctx)))
}
func (s *Stack) Add(closer Closer) {
	if closer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closers = append(s.closers, closer)
	}
}
func (s *Stack) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	closers := append([]Closer(nil), s.closers...)
	s.mu.Unlock()
	var result error
	for i := len(closers) - 1; i >= 0; i-- {
		result = errors.Join(result, closers[i].Close(ctx))
	}
	return result
}
