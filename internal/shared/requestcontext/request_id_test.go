package requestcontext_test

import (
	"context"
	"testing"

	"backend-infrastructure-go/internal/shared/requestcontext"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := requestcontext.WithRequestID(context.Background(), "request-42")
	if got := requestcontext.RequestID(ctx); got != "request-42" {
		t.Fatalf("request id = %q", got)
	}
}

func TestRequestIDMissingReturnsEmptyString(t *testing.T) {
	if got := requestcontext.RequestID(context.Background()); got != "" {
		t.Fatalf("request id = %q", got)
	}
}
