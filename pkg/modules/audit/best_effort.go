package audit

import (
	"context"
	"log/slog"
)

type FailureObserver interface {
	AuditWriteFailed(action string)
}

type recordObserver interface {
	ObserveAudit(action, result string)
}

type BestEffortRecorder struct {
	recorder Recorder
	logger   *slog.Logger
	observer FailureObserver
}

func NewBestEffortRecorder(recorder Recorder, logger *slog.Logger, observer FailureObserver) *BestEffortRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &BestEffortRecorder{recorder: recorder, logger: logger, observer: observer}
}

// Preserve records the audit event and always returns the primary operation's outcome.
func (recorder *BestEffortRecorder) Preserve(ctx context.Context, event NewEvent, primaryErr error) error {
	if recorder == nil || recorder.recorder == nil {
		return primaryErr
	}
	if _, err := recorder.recorder.Record(ctx, event); err != nil {
		recorder.logger.ErrorContext(ctx, "audit write failed", "action", event.Action, "error", err)
		if recorder.observer != nil {
			recorder.observer.AuditWriteFailed(event.Action)
		}
	} else if observer, ok := recorder.observer.(recordObserver); ok {
		observer.ObserveAudit(event.Action, string(event.Result))
	}
	return primaryErr
}
