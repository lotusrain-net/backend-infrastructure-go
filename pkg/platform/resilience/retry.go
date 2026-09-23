package resilience

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidRetryPolicy = errors.New("invalid retry policy")

type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	RetryIf        func(error) bool
}

func Retry(ctx context.Context, policy RetryPolicy, operation func(context.Context) error) error {
	if policy.MaxAttempts <= 0 || policy.InitialBackoff < 0 || policy.MaxBackoff < 0 {
		return ErrInvalidRetryPolicy
	}

	delay := policy.InitialBackoff
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation(ctx)
		if err == nil {
			return nil
		}
		if attempt == policy.MaxAttempts || (policy.RetryIf != nil && !policy.RetryIf(err)) {
			return err
		}
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				stopTimer(timer)
				return ctx.Err()
			case <-timer.C:
			}
		}
		delay = nextBackoff(delay, policy.MaxBackoff)
	}
	return nil
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current == 0 {
		return 0
	}
	if maximum > 0 && current >= maximum {
		return maximum
	}
	next := current * 2
	if next < current || (maximum > 0 && next > maximum) {
		return maximum
	}
	return next
}
