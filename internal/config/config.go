package config

import (
	"errors"
	"fmt"
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
	LogLevel                 string
	DatabaseURL              string
	RedisAddr                string
	JWTSecret                string
	ShutdownTimeout          time.Duration
	BreakerFailures          uint32
	BreakerTimeout           time.Duration
	DatabaseMinConns         int32
	DatabaseMaxConns         int32
	RedisPassword            string
	RedisDB                  int
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
}

func Load() (Config, error) {
	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	breakerTimeout, err := duration("BREAKER_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	breakerFailures, err := unsigned("BREAKER_FAILURES", 5)
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

	cfg := Config{
		Environment:              value("APP_ENV", "development"),
		ServiceName:              value("SERVICE_NAME", "backend-infrastructure-go"),
		HTTPAddr:                 value("HTTP_ADDR", ":8080"),
		LogLevel:                 strings.ToLower(value("LOG_LEVEL", "info")),
		DatabaseURL:              value("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable"),
		RedisAddr:                value("REDIS_ADDR", "localhost:6379"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		ShutdownTimeout:          shutdownTimeout,
		BreakerFailures:          breakerFailures,
		BreakerTimeout:           breakerTimeout,
		DatabaseMinConns:         int32(databaseMinConns),
		DatabaseMaxConns:         int32(databaseMaxConns),
		RedisPassword:            os.Getenv("REDIS_PASSWORD"),
		RedisDB:                  redisDB,
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
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
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
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.RedisAddr == "" {
		return errors.New("REDIS_ADDR is required")
	}
	if len(c.JWTSecret) < minimumSecretLength {
		return fmt.Errorf("JWT_SECRET must contain at least %d bytes", minimumSecretLength)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error: %q", c.LogLevel)
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("SHUTDOWN_TIMEOUT must be positive")
	}
	if c.BreakerFailures == 0 {
		return errors.New("BREAKER_FAILURES must be positive")
	}
	if c.BreakerTimeout <= 0 {
		return errors.New("BREAKER_TIMEOUT must be positive")
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

func unsigned(name string, fallback uint32) (uint32, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return uint32(parsed), nil
}
