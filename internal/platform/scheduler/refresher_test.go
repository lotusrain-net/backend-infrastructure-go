package scheduler

import (
	"context"
	"testing"

	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
)

type sourceStub struct {
	schedules []taskmodule.Schedule
	called    bool
}

func (source *sourceStub) Enabled(context.Context) ([]taskmodule.Schedule, error) {
	source.called = true
	return source.schedules, nil
}

type registrarStub struct {
	schedules []taskmodule.Schedule
}

func (registrar *registrarStub) Replace(_ context.Context, schedules []taskmodule.Schedule) error {
	registrar.schedules = schedules
	return nil
}

func TestRefresherReplacesRegistrationsFromEnabledSchedules(t *testing.T) {
	t.Parallel()

	want := []taskmodule.Schedule{{ID: "schedule-1", CronExpression: "*/5 * * * *", Enabled: true}}
	source := &sourceStub{schedules: want}
	registrar := &registrarStub{}
	refresher := NewRefresher(source, registrar)

	if err := refresher.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if !source.called || len(registrar.schedules) != 1 || registrar.schedules[0].ID != want[0].ID {
		t.Fatalf("source called=%v registrations=%+v", source.called, registrar.schedules)
	}
}
