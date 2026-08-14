package postgres

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidateAcceptsProductionSettings(t *testing.T) {
	cfg := Config{
		URL:               "postgres://app:secret@db:5432/app?sslmode=require",
		MinConns:          2,
		MaxConns:          20,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   15 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidSettings(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "missing URL", cfg: Config{MinConns: 1, MaxConns: 2}, want: "URL"},
		{name: "wrong scheme", cfg: Config{URL: "mysql://db/app", MaxConns: 2}, want: "postgres"},
		{name: "negative minimum", cfg: Config{URL: "postgres://db/app", MinConns: -1, MaxConns: 2}, want: "MinConns"},
		{name: "zero maximum", cfg: Config{URL: "postgres://db/app"}, want: "MaxConns"},
		{name: "minimum exceeds maximum", cfg: Config{URL: "postgres://db/app", MinConns: 3, MaxConns: 2}, want: "MinConns"},
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
