package task

import (
	"errors"
	"testing"
)

func TestValidateTransitionAcceptsLifecycleTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from Status
		to   Status
	}{
		{StatusQueued, StatusRunning},
		{StatusQueued, StatusCancelled},
		{StatusQueued, StatusFailed},
		{StatusRunning, StatusSucceeded},
		{StatusRunning, StatusFailed},
		{StatusRunning, StatusCancelled},
		{StatusRunning, StatusRunning},
	}

	for _, test := range tests {
		if err := ValidateTransition(test.from, test.to); err != nil {
			t.Errorf("ValidateTransition(%q, %q) error = %v", test.from, test.to, err)
		}
	}
}

func TestValidateTransitionRejectsInvalidAndTerminalTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from Status
		to   Status
	}{
		{StatusQueued, StatusSucceeded},
		{StatusSucceeded, StatusRunning},
		{StatusFailed, StatusRunning},
		{StatusCancelled, StatusRunning},
		{Status("unknown"), StatusRunning},
	}

	for _, test := range tests {
		err := ValidateTransition(test.from, test.to)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("ValidateTransition(%q, %q) error = %v, want invalid transition", test.from, test.to, err)
		}
	}
}
