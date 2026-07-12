package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryStopsAfterSuccess(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 4}, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryIsBoundedByMaximumAttempts(t *testing.T) {
	want := errors.New("unavailable")
	attempts := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 3}, func(context.Context) error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("Retry() error = %v, want %v", err, want)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryStopsDuringBackoffWhenContextExpires(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	attempts := 0
	err := Retry(ctx, RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Second}, func(context.Context) error {
		attempts++
		return errors.New("temporary")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Retry() error = %v, want deadline exceeded", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryHonorsRetryPredicate(t *testing.T) {
	want := errors.New("permanent")
	attempts := 0
	err := Retry(context.Background(), RetryPolicy{
		MaxAttempts: 3,
		RetryIf:     func(error) bool { return false },
	}, func(context.Context) error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("Retry() error = %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryRejectsInvalidMaximumAttempts(t *testing.T) {
	err := Retry(context.Background(), RetryPolicy{}, func(context.Context) error { return nil })
	if !errors.Is(err, ErrInvalidRetryPolicy) {
		t.Fatalf("Retry() error = %v, want invalid retry policy", err)
	}
}
