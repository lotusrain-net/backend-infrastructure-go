package cache

import (
	"strings"
	"testing"
	"time"
)

func TestKeyBuilderBuildsStableNamespacedKeys(t *testing.T) {
	keys, err := NewKeyBuilder("backend", "prod")
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}

	got, err := keys.Build("refresh_token", "user-123", "token-456")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if want := "backend:prod:refresh_token:user-123:token-456"; got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestKeyBuilderRejectsUnsafeSegments(t *testing.T) {
	if _, err := NewKeyBuilder("backend:bad", "prod"); err == nil {
		t.Fatal("NewKeyBuilder() accepted a separator in namespace")
	}

	keys, err := NewKeyBuilder("backend", "prod")
	if err != nil {
		t.Fatal(err)
	}
	for _, segment := range []string{"", "has:colon", " leading", "trailing "} {
		if _, err := keys.Build("users", segment); err == nil {
			t.Fatalf("Build() accepted unsafe segment %q", segment)
		}
	}
}

func TestConfigValidateRejectsInvalidRedisSettings(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "missing address", cfg: Config{DB: 0}, want: "Addr"},
		{name: "negative database", cfg: Config{Addr: "redis:6379", DB: -1}, want: "DB"},
		{name: "negative dial timeout", cfg: Config{Addr: "redis:6379", DialTimeout: -time.Second}, want: "DialTimeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateTTLRequiresPositiveDuration(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second} {
		if err := ValidateTTL(ttl); err == nil {
			t.Fatalf("ValidateTTL(%s) returned nil", ttl)
		}
	}
	if err := ValidateTTL(time.Minute); err != nil {
		t.Fatalf("ValidateTTL() error = %v", err)
	}
}
