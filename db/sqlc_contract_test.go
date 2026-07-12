package db_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLCConfigurationAndQueriesExist(t *testing.T) {
	config, err := os.ReadFile("sqlc.yaml")
	if err != nil {
		t.Fatalf("read sqlc.yaml: %v", err)
	}
	for _, fragment := range []string{"version: \"2\"", "engine: postgresql", "queries: queries", "schema: migrations", "package: dbgen"} {
		if !strings.Contains(string(config), fragment) {
			t.Errorf("sqlc.yaml does not contain %q", fragment)
		}
	}

	entries, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 4 {
		t.Fatalf("query files = %d, want at least 4", len(entries))
	}
	for _, entry := range entries {
		contents, err := os.ReadFile(entry)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "-- name:") {
			t.Errorf("%s has no sqlc named query", entry)
		}
	}
}

func TestSQLCDriftGateUsesPinnedGeneratorAndTemporaryOutput(t *testing.T) {
	contents, err := os.ReadFile("verify-sqlc.ps1")
	if err != nil {
		t.Fatalf("read verify-sqlc.ps1: %v", err)
	}
	text := string(contents)
	for _, fragment := range []string{
		"github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0",
		"[System.IO.Path]::GetTempPath()",
		"Get-FileHash",
		"SQLC generated output is stale",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("verify-sqlc.ps1 does not contain %q", fragment)
		}
	}
}
