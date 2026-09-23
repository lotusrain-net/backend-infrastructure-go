package config

import (
	"strings"
	"testing"
	"time"
)

func TestAuthenticationConfiguration(t *testing.T) {
	valid := Config{RecoveryCodeTTL: 30 * 24 * time.Hour, Environment: "production", AuthenticationKey: strings.Repeat("01", 32), AuthenticationPepper: strings.Repeat("02", 32), SMTPEnabled: true, SMTPHost: "smtp.example.com", SMTPPort: "465", SMTPUsername: "sender", SMTPPassword: "secure-password", SMTPFrom: "noreply@example.com", SMTPTLSMode: "tls", SMTPTimeout: 10 * time.Second}
	if e := valid.ValidateAuthentication(); e != nil {
		t.Fatal(e)
	}
	valid.SMTPTLSMode = "plain"
	if err := valid.ValidateAuthentication(); err != nil {
		t.Fatal("explicit production plain SMTP refused", err)
	}
	valid.SMTPTLSMode = "tls"
	for _, mutate := range []func(*Config){func(c *Config) { c.RecoveryCodeTTL = 0 }, func(c *Config) { c.SMTPEnabled = false }, func(c *Config) { c.SMTPHost = "" }, func(c *Config) { c.SMTPTLSMode = "none" }, func(c *Config) { c.AuthenticationKey = "bad" }, func(c *Config) { c.SMTPPassword = "" }, func(c *Config) { c.SMTPTimeout = 0 }} {
		c := valid
		mutate(&c)
		if c.ValidateAuthentication() == nil {
			t.Fatal("accepted unsafe authentication config")
		}
	}
	valid.Environment = "test"
	valid.SMTPEnabled = false
	if valid.ValidateAuthentication() != nil {
		t.Fatal("explicit development transport disable refused")
	}
}
