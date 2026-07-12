package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerificationRunnerFailsClearlyWithoutEnvironment(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E test source")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	command := exec.Command("go", "run", "./scripts/verification")
	command.Dir = repositoryRoot
	command.Env = withoutEnvironment(os.Environ(),
		"E2E_BASE_URL", "E2E_ADMIN_EMAIL", "E2E_ADMIN_PASSWORD", "E2E_RUN",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("runner succeeded without required environment: %s", output)
	}
	for _, name := range []string{"E2E_BASE_URL", "E2E_ADMIN_EMAIL", "E2E_ADMIN_PASSWORD"} {
		if !strings.Contains(string(output), name) {
			t.Errorf("runner output %q does not identify missing %s", output, name)
		}
	}
}

func withoutEnvironment(environment []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[strings.ToUpper(name)] = struct{}{}
	}
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if _, exists := blocked[strings.ToUpper(name)]; !exists {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
