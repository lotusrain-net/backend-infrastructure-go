package migrations_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationSetIsCompleteAndReversible(t *testing.T) {
	wants := map[string][]string{
		"000001_identity.up.sql":      {"CREATE TABLE users", "CREATE TABLE roles", "CREATE TABLE permissions", "CREATE TABLE user_roles", "CREATE TABLE role_permissions"},
		"000001_identity.down.sql":    {"DROP TABLE IF EXISTS role_permissions", "DROP TABLE IF EXISTS users"},
		"000002_audit.up.sql":         {"CREATE TABLE audit_logs"},
		"000002_audit.down.sql":       {"DROP TABLE IF EXISTS audit_logs"},
		"000003_tasks.up.sql":         {"CREATE TABLE task_definitions", "CREATE TABLE task_executions", "CREATE TABLE task_schedules"},
		"000003_tasks.down.sql":       {"DROP TABLE IF EXISTS task_schedules", "DROP TABLE IF EXISTS task_definitions"},
		"000004_seed_rbac.up.sql":     {"ON CONFLICT", "admin", "user"},
		"000004_seed_rbac.down.sql":   {"DELETE FROM permissions", "DELETE FROM roles"},
		"000005_task_outbox.up.sql":   {"CREATE TABLE task_outbox_messages", "published_at", "task_outbox_pending_idx"},
		"000005_task_outbox.down.sql": {"DROP TABLE IF EXISTS task_outbox_messages"},
	}

	for name, fragments := range wants {
		contents, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		text := string(contents)
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s does not contain %q", name, fragment)
			}
		}
	}
}

func TestSeedDownExplicitlyRemovesEverySeedPermissionReference(t *testing.T) {
	contents, err := os.ReadFile("000004_seed_rbac.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(contents), "\r\n", "\n")
	referenceDelete := "DELETE FROM role_permissions\nWHERE permission_id IN"
	if !strings.Contains(text, referenceDelete) {
		t.Fatalf("seed down must explicitly remove all permission references before deleting seed permissions")
	}
	if strings.Index(text, referenceDelete) > strings.Index(text, "DELETE FROM permissions") {
		t.Fatal("role permission references must be deleted before seed permissions")
	}
}
