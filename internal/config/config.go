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
	Environment     string
	ServiceName     string
	HTTPAddr        string
	LogLevel        string
	DatabaseURL     string
	RedisAddr       string
	JWTSecret       string
	ShutdownTimeout time.Duration
	BreakerFailures uint32
	BreakerTimeout  time.Duration
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

	cfg := Config{
		Environment:     value("APP_ENV", "development"),
		ServiceName:     value("SERVICE_NAME", "backend-infrastructure-go"),
		HTTPAddr:        value("HTTP_ADDR", ":8080"),
		LogLevel:        strings.ToLower(value("LOG_LEVEL", "info")),
		DatabaseURL:     value("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/backend?sslmode=disable"),
		RedisAddr:       value("REDIS_ADDR", "localhost:6379"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		ShutdownTimeout: shutdownTimeout,
		BreakerFailures: breakerFailures,
		BreakerTimeout:  breakerTimeout,
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
	return nil
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
