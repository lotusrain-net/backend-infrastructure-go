package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsUnavailableRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := Open(ctx, Config{
		Addr:         "127.0.0.1:1",
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
	})
	if client != nil {
		_ = client.Close()
		t.Fatal("Open() returned a client for unavailable Redis")
	}
	if err == nil || !strings.Contains(err.Error(), "redis health check") {
		t.Fatalf("Open() error = %v, want redis health check error", err)
	}
}

func TestNilClientHealthReturnsError(t *testing.T) {
	var client *Client
	if err := client.Health(context.Background()); err == nil {
		t.Fatal("Health() returned nil for a nil client")
	}
}
