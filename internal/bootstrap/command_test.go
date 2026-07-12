package bootstrap

import (
	"bytes"
	"context"
	"testing"
)

func TestExecuteRejectsInvalidConfiguration(t *testing.T) {
	setCommandEnvironment(t)
	t.Setenv("SERVICE_NAME", " ")

	err := Execute(context.Background(), &bytes.Buffer{}, "api")
	if err == nil {
		t.Fatal("Execute() error = nil, want configuration error")
	}
}

func TestExecuteStopsWhenContextIsCancelled(t *testing.T) {
	setCommandEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Execute(ctx, &bytes.Buffer{}, "worker"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func setCommandEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("SERVICE_NAME", "backend-infrastructure-go")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("BREAKER_FAILURES", "5")
	t.Setenv("BREAKER_TIMEOUT", "30s")
}
