package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var requiredEnvironment = []string{
	"E2E_BASE_URL",
	"E2E_ADMIN_EMAIL",
	"E2E_ADMIN_PASSWORD",
}

func main() {
	if missing := missingEnvironment(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "E2E verification requires environment variables: %s\n", strings.Join(missing, ", "))
		os.Exit(2)
	}
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	command := exec.Command("go", "test", "./tests/e2e", "-run", "^TestComposeWorkflow$", "-count=1", "-v")
	command.Dir = repositoryRoot
	command.Env = append(os.Environ(), "E2E_RUN=1")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "start E2E verification: %v\n", err)
		os.Exit(1)
	}
}

func missingEnvironment() []string {
	missing := make([]string, 0, len(requiredEnvironment))
	for _, name := range requiredEnvironment {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func findRepositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("go.mod not found from working directory")
		}
		directory = parent
	}
}
