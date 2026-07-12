package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

type manualClock struct {
	now time.Time
}

func (c *manualClock) Now() time.Time {
	return c.now
}

func TestCircuitBreakerOpensAtFailureThreshold(t *testing.T) {
	clock := &manualClock{now: time.Unix(100, 0)}
	breaker, err := NewCircuitBreaker(2, time.Minute, WithClock(clock.Now))
	if err != nil {
		t.Fatalf("NewCircuitBreaker() error = %v", err)
	}
	want := errors.New("dependency failed")
	operation := func(context.Context) error { return want }

	_ = breaker.Execute(context.Background(), operation)
	_ = breaker.Execute(context.Background(), operation)
	if breaker.State() != StateOpen {
		t.Fatalf("State() = %s, want open", breaker.State())
	}

	called := false
	err = breaker.Execute(context.Background(), func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Execute() error = %v, want circuit open", err)
	}
	if called {
		t.Fatal("operation called while circuit was open")
	}
}

func TestCircuitBreakerClosesAfterSuccessfulHalfOpenProbe(t *testing.T) {
	clock := &manualClock{now: time.Unix(100, 0)}
	breaker, _ := NewCircuitBreaker(1, time.Minute, WithClock(clock.Now))
	_ = breaker.Execute(context.Background(), func(context.Context) error { return errors.New("failed") })
	clock.now = clock.now.Add(time.Minute)

	if err := breaker.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if breaker.State() != StateClosed {
		t.Fatalf("State() = %s, want closed", breaker.State())
	}
}

func TestCircuitBreakerReopensAfterFailedHalfOpenProbe(t *testing.T) {
	clock := &manualClock{now: time.Unix(100, 0)}
	breaker, _ := NewCircuitBreaker(1, time.Minute, WithClock(clock.Now))
	_ = breaker.Execute(context.Background(), func(context.Context) error { return errors.New("failed") })
	clock.now = clock.now.Add(time.Minute)

	_ = breaker.Execute(context.Background(), func(context.Context) error { return errors.New("still failed") })
	if breaker.State() != StateOpen {
		t.Fatalf("State() = %s, want open", breaker.State())
	}
}

func TestCircuitBreakerRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewCircuitBreaker(0, time.Second); err == nil {
		t.Fatal("NewCircuitBreaker() error = nil, want threshold validation error")
	}
	if _, err := NewCircuitBreaker(1, 0); err == nil {
		t.Fatal("NewCircuitBreaker() error = nil, want timeout validation error")
	}
}

func TestCircuitBreakerCountsDependencyTimeoutAsFailure(t *testing.T) {
	breaker, _ := NewCircuitBreaker(1, time.Minute)

	_ = breaker.Execute(context.Background(), func(context.Context) error {
		return context.DeadlineExceeded
	})

	if breaker.State() != StateOpen {
		t.Fatalf("State() = %s, want open", breaker.State())
	}
}

func TestCircuitBreakerDoesNotCountCallerCancellation(t *testing.T) {
	breaker, _ := NewCircuitBreaker(1, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())

	_ = breaker.Execute(ctx, func(context.Context) error {
		cancel()
		return ctx.Err()
	})

	if breaker.State() != StateClosed {
		t.Fatalf("State() = %s, want closed", breaker.State())
	}
}
