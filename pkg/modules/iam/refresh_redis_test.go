package iam

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type redisRefreshTestCache struct{ client *redis.Client }

func (c redisRefreshTestCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}
func (c redisRefreshTestCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return c.client.Eval(ctx, script, keys, args...).Result()
}

func TestRedisRefreshScriptsKeepTokenAndUserIndexConsistent(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRefreshStore(redisRefreshTestCache{client: client}, "iam:test", time.Hour)
	ctx := context.Background()

	first, err := store.Issue(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Issue(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(ctx, first); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replaced token error = %v", err)
	}
	if keys := server.Keys(); len(keys) != 2 {
		t.Fatalf("Redis keys after replace = %v, want one token and one user index", keys)
	}

	userID, err := store.Lookup(ctx, second)
	if err != nil || userID != "user-1" {
		t.Fatalf("Lookup() = %q, %v", userID, err)
	}
	rotated, err := store.Rotate(ctx, second, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(ctx, second); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("rotated token replay error = %v", err)
	}
	if err := store.RevokeAll(ctx, "user-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(ctx, rotated); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("revoked token error = %v", err)
	}
	if keys := server.Keys(); len(keys) != 0 {
		t.Fatalf("Redis keys after revoke-all = %v, want none", keys)
	}
}
