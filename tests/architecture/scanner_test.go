package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type moduleImport struct {
	file       string
	importPath string
}

func scanModuleImports(t *testing.T) []moduleImport {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate architecture test source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	modulesRoot := filepath.Join(repositoryRoot, "internal", "modules")
	var dependencies []moduleImport
	err := filepath.WalkDir(modulesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			dependencies = append(dependencies, moduleImport{
				file:       filepath.ToSlash(relative),
				importPath: value,
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan module imports: %v", err)
	}
	return dependencies
}

func isAdapterFile(path string) bool {
	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, "/postgres/") ||
		strings.Contains(normalized, "/adapter/") ||
		strings.Contains(normalized, "/adapters/") {
		return true
	}
	return false
}
