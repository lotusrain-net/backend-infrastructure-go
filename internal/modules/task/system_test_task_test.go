package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestSystemTestHandlerReturnsRequestedProcessedRows(t *testing.T) {
	t.Parallel()

	handler := SystemTestHandler{}
	result, err := handler.Handle(context.Background(), json.RawMessage(`{"processed_rows":12}`))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.ProcessedRows != 12 {
		t.Fatalf("ProcessedRows = %d, want 12", result.ProcessedRows)
	}
}

func TestSystemTestHandlerCanExerciseFailurePath(t *testing.T) {
	t.Parallel()

	handler := SystemTestHandler{}
	_, err := handler.Handle(context.Background(), json.RawMessage(`{"fail":true}`))
	if !errors.Is(err, ErrSystemTestFailure) {
		t.Fatalf("Handle() error = %v, want system test failure", err)
	}
}
