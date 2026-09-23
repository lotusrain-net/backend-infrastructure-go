package task

import (
	"context"
	"encoding/json"
	"errors"
)

const SystemTestTaskType = "system.test"

var ErrSystemTestFailure = errors.New("system test task failed")

type SystemTestHandler struct{}

func (SystemTestHandler) Handle(ctx context.Context, payload json.RawMessage) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var request struct {
		ProcessedRows int64 `json:"processed_rows"`
		Fail          bool  `json:"fail"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return Result{}, err
	}
	if request.Fail {
		return Result{}, ErrSystemTestFailure
	}
	return Result{ProcessedRows: request.ProcessedRows}, nil
}
