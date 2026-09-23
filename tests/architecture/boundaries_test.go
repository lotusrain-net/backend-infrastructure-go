package architecture_test

import (
	"strings"
	"testing"
)

func TestModulesDoNotImportCompositionOrConcreteHTTPAdapters(t *testing.T) {
	t.Parallel()

	for _, dependency := range scanModuleImports(t) {
		for _, forbidden := range []string{
			"github.com/lotusrain-net/backend-infrastructure-go/cmd",
			"github.com/lotusrain-net/backend-infrastructure-go/internal/bootstrap",
			"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver",
		} {
			if strings.HasPrefix(dependency.importPath, forbidden) {
				t.Errorf("%s imports forbidden composition dependency %s", dependency.file, dependency.importPath)
			}
		}
	}
}

func TestDomainFilesDoNotImportInfrastructureSDKs(t *testing.T) {
	t.Parallel()

	for _, dependency := range scanModuleImports(t) {
		for _, forbidden := range []string{
			"net/http",
			"github.com/go-chi/chi",
			"github.com/jackc/pgx",
			"github.com/redis/go-redis",
			"github.com/hibiken/asynq",
			"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform",
		} {
			if strings.HasPrefix(dependency.importPath, forbidden) {
				t.Errorf("%s imports forbidden infrastructure dependency %s", dependency.file, dependency.importPath)
			}
		}
	}
}

func TestPublicPackagesDoNotImportInternalCode(t *testing.T) {
	t.Parallel()

	forbiddenPrefixes := []string{
		"github.com/lotusrain-net/backend-infrastructure-go/internal/",
		"github.com/lotusrain-net/backend-infrastructure-go/cmd/",
		"github.com/lotusrain-net/backend-infrastructure-go/db/",
		"github.com/lotusrain-net/backend-infrastructure-go/scripts/",
		"github.com/lotusrain-net/backend-infrastructure-go/tests/",
	}
	for _, dependency := range scanPackageImports(t, "pkg") {
		for _, forbidden := range forbiddenPrefixes {
			if strings.HasPrefix(dependency.importPath, forbidden) {
				t.Errorf("%s imports private dependency %s", dependency.file, dependency.importPath)
			}
		}
	}
}
