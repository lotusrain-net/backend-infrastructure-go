package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type componentFunc struct {
	run      func(context.Context) error
	shutdown func(context.Context) error
}

func (c componentFunc) Run(ctx context.Context) error {
	return c.run(ctx)
}

func (c componentFunc) Shutdown(ctx context.Context) error {
	return c.shutdown(ctx)
}

func TestRunCancelsComponentAndCallsShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runCancelled := make(chan struct{})
	shutdownCalled := make(chan struct{})
	component := componentFunc{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			close(runCancelled)
			return nil
		},
		shutdown: func(context.Context) error {
			close(shutdownCalled)
			return nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, slog.Default(), time.Second, component)
	}()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
	select {
	case <-runCancelled:
	default:
		t.Fatal("component did not receive cancellation")
	}
	select {
	case <-shutdownCalled:
	default:
		t.Fatal("component shutdown hook was not called")
	}
}

func TestRunReturnsComponentFailureAndStillShutsDown(t *testing.T) {
	want := errors.New("component failed")
	shutdownCalled := false
	component := componentFunc{
		run: func(context.Context) error { return want },
		shutdown: func(context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	err := Run(context.Background(), slog.Default(), time.Second, component)
	if !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
	if !shutdownCalled {
		t.Fatal("component shutdown hook was not called")
	}
}

func TestRunBoundsShutdownByTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	component := componentFunc{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		shutdown: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	cancel()
	started := time.Now()
	err := Run(ctx, slog.Default(), 20*time.Millisecond, component)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Run() shutdown took %s, want bounded duration", elapsed)
	}
}
