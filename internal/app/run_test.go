package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"backend-infrastructure-go/internal/platform/logging"
)

type completedComponent struct{}

func (completedComponent) Run(context.Context) error      { return nil }
func (completedComponent) Shutdown(context.Context) error { return nil }

func TestExecuteComponentLogsLifecycle(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(&output, slog.LevelInfo, "test").With("role", "worker")

	if err := executeComponent(context.Background(), logger, time.Second, completedComponent{}); err != nil {
		t.Fatalf("executeComponent() error = %v", err)
	}
	logs := output.String()
	for _, message := range []string{"service starting", "service stopped"} {
		if !strings.Contains(logs, message) {
			t.Fatalf("logs = %q, want %q", logs, message)
		}
	}
}
