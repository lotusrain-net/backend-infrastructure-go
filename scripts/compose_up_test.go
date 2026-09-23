package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestComposeUpInitializesTheDevelopmentProfileAndWaitsForHealth(t *testing.T) {
	script, err := os.ReadFile("compose-up.sh")
	if err != nil {
		t.Fatalf("read compose-up.sh: %v", err)
	}

	contents := string(script)
	for _, snippet := range []string{
		"set -eu",
		".env.example",
		"openssl rand -hex",
		"docker compose --env-file \"$env_file\" -f \"$compose_file\" config",
		"docker compose --env-file \"$env_file\" -f \"$compose_file\" up -d --build --wait",
	} {
		if !strings.Contains(contents, snippet) {
			t.Errorf("compose-up.sh must contain %q", snippet)
		}
	}
}

func TestComposeUpGeneratesIndependentAuthenticationSecrets(t *testing.T) {
	raw, err := os.ReadFile("compose-up.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"authentication_key=$(generate_secret 32)", "authentication_pepper=$(generate_secret 32)", "AUTHENTICATION_KEY=$authentication_key", "AUTHENTICATION_PEPPER=$authentication_pepper"} {
		if !strings.Contains(string(raw), required) {
			t.Errorf("missing %s", required)
		}
	}
}
