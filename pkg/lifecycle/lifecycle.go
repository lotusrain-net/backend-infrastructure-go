package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"time"
)

// Component is a long-running service with a coordinated shutdown hook.
type Component interface {
	// Run serves until the context is canceled or the component stops.
	Run(context.Context) error
	// Shutdown releases component resources within the supplied bound.
	Shutdown(context.Context) error
}

// Run starts all components, cancels peers on the first error, shuts components
// down in reverse order, and bounds both shutdown hooks and worker completion.
func Run(parent context.Context, logger *slog.Logger, shutdownTimeout time.Duration, components ...Component) error {
	if parent == nil {
		parent = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if shutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be positive")
	}
	for _, component := range components {
		if nilInterface(component) {
			return errors.New("lifecycle component is nil")
		}
	}
	runCtx, cancel := context.WithCancel(parent)
	defer cancel()

	results := make(chan error, len(components))
	var workers sync.WaitGroup
	for _, component := range components {
		workers.Add(1)
		go func(component Component) {
			defer workers.Done()
			results <- component.Run(runCtx)
		}(component)
	}

	var runErr error
	if len(components) == 0 {
		<-parent.Done()
	} else {
		remaining := len(components)
		for remaining > 0 {
			select {
			case <-parent.Done():
				remaining = 0
			case err := <-results:
				remaining--
				if err != nil && !(errors.Is(err, context.Canceled) && runCtx.Err() != nil) {
					runErr = err
					logger.Error("component stopped", "error", err)
					remaining = 0
				}
			}
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	var shutdownErr error
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		shutdownResult := make(chan error, 1)
		go func() {
			shutdownResult <- component.Shutdown(shutdownCtx)
		}()
		select {
		case err := <-shutdownResult:
			if err != nil && !errors.Is(err, context.Canceled) {
				shutdownErr = errors.Join(shutdownErr, err)
			}
		case <-shutdownCtx.Done():
			return errors.Join(runErr, shutdownErr, shutdownCtx.Err())
		}
	}

	stopped := make(chan struct{})
	go func() {
		workers.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
		return errors.Join(runErr, shutdownErr)
	case <-shutdownCtx.Done():
		return errors.Join(runErr, shutdownErr, shutdownCtx.Err())
	}
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// Closer releases a resource using the caller's shutdown context.
type Closer interface {
	// Close releases the owned resource within the supplied context.
	Close(context.Context) error
}

// CloserFunc adapts a function to Closer.
type CloserFunc func(context.Context) error

// Close invokes the wrapped function. A nil function is a no-op.
func (closer CloserFunc) Close(ctx context.Context) error {
	if closer == nil {
		return nil
	}
	return closer(ctx)
}

// Stack owns closers and releases them once in reverse registration order.
type Stack struct {
	mu      sync.Mutex
	closers []Closer
	closed  bool
}

// NewStack returns an empty resource stack.
func NewStack() *Stack { return &Stack{} }

// Add registers closer unless the stack has already closed.
func (stack *Stack) Add(closer Closer) {
	if stack == nil || nilInterface(closer) {
		return
	}
	stack.mu.Lock()
	defer stack.mu.Unlock()
	if !stack.closed {
		stack.closers = append(stack.closers, closer)
	}
}

// Close releases registered resources once in reverse order and joins errors.
func (stack *Stack) Close(ctx context.Context) error {
	if stack == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stack.mu.Lock()
	if stack.closed {
		stack.mu.Unlock()
		return nil
	}
	stack.closed = true
	closers := append([]Closer(nil), stack.closers...)
	stack.mu.Unlock()

	var result error
	for index := len(closers) - 1; index >= 0; index-- {
		result = errors.Join(result, closers[index].Close(ctx))
	}
	return result
}
