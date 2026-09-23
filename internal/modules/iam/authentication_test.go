package iam

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthenticationDefaultsAndPrivateBootstrap(t *testing.T) {
	s := DefaultAuthenticationSettings()
	if !s.PasswordLoginEnabled || s.RegistrationEnabled || !s.RegistrationEmailVerificationRequired || len(s.AllowedEmailDomains) != 0 {
		t.Fatalf("unsafe defaults: %+v", s)
	}
	state := AuthenticationState{AuthenticationSettings: s, BootstrapAdminUserID: "secret-id"}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-id") || strings.Contains(string(b), "initialized") {
		t.Fatal("private initialization state exposed")
	}
}

func TestAuthenticationDomainValidation(t *testing.T) {
	for _, domain := range []string{"@example.com", "https://example.com", "example.com/path", "a b.com", "*.example.com"} {
		s := DefaultAuthenticationSettings()
		s.AllowedEmailDomains = []string{domain}
		if s.Validate() == nil {
			t.Errorf("accepted invalid domain %q", domain)
		}
	}
}
