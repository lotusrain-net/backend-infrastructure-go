// Package requestcontext is a compatibility adapter for pkg/httpkit request IDs.
package requestcontext

import (
	"context"

	"github.com/jyysy/backend-infrastructure-go/pkg/httpkit"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return httpkit.WithRequestID(ctx, requestID)
}

func RequestID(ctx context.Context) string { return httpkit.RequestIDFromContext(ctx) }
