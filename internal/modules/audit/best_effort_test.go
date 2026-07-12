package audit_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"backend-infrastructure-go/internal/modules/audit"
)

func TestBestEffortRecorderReportsAuditFailureWithoutReplacingPrimaryOutcome(t *testing.T) {
	backendErr := errors.New("audit backend unavailable")
	primaryErr := errors.New("primary operation failed")
	inner := recorderStub{err: backendErr}
	observer := &failureObserverStub{}
	var logs bytes.Buffer
	recorder := audit.NewBestEffortRecorder(inner, slog.New(slog.NewJSONHandler(&logs, nil)), observer)

	got := recorder.Preserve(context.Background(), validEvent(), primaryErr)

	if !errors.Is(got, primaryErr) {
		t.Fatalf("Preserve() = %v, want primary error", got)
	}
	if observer.action != "auth.login" || !strings.Contains(logs.String(), "audit backend unavailable") {
		t.Fatalf("observer=%#v logs=%s", observer, logs.String())
	}
}

func TestBestEffortRecorderKeepsSuccessfulPrimaryOutcome(t *testing.T) {
	recorder := audit.NewBestEffortRecorder(recorderStub{err: errors.New("down")}, nil, nil)
	if err := recorder.Preserve(context.Background(), validEvent(), nil); err != nil {
		t.Fatalf("Preserve() = %v, want nil", err)
	}
}

func TestBestEffortRecorderObservesSuccessfulAuditRecord(t *testing.T) {
	observer := &fullObserverStub{}
	recorder := audit.NewBestEffortRecorder(recorderStub{}, nil, observer)

	_ = recorder.Preserve(context.Background(), validEvent(), nil)

	if observer.action != "auth.login" || observer.result != "success" {
		t.Fatalf("observer = %#v", observer)
	}
}

type recorderStub struct{ err error }

func (stub recorderStub) Record(context.Context, audit.NewEvent) (audit.Event, error) {
	return audit.Event{}, stub.err
}

type failureObserverStub struct{ action string }

func (observer *failureObserverStub) AuditWriteFailed(action string) { observer.action = action }

type fullObserverStub struct {
	action string
	result string
}

func (observer *fullObserverStub) AuditWriteFailed(action string) { observer.action = action }
func (observer *fullObserverStub) ObserveAudit(action, result string) {
	observer.action, observer.result = action, result
}
