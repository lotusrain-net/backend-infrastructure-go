package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/audit"
)

func TestServiceRecordsValidEvent(t *testing.T) {
	repo := &repositoryStub{created: audit.Event{ID: "audit-1"}}
	service := audit.NewService(repo)
	actorID := "8d3f4a0e-dab4-4af7-bd44-dbf3213c5b66"

	got, err := service.Record(context.Background(), audit.NewEvent{
		RequestID: "request-1", ActorID: &actorID, Action: "auth.login", Result: audit.ResultSuccess,
		ResourceType: "session", ResourceID: "session-1", Metadata: map[string]any{"method": "password"},
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if got.ID != "audit-1" || repo.received.RequestID != "request-1" {
		t.Fatalf("Record() = %#v, received = %#v", got, repo.received)
	}
}

func TestServiceRejectsIncompleteEvent(t *testing.T) {
	service := audit.NewService(&repositoryStub{})
	_, err := service.Record(context.Background(), audit.NewEvent{Result: audit.ResultSuccess})
	if !errors.Is(err, audit.ErrInvalidEvent) {
		t.Fatalf("Record() error = %v, want ErrInvalidEvent", err)
	}
}

func TestServicePropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("postgres unavailable")
	service := audit.NewService(&repositoryStub{createErr: want})
	_, err := service.Record(context.Background(), validEvent())
	if !errors.Is(err, want) {
		t.Fatalf("Record() error = %v, want %v", err, want)
	}
}

func TestServiceListsFilteredPage(t *testing.T) {
	repo := &repositoryStub{
		items: []audit.Event{{ID: "audit-2", Action: "task.create"}},
		total: 3,
	}
	service := audit.NewService(repo)
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	page, err := service.List(context.Background(), audit.Query{
		Filter: audit.Filter{Action: "task.create", Result: audit.ResultFailure, From: &from},
		Page:   2, Size: 1,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Items) != 1 || page.Meta.Total != 3 || page.Meta.Page != 2 || page.Meta.Pages != 3 {
		t.Fatalf("List() = %#v", page)
	}
	if repo.filter.Action != "task.create" || repo.limit != 1 || repo.offset != 1 {
		t.Fatalf("repository args = filter %#v limit %d offset %d", repo.filter, repo.limit, repo.offset)
	}
}

func TestServiceNormalizesPagination(t *testing.T) {
	repo := &repositoryStub{}
	service := audit.NewService(repo)
	_, err := service.List(context.Background(), audit.Query{Page: 0, Size: 1000})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.limit != 100 || repo.offset != 0 {
		t.Fatalf("repository limit=%d offset=%d", repo.limit, repo.offset)
	}
}

func validEvent() audit.NewEvent {
	return audit.NewEvent{RequestID: "request-1", Action: "auth.login", Result: audit.ResultSuccess, ResourceType: "session"}
}

type repositoryStub struct {
	created   audit.Event
	received  audit.NewEvent
	createErr error
	items     []audit.Event
	total     int64
	listErr   error
	filter    audit.Filter
	limit     int
	offset    int
}

func (repo *repositoryStub) Create(_ context.Context, event audit.NewEvent) (audit.Event, error) {
	repo.received = event
	if repo.createErr != nil {
		return audit.Event{}, repo.createErr
	}
	return repo.created, nil
}

func (repo *repositoryStub) List(_ context.Context, filter audit.Filter, limit, offset int) ([]audit.Event, int64, error) {
	repo.filter, repo.limit, repo.offset = filter, limit, offset
	return repo.items, repo.total, repo.listErr
}
