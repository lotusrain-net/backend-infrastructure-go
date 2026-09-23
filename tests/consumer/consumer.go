// Package consumer verifies that all supported packages can be imported from an
// external module without relying on repository-internal types.
package consumer

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/apikit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/lifecycle"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/pagination"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
)

var (
	_ = apikit.Envelope[pagination.Page[string]]{}
	_ = httpkit.CORS(httpkit.CORSConfig{})
	_ = lifecycle.NewStack()
	_ = logging.New(&bytes.Buffer{}, slog.LevelInfo, "consumer")
	_ = postgres.Config{URL: "postgres://localhost/app", MaxConns: 1}
	_ http.Handler
)

type component struct{}

func (component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (component) Shutdown(context.Context) error { return nil }

func Compile(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	return lifecycle.Run(ctx, nil, time.Second, component{})
}
