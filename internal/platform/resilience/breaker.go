package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type BreakerState uint8

const (
	StateClosed BreakerState = iota
	StateOpen
	StateHalfOpen
)

func (state BreakerState) String() string {
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type BreakerOption func(*CircuitBreaker)

func WithClock(now func() time.Time) BreakerOption {
	return func(breaker *CircuitBreaker) {
		breaker.now = now
	}
}

type CircuitBreaker struct {
	mu               sync.Mutex
	failureThreshold uint32
	openTimeout      time.Duration
	now              func() time.Time
	state            BreakerState
	failures         uint32
	openedAt         time.Time
	probeInFlight    bool
}

func NewCircuitBreaker(failureThreshold uint32, openTimeout time.Duration, options ...BreakerOption) (*CircuitBreaker, error) {
	if failureThreshold == 0 {
		return nil, errors.New("failure threshold must be positive")
	}
	if openTimeout <= 0 {
		return nil, errors.New("open timeout must be positive")
	}
	breaker := &CircuitBreaker{
		failureThreshold: failureThreshold,
		openTimeout:      openTimeout,
		now:              time.Now,
		state:            StateClosed,
	}
	for _, option := range options {
		option(breaker)
	}
	if breaker.now == nil {
		return nil, errors.New("clock is required")
	}
	return breaker, nil
}

func (breaker *CircuitBreaker) State() BreakerState {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	breaker.advanceState()
	return breaker.state
}

func (breaker *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := breaker.acquire(); err != nil {
		return err
	}

	err := operation(ctx)
	breaker.record(ctx, err)
	return err
}

func (breaker *CircuitBreaker) acquire() error {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	breaker.advanceState()
	if breaker.state == StateOpen {
		return ErrCircuitOpen
	}
	if breaker.state == StateHalfOpen {
		if breaker.probeInFlight {
			return ErrCircuitOpen
		}
		breaker.probeInFlight = true
	}
	return nil
}

func (breaker *CircuitBreaker) record(ctx context.Context, err error) {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	if breaker.state == StateHalfOpen {
		breaker.probeInFlight = false
		if err == nil {
			breaker.state = StateClosed
			breaker.failures = 0
			return
		}
		breaker.open()
		return
	}
	if err == nil {
		breaker.failures = 0
		return
	}
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return
	}
	breaker.failures++
	if breaker.failures >= breaker.failureThreshold {
		breaker.open()
	}
}

func (breaker *CircuitBreaker) advanceState() {
	if breaker.state == StateOpen && breaker.now().Sub(breaker.openedAt) >= breaker.openTimeout {
		breaker.state = StateHalfOpen
		breaker.probeInFlight = false
	}
}

func (breaker *CircuitBreaker) open() {
	breaker.state = StateOpen
	breaker.failures = 0
	breaker.openedAt = breaker.now()
	breaker.probeInFlight = false
}

func (breaker *CircuitBreaker) String() string {
	return fmt.Sprintf("circuit-breaker(%s)", breaker.State())
}
