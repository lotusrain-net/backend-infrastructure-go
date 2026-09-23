package cache

import (
	"crypto/tls"
	"path/filepath"
	"testing"
)

func TestNewTLSConfigRequiresTLS12AndCarriesServerName(t *testing.T) {
	config, err := NewTLSConfig(true, "redis.example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if config.MinVersion != tls.VersionTLS12 || config.ServerName != "redis.example.com" {
		t.Fatalf("TLS config = %+v", config)
	}
}

func TestNewTLSConfigRejectsUnreadableCA(t *testing.T) {
	_, err := NewTLSConfig(true, "redis.example.com", filepath.Join(t.TempDir(), "missing.pem"))
	if err == nil {
		t.Fatal("NewTLSConfig() error = nil, want unreadable CA error")
	}
}

func TestNewTLSConfigReturnsNilWhenDisabled(t *testing.T) {
	config, err := NewTLSConfig(false, "", "")
	if err != nil || config != nil {
		t.Fatalf("config = %+v, error = %v", config, err)
	}
}
