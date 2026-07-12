package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	for _, name := range []string{
		"APP_ENV",
		"SERVICE_NAME",
		"HTTP_ADDR",
		"LOG_LEVEL",
		"DATABASE_URL",
		"REDIS_ADDR",
		"SHUTDOWN_TIMEOUT",
		"BREAKER_FAILURES",
		"BREAKER_TIMEOUT",
	} {
		unsetEnvironment(t, name)
	}
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.ServiceName != "backend-infrastructure-go" {
		t.Fatalf("ServiceName = %q, want backend-infrastructure-go", cfg.ServiceName)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.WorkerMetricsAddr != ":9090" {
		t.Fatalf("WorkerMetricsAddr = %q, want :9090", cfg.WorkerMetricsAddr)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want development default", cfg.DatabaseURL)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Fatalf("RedisAddr = %q, want localhost:6379", cfg.RedisAddr)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.BreakerFailures != 5 {
		t.Fatalf("BreakerFailures = %d, want 5", cfg.BreakerFailures)
	}
	if cfg.BreakerTimeout != 30*time.Second {
		t.Fatalf("BreakerTimeout = %s, want 30s", cfg.BreakerTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("SHUTDOWN_TIMEOUT", "soon")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}

func TestLoadRequiresStrongJWTSecret(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("JWT_SECRET", "short")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AdminEmail = "admin@example.com"
	cfg.AdminUsername = "admin"
	cfg.AdminPassword = "strong-admin-password"
	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("ValidateAPI() error = nil, want secret validation error")
	}
}

func TestLoadAllowsNonAPIRolesWithoutJWTSecret(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestValidateAPIRejectsUnsafeProductionSecrets(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "replace-with-at-least-32-random-bytes")
	t.Setenv("COOKIE_SECURE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.AdminEmail = "admin@example.com"
	cfg.AdminUsername = "admin"
	cfg.AdminPassword = "replace-with-strong-admin-password"
	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("ValidateAPI() error = nil, want production placeholder rejection")
	}
}

func TestValidateAPIRequiresSecureCookiesInProduction(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("COOKIE_SECURE", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.JWTSecret = "0123456789abcdef0123456789abcdef"
	cfg.AdminEmail = "admin@example.com"
	cfg.AdminUsername = "admin"
	cfg.AdminPassword = "strong-admin-password"
	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("ValidateAPI() error = nil, want secure cookie requirement")
	}
}

func TestLoadRejectsProductionDatabasePlaceholderPassword(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://backend:replace-with-random-database-password@localhost:5432/backend?sslmode=disable")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production database placeholder rejection")
	}
}

func TestLoadRejectsUnknownLogLevel(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want log level validation error")
	}
}

func TestLoadRejectsEmptyRequiredValues(t *testing.T) {
	for _, name := range []string{"SERVICE_NAME", "HTTP_ADDR", "DATABASE_URL", "REDIS_ADDR"} {
		t.Run(name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(name, "   ")

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want %s required error", name)
			}
		})
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("APP_ENV", "staging")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want APP_ENV validation error")
	}
}

func TestLoadRejectsInvalidBreakerFailures(t *testing.T) {
	for _, value := range []string{"many", "-1", "0"} {
		t.Run(value, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv("BREAKER_FAILURES", value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for BREAKER_FAILURES=%q", value)
			}
		})
	}
}

func TestLoadRejectsInvalidBreakerTimeout(t *testing.T) {
	for _, value := range []string{"later", "0s", "-1s"} {
		t.Run(value, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv("BREAKER_TIMEOUT", value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for BREAKER_TIMEOUT=%q", value)
			}
		})
	}
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("SERVICE_NAME", "backend-infrastructure-go")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("BREAKER_FAILURES", "5")
	t.Setenv("BREAKER_TIMEOUT", "30s")
}

func TestLoadIncludesRuntimeWiringDefaults(t *testing.T) {
	setValidEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTIssuer == "" || cfg.AccessTokenTTL <= 0 || cfg.RefreshTokenTTL <= 0 {
		t.Fatalf("token config=%+v", cfg)
	}
	if cfg.DatabaseMaxConns <= 0 || cfg.AsynqConcurrency <= 0 || cfg.SchedulerRefreshInterval <= 0 {
		t.Fatalf("runtime config=%+v", cfg)
	}
}

func TestLoadRejectsMalformedRuntimeWiringSettings(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("ASYNQ_CONCURRENCY", "many")
	if _, err := Load(); err == nil {
		t.Fatal("expected malformed runtime configuration error")
	}
}

func TestLoadParsesCORSOriginsAndCredentials(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "https://app.example.com" || cfg.CORSAllowedOrigins[1] != "https://admin.example.com" {
		t.Fatalf("CORSAllowedOrigins = %#v", cfg.CORSAllowedOrigins)
	}
	if !cfg.CORSAllowCredentials {
		t.Fatal("CORSAllowCredentials = false, want true")
	}
}

func TestLoadParsesTrustedProxyCIDRs(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8, 2001:db8::/32")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 || cfg.TrustedProxyCIDRs[0].String() != "10.0.0.0/8" || cfg.TrustedProxyCIDRs[1].String() != "2001:db8::/32" {
		t.Fatalf("TrustedProxyCIDRs = %#v", cfg.TrustedProxyCIDRs)
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,not-a-cidr")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestValidateAPIRequiresStrongAdminBootstrapCredentials(t *testing.T) {
	setValidEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("expected missing admin credentials error")
	}
	cfg.AdminEmail = "admin@example.com"
	cfg.AdminUsername = "admin"
	cfg.AdminPassword = "short"
	if err := cfg.ValidateAPI(); err == nil {
		t.Fatal("expected weak admin password error")
	}
	cfg.AdminPassword = "long-admin-password"
	if err := cfg.ValidateAPI(); err != nil {
		t.Fatal(err)
	}
}

func unsetEnvironment(t *testing.T, name string) {
	t.Helper()
	value, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("Unsetenv(%q): %v", name, err)
	}
	t.Cleanup(func() {
		if existed {
			if err := os.Setenv(name, value); err != nil {
				t.Errorf("restore %q: %v", name, err)
			}
			return
		}
		if err := os.Unsetenv(name); err != nil {
			t.Errorf("clear %q: %v", name, err)
		}
	})
}
