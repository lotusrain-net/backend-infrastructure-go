package consumer_test

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/migrations"
)

func TestEmbeddedMigrationsFromExternalModule(t *testing.T) {
	// Neither FS nor Source may depend on the consumer's working directory.
	t.Chdir(t.TempDir())
	driver, err := migrations.Source()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := driver.Close(); err != nil {
			t.Error(err)
		}
	})
	version, err := driver.First()
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{
		"identity", "audit", "tasks", "seed_rbac", "task_outbox",
		"audit_timeline_index", "user_preferences", "role_system_protection", "authentication",
	} {
		wantVersion := uint(i + 1)
		if i > 0 {
			version, err = driver.Next(version)
			if err != nil {
				t.Fatal(err)
			}
		}
		if version != wantVersion {
			t.Fatalf("migration version = %d, want %d", version, wantVersion)
		}
		for _, direction := range []struct {
			name string
			read func(uint) (io.ReadCloser, string, error)
		}{
			{"up", driver.ReadUp},
			{"down", driver.ReadDown},
		} {
			filename := fmt.Sprintf("%06d_%s.%s.sql", version, name, direction.name)
			want, err := fs.ReadFile(migrations.FS(), filename)
			if err != nil {
				t.Fatal(err)
			}
			if len(bytes.TrimSpace(want)) == 0 {
				t.Fatalf("embedded migration %s is empty", filename)
			}
			reader, identifier, err := direction.read(version)
			if err != nil {
				t.Fatalf("read %s: %v", filename, err)
			}
			got, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("read %s: %v; close: %v", filename, readErr, closeErr)
			}
			if identifier != name || !bytes.Equal(got, want) {
				t.Fatalf("migration source disagrees with embedded file %s (identifier %q)", filename, identifier)
			}
		}
	}
}
