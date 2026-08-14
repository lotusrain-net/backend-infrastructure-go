package scheduler

import (
	"context"
	"fmt"

	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
)

type Source interface {
	Enabled(context.Context) ([]taskmodule.Schedule, error)
}

type Registrar interface {
	Replace(context.Context, []taskmodule.Schedule) error
}

type Refresher struct {
	source    Source
	registrar Registrar
}

func NewRefresher(source Source, registrar Registrar) *Refresher {
	return &Refresher{source: source, registrar: registrar}
}

func (refresher *Refresher) Refresh(ctx context.Context) error {
	schedules, err := refresher.source.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled task schedules: %w", err)
	}
	if err := refresher.registrar.Replace(ctx, schedules); err != nil {
		return fmt.Errorf("replace task schedules: %w", err)
	}
	return nil
}
