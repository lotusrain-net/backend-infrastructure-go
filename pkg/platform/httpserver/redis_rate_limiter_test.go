package httpserver_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/httpserver"
)

func TestRedisRateLimiterExecutesSlidingWindowScript(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := httpserver.NewRedisRateLimiter(client, "ratelimit", func() time.Time { return now })

	for attempt := 0; attempt < 2; attempt++ {
		decision, err := limiter.Allow(context.Background(), "client", 2, time.Minute)
		if err != nil || !decision.Allowed {
			t.Fatalf("attempt %d: decision=%#v err=%v", attempt+1, decision, err)
		}
	}
	decision, err := limiter.Allow(context.Background(), "client", 2, time.Minute)
	if err != nil || decision.Allowed || decision.RetryAfter != time.Minute {
		t.Fatalf("denied decision=%#v err=%v", decision, err)
	}
	if got := client.ZCard(context.Background(), "ratelimit:client").Val(); got != 2 {
		t.Fatalf("ZCARD = %d", got)
	}

	now = now.Add(time.Minute)
	decision, err = limiter.Allow(context.Background(), "client", 2, time.Minute)
	if err != nil || !decision.Allowed {
		t.Fatalf("post-window decision=%#v err=%v", decision, err)
	}
}

func TestRedisRateLimiterIsAtomicUnderConcurrency(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), PoolSize: 20})
	t.Cleanup(func() { _ = client.Close() })
	limiter := httpserver.NewRedisRateLimiter(client, "ratelimit", func() time.Time {
		return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	})

	var allowed atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 50; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			decision, err := limiter.Allow(context.Background(), "shared", 10, time.Minute)
			if err != nil {
				t.Errorf("Allow: %v", err)
				return
			}
			if decision.Allowed {
				allowed.Add(1)
			}
		}()
	}
	wait.Wait()

	if got := allowed.Load(); got != 10 {
		t.Fatalf("allowed = %d, want 10", got)
	}
}

func TestRedisRateLimiterDoesNotCollideAcrossInstances(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	now := func() time.Time { return time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC) }
	first := httpserver.NewRedisRateLimiter(client, "ratelimit", now)
	second := httpserver.NewRedisRateLimiter(client, "ratelimit", now)

	for _, limiter := range []*httpserver.RedisRateLimiter{first, second} {
		decision, err := limiter.Allow(context.Background(), "shared", 2, time.Minute)
		if err != nil || !decision.Allowed {
			t.Fatalf("decision=%#v err=%v", decision, err)
		}
	}
	decision, err := first.Allow(context.Background(), "shared", 2, time.Minute)
	if err != nil || decision.Allowed {
		t.Fatalf("third decision=%#v err=%v", decision, err)
	}
}

func TestRedisRateLimiterPropagatesBackendFailure(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	limiter := httpserver.NewRedisRateLimiter(client, "ratelimit", time.Now)
	server.Close()

	if _, err := limiter.Allow(context.Background(), "client", 1, time.Minute); err == nil {
		t.Fatal("closed Redis must return an error")
	}
}
