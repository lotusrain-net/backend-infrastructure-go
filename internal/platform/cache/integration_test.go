package cache

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestIntegrationTTLReadAndDelete(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	db := 0
	if raw := os.Getenv("REDIS_TEST_DB"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("REDIS_TEST_DB: %v", err)
		}
		db = parsed
	}

	ctx := t.Context()
	client, err := Open(ctx, Config{Addr: addr, Password: os.Getenv("REDIS_TEST_PASSWORD"), DB: db})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	keys, err := NewKeyBuilder("backend", "integration")
	if err != nil {
		t.Fatal(err)
	}
	key, err := keys.Build("sp02", strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = client.Delete(ctx, key) })

	if err := client.Set(ctx, key, "value", 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if got, err := client.Get(ctx, key); err != nil || got != "value" {
		t.Fatalf("Get() = %q, %v", got, err)
	}
	if ttl, err := client.TTL(ctx, key); err != nil || ttl <= 0 || ttl > 30*time.Second {
		t.Fatalf("TTL() = %s, %v", ttl, err)
	}
	if deleted, err := client.Delete(ctx, key); err != nil || deleted != 1 {
		t.Fatalf("Delete() = %d, %v", deleted, err)
	}
}
