package architecture_test

import (
	"strings"
	"testing"
)

func TestModulesDoNotImportCompositionOrConcreteHTTPAdapters(t *testing.T) {
	t.Parallel()

	for _, dependency := range scanModuleImports(t) {
		for _, forbidden := range []string{
			"backend-infrastructure-go/cmd",
			"backend-infrastructure-go/internal/bootstrap",
			"backend-infrastructure-go/internal/platform/httpserver",
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
			"backend-infrastructure-go/internal/platform",
		} {
			if strings.HasPrefix(dependency.importPath, forbidden) {
				t.Errorf("%s imports forbidden infrastructure dependency %s", dependency.file, dependency.importPath)
			}
		}
	}
}
