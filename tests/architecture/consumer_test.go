package architecture_test

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExternalConsumerCoversAllPublicPackages(t *testing.T) {
	imports := make(map[string]bool)
	for _, dependency := range scanPackageImports(t, "tests/consumer") {
		imports[dependency.importPath] = true
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate architecture test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	packages := make(map[string]bool)
	err := filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		packages["github.com/lotusrain-net/backend-infrastructure-go/"+filepath.ToSlash(relative)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) == 0 {
		t.Fatal("no public packages found")
	}
	for name := range packages {
		if !imports[name] {
			t.Errorf("external consumer does not import public package %s", name)
		}
	}
}
