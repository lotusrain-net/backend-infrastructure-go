package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestAuditTimelineMigrationIndexesStableSortOrder(t *testing.T) {
	contents, err := os.ReadFile("000006_audit_timeline_index.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.Join(strings.Fields(string(contents)), " ")
	if !strings.Contains(normalized, "ON audit_logs(created_at DESC, id DESC)") {
		t.Fatalf("migration does not index audit timeline order: %s", normalized)
	}
}
