package lifecycle

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

func TestRunTreatsSpontaneousCancellationAsComponentFailure(t *testing.T) {
	component := componentFunc{
		run:      func(context.Context) error { return context.Canceled },
		shutdown: func(context.Context) error { return nil },
	}

	err := Run(context.Background(), slog.Default(), time.Second, component)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", err)
	}
}

func TestRunSuppressesCancellationCausedByParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	component := componentFunc{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		shutdown: func(context.Context) error { return nil },
	}

	cancel()
	if err := Run(ctx, slog.Default(), time.Second, component); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunWaitsForParentAfterAComponentCompletesSuccessfully(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	completed := make(chan struct{})
	peerCancelled := make(chan struct{})
	component := componentFunc{
		run: func(context.Context) error {
			close(completed)
			return nil
		},
		shutdown: func(context.Context) error { return nil },
	}
	peer := componentFunc{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			close(peerCancelled)
			return ctx.Err()
		},
		shutdown: func(context.Context) error { return nil },
	}

	done := make(chan error, 1)
	go func() { done <- Run(ctx, slog.Default(), time.Second, component, peer) }()
	<-completed
	select {
	case <-peerCancelled:
		t.Fatal("successful component completion cancelled its peer")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
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

func TestStackClosesOnceInReverseOrder(t *testing.T) {
	stack := NewStack()
	var order []int
	stack.Add(CloserFunc(func(context.Context) error { order = append(order, 1); return nil }))
	stack.Add(CloserFunc(func(context.Context) error { order = append(order, 2); return nil }))

	if err := stack.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := stack.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("close order = %v", order)
	}
}

type pointerCloser struct{}

func (*pointerCloser) Close(context.Context) error { return nil }

func TestStackIgnoresTypedNilCloser(t *testing.T) {
	stack := NewStack()
	var closer *pointerCloser
	stack.Add(closer)
	if err := stack.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsNilComponentsBeforeStartingPeers(t *testing.T) {
	started := false
	valid := componentFunc{
		run: func(context.Context) error {
			started = true
			return nil
		},
		shutdown: func(context.Context) error { return nil },
	}

	if err := Run(context.Background(), slog.Default(), time.Second, valid, nil); err == nil {
		t.Fatal("Run() error = nil")
	}
	if started {
		t.Fatal("component started before the component list was validated")
	}
}

func TestRunRejectsTypedNilComponentBeforeStartingPeers(t *testing.T) {
	started := false
	valid := componentFunc{
		run: func(context.Context) error {
			started = true
			return nil
		},
		shutdown: func(context.Context) error { return nil },
	}
	var typedNil *pointerComponent

	if err := Run(context.Background(), slog.Default(), time.Second, valid, typedNil); err == nil {
		t.Fatal("Run() error = nil")
	}
	if started {
		t.Fatal("component started before the typed nil was rejected")
	}
}

func TestRunBoundsShutdownThatIgnoresContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	component := componentFunc{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		shutdown: func(context.Context) error {
			select {}
		},
	}
	cancel()

	started := time.Now()
	err := Run(ctx, slog.Default(), 30*time.Millisecond, component)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Run() shutdown took %s, want bounded duration", elapsed)
	}
}

func TestRunStartsAllShutdownHooksInReverseOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	order := make(chan int, 2)
	component := func(id int) componentFunc {
		return componentFunc{
			run:      func(ctx context.Context) error { <-ctx.Done(); return nil },
			shutdown: func(context.Context) error { order <- id; return nil },
		}
	}
	cancel()

	if err := Run(ctx, slog.Default(), time.Second, component(1), component(2)); err != nil {
		t.Fatal(err)
	}
	got := []int{<-order, <-order}
	if got[0] != 2 || got[1] != 1 {
		t.Fatalf("shutdown order = %v", got)
	}
}

func TestRunCompletesEachShutdownHookBeforeStartingThePreviousOne(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	firstStarted := make(chan struct{})
	concurrentShutdown := errors.New("shutdown hooks overlapped")
	component := func(shutdown func(context.Context) error) componentFunc {
		return componentFunc{
			run:      func(ctx context.Context) error { <-ctx.Done(); return nil },
			shutdown: shutdown,
		}
	}
	cancel()

	err := Run(ctx, slog.Default(), time.Second,
		component(func(context.Context) error { close(firstStarted); return nil }),
		component(func(context.Context) error {
			select {
			case <-firstStarted:
				return concurrentShutdown
			case <-time.After(20 * time.Millisecond):
				return nil
			}
		}),
	)
	if errors.Is(err, concurrentShutdown) {
		t.Fatal("previous component began shutdown before the later component completed")
	}
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	default:
		t.Fatal("previous component was not shut down")
	}
}

type pointerComponent struct{}

func (*pointerComponent) Run(context.Context) error      { return nil }
func (*pointerComponent) Shutdown(context.Context) error { return nil }
