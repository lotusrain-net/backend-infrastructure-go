package migrations_test

import (
	"io/fs"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/migrations"
)

func TestEmbeddedMigrationSetIsComplete(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		present[entry.Name()] = true
	}
	for _, name := range []string{
		"000001_identity.up.sql", "000001_identity.down.sql",
		"000002_audit.up.sql", "000002_audit.down.sql",
		"000003_tasks.up.sql", "000003_tasks.down.sql",
		"000004_seed_rbac.up.sql", "000004_seed_rbac.down.sql",
		"000005_task_outbox.up.sql", "000005_task_outbox.down.sql",
		"000006_audit_timeline_index.up.sql", "000006_audit_timeline_index.down.sql",
		"000007_user_preferences.up.sql", "000007_user_preferences.down.sql",
		"000008_role_system_protection.up.sql", "000008_role_system_protection.down.sql",
		"000009_authentication.up.sql", "000009_authentication.down.sql",
	} {
		if !present[name] {
			t.Errorf("embedded migrations missing %s", name)
		}
	}
}
