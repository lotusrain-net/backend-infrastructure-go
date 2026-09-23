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
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/config"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/httpkit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/lifecycle"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/logging"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/migrations"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/audit"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/task"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/pagination"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcache"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcrypto"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/cache"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/auditstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/iamstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/taskstore"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver/iamhttp"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver/requestmeta"
	platformlogging "github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/logging"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/mailer"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/observability"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/queue"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/resilience"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/scheduler"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/shared/apperror"
	sharedpagination "github.com/lotusrain-net/backend-infrastructure-go/pkg/shared/pagination"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/shared/requestcontext"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/shared/response"
)

var (
	_ = apikit.Envelope[pagination.Page[string]]{}
	_ = httpkit.CORS(httpkit.CORSConfig{})
	_ = lifecycle.NewStack()
	_ = logging.New(&bytes.Buffer{}, slog.LevelInfo, "consumer")
	_ = postgres.Config{URL: "postgres://localhost/app", MaxConns: 1}
	_ http.Handler
)

// Reference the promoted public APIs without opening infrastructure connections.
var (
	_ = config.Config{}
	_ = migrations.FS
	_ = migrations.Source
	_ = audit.NewService
	_ = iam.NewService
	_ = task.NewSubmissionService
	_ = authcache.New
	_ = authcrypto.New
	_ = cache.Config{}
	_ = database.Config{}
	_ = auditstore.New
	_ = dbgen.New
	_ = iamstore.New
	_ = taskstore.NewPostgresExecutionStore
	_ = httpserver.RouterOptions{}
	_ = iamhttp.RegisterRoutes
	_ = requestmeta.Middleware
	_ = platformlogging.New
	_ = mailer.Config{}
	_ = observability.Config{}
	_ = queue.NewAsynqPublisher
	_ = resilience.RetryPolicy{}
	_ = scheduler.NewRefresher
	_ = apperror.New
	_ = sharedpagination.Page[string]{}
	_ = requestcontext.WithRequestID
	_ = response.Envelope[string]{}
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
