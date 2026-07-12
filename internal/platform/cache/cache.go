package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return errors.New("redis Addr is required")
	}
	if c.DB < 0 {
		return errors.New("redis DB must not be negative")
	}
	if c.DialTimeout < 0 {
		return errors.New("redis DialTimeout must not be negative")
	}
	if c.ReadTimeout < 0 {
		return errors.New("redis ReadTimeout must not be negative")
	}
	if c.WriteTimeout < 0 {
		return errors.New("redis WriteTimeout must not be negative")
	}
	return nil
}

type Client struct {
	client *redis.Client
}

func Open(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	client := &Client{client: rdb}
	if err := client.Health(ctx); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("redis client is nil")
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check: %w", err)
	}
	return nil
}

func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := ValidateTTL(ttl); err != nil {
		return err
	}
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set redis key: %w", err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("get redis key: %w", err)
	}
	return value, nil
}

func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("get redis key TTL: %w", err)
	}
	return ttl, nil
}

func (c *Client) Delete(ctx context.Context, keys ...string) (int64, error) {
	deleted, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("delete redis keys: %w", err)
	}
	return deleted, nil
}

func ValidateTTL(ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("redis TTL must be positive")
	}
	return nil
}

type KeyBuilder struct {
	prefix string
}

func NewKeyBuilder(namespace, environment string) (KeyBuilder, error) {
	if err := validateSegment(namespace); err != nil {
		return KeyBuilder{}, fmt.Errorf("namespace: %w", err)
	}
	if err := validateSegment(environment); err != nil {
		return KeyBuilder{}, fmt.Errorf("environment: %w", err)
	}
	return KeyBuilder{prefix: namespace + ":" + environment}, nil
}

func (b KeyBuilder) Build(segments ...string) (string, error) {
	if b.prefix == "" {
		return "", errors.New("key builder is not initialized")
	}
	if len(segments) == 0 {
		return "", errors.New("at least one key segment is required")
	}
	for _, segment := range segments {
		if err := validateSegment(segment); err != nil {
			return "", err
		}
	}
	return b.prefix + ":" + strings.Join(segments, ":"), nil
}

func validateSegment(segment string) error {
	if segment == "" || strings.TrimSpace(segment) != segment || strings.Contains(segment, ":") {
		return fmt.Errorf("unsafe key segment %q", segment)
	}
	return nil
}
