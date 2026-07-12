package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Component interface {
	Run(context.Context) error
	Shutdown(context.Context) error
}

func Run(parent context.Context, logger *slog.Logger, shutdownTimeout time.Duration, components ...Component) error {
	runCtx, cancel := context.WithCancel(parent)
	defer cancel()

	results := make(chan error, len(components))
	var workers sync.WaitGroup
	for _, component := range components {
		workers.Add(1)
		go func(component Component) {
			defer workers.Done()
			results <- component.Run(runCtx)
		}(component)
	}

	var runErr error
	if len(components) == 0 {
		<-parent.Done()
	} else {
		select {
		case <-parent.Done():
		case err := <-results:
			if err != nil && !errors.Is(err, context.Canceled) {
				runErr = err
				logger.Error("component stopped", "error", err)
			}
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	for index := len(components) - 1; index >= 0; index-- {
		if err := components[index].Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			runErr = errors.Join(runErr, err)
		}
	}

	stopped := make(chan struct{})
	go func() {
		workers.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
		return runErr
	case <-shutdownCtx.Done():
		return errors.Join(runErr, shutdownCtx.Err())
	}
}
