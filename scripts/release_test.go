package scripts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNPMBootstrapOnlyRequiresFirstPackagePublication(t *testing.T) {
	for _, tool := range []string{"bash", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required to execute the release workflow script", tool)
		}
	}
	raw, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, step := range workflow.Jobs["bootstrap"].Steps {
		if step.ID == "npm" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("release workflow is missing the npm bootstrap detection script")
	}

	for _, tc := range []struct {
		name          string
		packageStatus string
		versionStatus string
		curlExit      string
		want          string
		wantError     bool
	}{
		{"new package", "404", "404", "0", "required=true\n", false},
		{"existing package with unpublished version", "200", "404", "0", "required=false\n", false},
		{"retry published version", "200", "200", "0", "required=false\n", false},
		{"registry unavailable", "503", "503", "0", "", true},
		{"network failure", "000", "000", "7", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			artifacts := filepath.Join(root, "release-artifacts")
			if err := os.Mkdir(artifacts, 0o700); err != nil {
				t.Fatal(err)
			}
			payload := "verified tarball fixture\n"
			for name, content := range map[string]string{
				"SOURCE_COMMIT":   "test-sha\n",
				"PACKAGE_NAME":    "@lotusrain-net/backend-infrastructure-web\n",
				"PACKAGE_VERSION": "0.2.1\n",
				"package.tgz":     payload,
				"SHA256SUMS":      fmt.Sprintf("%x  package.tgz\n", sha256.Sum256([]byte(payload))),
			} {
				if err := os.WriteFile(filepath.Join(artifacts, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			outputFile := filepath.Join(root, "output")
			// Model registry state without network access. An existing package
			// and an unpublished version deliberately have different statuses.
			const registry = `
curl() {
  case "${@: -1}" in
    https://registry.npmjs.org/@lotusrain-net%2Fbackend-infrastructure-web)
      printf '%s' "$TEST_PACKAGE_STATUS" ;;
    https://registry.npmjs.org/@lotusrain-net%2Fbackend-infrastructure-web/0.2.1)
      printf '%s' "$TEST_VERSION_STATUS" ;;
    *) return 99 ;;
  esac
  return "$TEST_CURL_EXIT"
}
`
			cmd := exec.Command("bash", "-c", registry+script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"RELEASE_SHA=test-sha", "GITHUB_OUTPUT="+outputFile,
				"TEST_PACKAGE_STATUS="+tc.packageStatus,
				"TEST_VERSION_STATUS="+tc.versionStatus,
				"TEST_CURL_EXIT="+tc.curlExit,
			)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantError {
				t.Fatalf("bootstrap error = %v, wantError = %v\n%s", err, tc.wantError, output)
			}
			got, err := os.ReadFile(outputFile)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("bootstrap output = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReleasePublishPassesExplicitTarballPath guards the OIDC publish step
// against a regression where the tarball is handed to npm as a bare relative
// path. npm reads a value like "release-artifacts/pkg.tgz" as a GitHub
// shorthand (owner/repo) and shells out to git, failing before it ever reaches
// the registry.
func TestReleasePublishPassesExplicitTarballPath(t *testing.T) {
	raw, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	const stepName = "Publish verified npm tarball with OIDC"
	var script string
	for _, step := range workflow.Jobs["publish"].Steps {
		if step.Name == stepName {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatalf("release workflow is missing the %q step", stepName)
	}
	// Allow an explicitly relative (./) or absolute path, but never a bare
	// relative one that npm could mistake for a git spec.
	explicit := regexp.MustCompile(`(?m)^\s*TARBALL="(?:\./|\$PWD/|\$\{PWD\}/|/)`)
	if !explicit.MatchString(script) {
		t.Fatalf("publish step must pass npm an explicitly-qualified tarball path:\n%s", script)
	}
}

