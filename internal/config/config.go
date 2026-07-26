package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const minimumSecretLength = 32

type Config struct {
	Environment              string
	ServiceName              string
	HTTPAddr                 string
	WorkerMetricsAddr        string
	LogLevel                 string
	DatabaseURL              string
	RedisAddr                string
	JWTSecret                string
	ShutdownTimeout          time.Duration
	DatabaseMinConns         int32
	DatabaseMaxConns         int32
	RedisPassword            string
	RedisDB                  int
	RedisTLS                 bool
	RedisTLSServerName       string
	RedisTLSCAFile           string
	AllowInsecureTransport   bool
	JWTIssuer                string
	AccessTokenTTL           time.Duration
	RefreshTokenTTL          time.Duration
	SecureCookies            bool
	RateLimit                int
	RateLimitWindow          time.Duration
	AsynqQueue               string
	AsynqConcurrency         int
	SchedulerRefreshInterval time.Duration
	MigrationsSource         string
	AdminEmail               string
	AdminUsername            string
	AdminPassword            string
	CORSAllowedOrigins       []string
	CORSAllowCredentials     bool
	TrustedProxyCIDRs        []netip.Prefix
}

func Load() (Config, error) {
	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	databaseMinConns, err := integer("DATABASE_MIN_CONNS", 1)
	if err != nil {
		return Config{}, err
	}
	databaseMaxConns, err := integer("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	redisDB, err := integer("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	redisTLS, err := boolean("REDIS_TLS", false)
	if err != nil {
		return Config{}, err
	}
	allowInsecureTransport, err := boolean("ALLOW_INSECURE_INTERNAL_TRANSPORT", false)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := duration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := duration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	secureCookies, err := boolean("COOKIE_SECURE", value("APP_ENV", "development") == "production")
	if err != nil {
		return Config{}, err
	}
	rateLimit, err := integer("RATE_LIMIT", 100)
	if err != nil {
		return Config{}, err
	}
	rateLimitWindow, err := duration("RATE_LIMIT_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	asynqConcurrency, err := integer("ASYNQ_CONCURRENCY", 10)
	if err != nil {
		return Config{}, err
	}
	schedulerRefreshInterval, err := duration("SCHEDULER_REFRESH_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	corsAllowCredentials, err := boolean("CORS_ALLOW_CREDENTIALS", false)
	if err != nil {
		return Config{}, err
	}
	trustedProxyCIDRs, err := prefixes("TRUSTED_PROXY_CIDRS")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:              value("APP_ENV", "development"),
		ServiceName:              value("SERVICE_NAME", "backend-infrastructure-go"),
		HTTPAddr:                 value("HTTP_ADDR", ":8080"),
		WorkerMetricsAddr:        value("WORKER_METRICS_ADDR", ":9090"),
		LogLevel:                 strings.ToLower(value("LOG_LEVEL", "info")),
		DatabaseURL:              value("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable"),
		RedisAddr:                value("REDIS_ADDR", "localhost:6379"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		ShutdownTimeout:          shutdownTimeout,
		DatabaseMinConns:         int32(databaseMinConns),
		DatabaseMaxConns:         int32(databaseMaxConns),
		RedisPassword:            os.Getenv("REDIS_PASSWORD"),
		RedisDB:                  redisDB,
		RedisTLS:                 redisTLS,
		RedisTLSServerName:       value("REDIS_TLS_SERVER_NAME", ""),
		RedisTLSCAFile:           value("REDIS_TLS_CA_FILE", ""),
		AllowInsecureTransport:   allowInsecureTransport,
		JWTIssuer:                value("JWT_ISSUER", "backend-infrastructure-go"),
		AccessTokenTTL:           accessTokenTTL,
		RefreshTokenTTL:          refreshTokenTTL,
		SecureCookies:            secureCookies,
		RateLimit:                rateLimit,
		RateLimitWindow:          rateLimitWindow,
		AsynqQueue:               value("ASYNQ_QUEUE", "default"),
		AsynqConcurrency:         asynqConcurrency,
		SchedulerRefreshInterval: schedulerRefreshInterval,
		MigrationsSource:         value("MIGRATIONS_SOURCE", "file://db/migrations"),
		AdminEmail:               strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))),
		AdminUsername:            strings.TrimSpace(os.Getenv("ADMIN_USERNAME")),
		AdminPassword:            os.Getenv("ADMIN_PASSWORD"),
		CORSAllowedOrigins:       list("CORS_ALLOWED_ORIGINS"),
		CORSAllowCredentials:     corsAllowCredentials,
		TrustedProxyCIDRs:        trustedProxyCIDRs,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) ValidateAPI() error {
	if len(c.JWTSecret) < minimumSecretLength {
		return fmt.Errorf("JWT_SECRET must contain at least %d bytes", minimumSecretLength)
	}
	if c.Environment == "production" {
		if !c.SecureCookies {
			return errors.New("COOKIE_SECURE must be true in production")
		}
		if isPlaceholderSecret(c.JWTSecret) {
			return errors.New("JWT_SECRET must not use a public placeholder in production")
		}
	}
	return nil
}

func (c Config) ValidateAdminBootstrap() error {
	if !strings.Contains(c.AdminEmail, "@") {
		return errors.New("ADMIN_EMAIL is required and must be an email")
	}
	if c.AdminUsername == "" {
		return errors.New("ADMIN_USERNAME is required")
	}
	if len(c.AdminPassword) < 12 {
		return errors.New("ADMIN_PASSWORD must contain at least 12 bytes")
	}
	if c.Environment == "production" && isPlaceholderSecret(c.AdminPassword) {
		return errors.New("ADMIN_PASSWORD must not use a public placeholder in production")
	}
	return nil
}

func (c Config) Validate() error {
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return fmt.Errorf("APP_ENV must be development, test, or production: %q", c.Environment)
	}
	if c.ServiceName == "" {
		return errors.New("SERVICE_NAME is required")
	}
	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR is required")
	}
	if c.WorkerMetricsAddr == "" {
		return errors.New("WORKER_METRICS_ADDR is required")
	}
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.Environment == "production" {
		parsed, err := url.Parse(c.DatabaseURL)
		if err != nil {
			return fmt.Errorf("DATABASE_URL is invalid: %w", err)
		}
		if parsed.User != nil {
			if password, ok := parsed.User.Password(); ok && isPlaceholderSecret(password) {
				return errors.New("DATABASE_URL must not use a public placeholder password in production")
			}
		}
		if !c.AllowInsecureTransport {
			switch strings.ToLower(parsed.Query().Get("sslmode")) {
			case "require", "verify-ca", "verify-full":
			default:
				return errors.New("DATABASE_URL must require TLS in production")
			}
			if !c.RedisTLS {
				return errors.New("REDIS_TLS must be true in production")
			}
		}
	}
	if c.RedisAddr == "" {
		return errors.New("REDIS_ADDR is required")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error: %q", c.LogLevel)
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("SHUTDOWN_TIMEOUT must be positive")
	}
	if c.DatabaseMinConns < 0 || c.DatabaseMaxConns <= 0 || c.DatabaseMinConns > c.DatabaseMaxConns {
		return errors.New("database connection limits are invalid")
	}
	if c.RedisDB < 0 {
		return errors.New("REDIS_DB must not be negative")
	}
	if c.JWTIssuer == "" || c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return errors.New("token settings are invalid")
	}
	if c.RateLimit <= 0 || c.RateLimitWindow <= 0 {
		return errors.New("rate limit settings are invalid")
	}
	if c.AsynqQueue == "" || c.AsynqConcurrency <= 0 || c.SchedulerRefreshInterval <= 0 {
		return errors.New("task runtime settings are invalid")
	}
	if c.MigrationsSource == "" {
		return errors.New("MIGRATIONS_SOURCE is required")
	}
	return nil
}

func integer(name string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}
func boolean(name string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func value(name, fallback string) string {
	if current, ok := os.LookupEnv(name); ok {
		return strings.TrimSpace(current)
	}
	return fallback
}

func isPlaceholderSecret(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "replace-with-") || strings.HasPrefix(normalized, "example-")
}

func list(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func prefixes(name string) ([]netip.Prefix, error) {
	values := list(name)
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}
