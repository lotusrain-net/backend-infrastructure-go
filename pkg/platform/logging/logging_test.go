package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewWritesStructuredJSONAtConfiguredLevel(t *testing.T) {
	var output bytes.Buffer
	logger := New(&output, slog.LevelInfo, "test")

	logger.Debug("hidden")
	logger.Info("started", "component", "api")

	got := output.String()
	if strings.Contains(got, "hidden") {
		t.Fatalf("output contains filtered debug record: %s", got)
	}
	for _, want := range []string{`"level":"INFO"`, `"msg":"started"`, `"service":"test"`, `"component":"api"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q does not contain %q", got, want)
		}
	}
}

func TestParseLevelRejectsUnknownValue(t *testing.T) {
	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatal("ParseLevel() error = nil, want validation error")
	}
}
