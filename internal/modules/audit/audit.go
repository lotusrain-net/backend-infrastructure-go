package audit

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"backend-infrastructure-go/internal/shared/pagination"
)

type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

var ErrInvalidEvent = errors.New("invalid audit event")

type RequestMetadata struct {
	IPAddress *netip.Addr
	UserAgent string
}

type NewEvent struct {
	RequestID    string
	ActorID      *string
	Action       string
	Result       Result
	ResourceType string
	ResourceID   string
	IPAddress    *netip.Addr
	UserAgent    string
	Metadata     map[string]any
}

type Event struct {
	ID           string         `json:"id"`
	RequestID    string         `json:"request_id"`
	ActorID      *string        `json:"actor_id,omitempty"`
	Action       string         `json:"action"`
	Result       Result         `json:"result"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	IPAddress    *netip.Addr    `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Filter struct {
	RequestID    string
	ActorID      string
	Action       string
	Result       Result
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
}

type Query struct {
	Filter Filter
	Page   int
	Size   int
}

type Repository interface {
	Create(context.Context, NewEvent) (Event, error)
	List(context.Context, Filter, int, int) ([]Event, int64, error)
}

type Recorder interface {
	Record(context.Context, NewEvent) (Event, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) Record(ctx context.Context, event NewEvent) (Event, error) {
	if service == nil || service.repository == nil {
		return Event{}, fmt.Errorf("%w: repository is required", ErrInvalidEvent)
	}
	if err := validate(event); err != nil {
		return Event{}, err
	}
	return service.repository.Create(ctx, event)
}

func (service *Service) List(ctx context.Context, query Query) (pagination.Page[Event], error) {
	page, size := pagination.Normalize(query.Page, query.Size)
	if service == nil || service.repository == nil {
		return pagination.Page[Event]{}, fmt.Errorf("audit repository is required")
	}
	items, total, err := service.repository.List(ctx, query.Filter, size, (page-1)*size)
	if err != nil {
		return pagination.Page[Event]{}, err
	}
	return pagination.New(items, page, size, total), nil
}

func validate(event NewEvent) error {
	if strings.TrimSpace(event.RequestID) == "" || strings.TrimSpace(event.Action) == "" || strings.TrimSpace(event.ResourceType) == "" {
		return fmt.Errorf("%w: request ID, action, and resource type are required", ErrInvalidEvent)
	}
	if event.Result != ResultSuccess && event.Result != ResultFailure {
		return fmt.Errorf("%w: result must be success or failure", ErrInvalidEvent)
	}
	return nil
}
