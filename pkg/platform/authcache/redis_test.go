package authcache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/redis/go-redis/v9"
)

func TestCodeLifecycleAndPrivacy(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	ctx := context.Background()
	code, e := s.Issue(ctx, "Test@EXAMPLE.COM", "register", "ip")
	if e != nil || len(code) != 6 {
		t.Fatal(code, e)
	}
	for _, key := range server.Keys() {
		if strings.Contains(strings.ToLower(key), "test@example") {
			t.Fatal("raw email in key")
		}
		if typ := server.Type(key); typ == "hash" {
			v := server.HGet(key, "digest")
			if v == code {
				t.Fatal("raw code")
			}
		}
	}
	var limited *iam.RateLimitError
	if _, e = s.Issue(ctx, "test@example.com", "register", "ip"); !errors.As(e, &limited) || limited.RetryAfter < 1 {
		t.Fatal(e)
	}
	if _, e = s.Verify(ctx, "test@example.com", "login", code); !errors.Is(e, iam.ErrInvalidCode) {
		t.Fatal("cross-purpose", e)
	}
	receipt, e := s.Verify(ctx, "test@example.com", "register", code)
	if e != nil {
		t.Fatal(e)
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if s.Consume(ctx, "test@example.com", "register", receipt) == nil {
				success.Add(1)
			}
		})
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("replay", success.Load())
	}
	if _, e = s.Verify(ctx, "test@example.com", "register", code); !errors.Is(e, iam.ErrInvalidCode) {
		t.Fatal(e)
	}
	server.FastForward(time.Minute)
	code, e = s.Issue(ctx, "test@example.com", "register", "ip")
	if e != nil {
		t.Fatal(e)
	}
	for range 5 {
		_, _ = s.Verify(ctx, "test@example.com", "register", "invalid")
	}
	if _, e = s.Verify(ctx, "test@example.com", "register", code); !errors.Is(e, iam.ErrInvalidCode) {
		t.Fatal("attempt limit", e)
	}
	server.FastForward(time.Minute)
	code, _ = s.Issue(ctx, "expiry@example.com", "register", "ip")
	server.FastForward(10 * time.Minute)
	if _, e = s.Verify(ctx, "expiry@example.com", "register", code); !errors.Is(e, iam.ErrInvalidCode) {
		t.Fatal("expiry", e)
	}
}
func TestChallengeAttemptsAndConsumption(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	ctx := context.Background()
	id, e := s.Challenges().Issue(ctx, iam.Challenge{UserID: "user", Version: 3})
	if e != nil {
		t.Fatal(e)
	}
	for range 5 {
		v, e := s.Challenges().Attempt(ctx, id)
		if e != nil || v.Version != 3 {
			t.Fatal(v, e)
		}
	}
	if _, e = s.Challenges().Attempt(ctx, id); e == nil {
		t.Fatal("unlimited attempts")
	}
	id, _ = s.Challenges().Issue(ctx, iam.Challenge{UserID: "user"})
	if e = s.Challenges().Consume(ctx, id); e != nil {
		t.Fatal(e)
	}
	if e = s.Challenges().Consume(ctx, id); e == nil {
		t.Fatal("replay")
	}
}

func TestEmailAndIPLimitsAreIndependentOfAccountAndPurpose(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		purpose := "login"
		if i%2 == 0 {
			purpose = "register"
		}
		if _, e := s.Issue(ctx, "limit@example.com", purpose, "ip"); e != nil {
			t.Fatal(i, e)
		}
		server.FastForward(time.Minute)
	}
	var limited *iam.RateLimitError
	if _, e := s.Issue(ctx, "limit@example.com", "login", "another-ip"); !errors.As(e, &limited) {
		t.Fatal("email limit", e)
	}
	for i := 0; i < 50; i++ {
		if _, e := s.Issue(ctx, fmt.Sprintf("u%d@example.com", i), "login", "shared-ip"); e != nil {
			t.Fatal(i, e)
		}
	}
	if _, e := s.Issue(ctx, "last@example.com", "login", "shared-ip"); !errors.As(e, &limited) {
		t.Fatal("IP limit", e)
	}
	prod := New(client, "production", []byte(strings.Repeat("p", 32)))
	if _, e := prod.Issue(ctx, "limit@example.com", "login", "shared-ip"); e != nil {
		t.Fatal("environments share limits", e)
	}
}

func TestChallengeExpires(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	id, e := s.Challenges().Issue(context.Background(), iam.Challenge{UserID: "user"})
	if e != nil {
		t.Fatal(e)
	}
	server.FastForward(5 * time.Minute)
	if _, e = s.Challenges().Attempt(context.Background(), id); !errors.Is(e, iam.ErrInvalidCode) {
		t.Fatal("expired challenge accepted", e)
	}
}

func TestCooldownDoesNotRoundDownBeforeSixtySeconds(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	store := New(client, "test", []byte(strings.Repeat("p", 32)))
	ctx := context.Background()
	if _, e := store.Issue(ctx, "cooldown@example.com", "register", "ip"); e != nil {
		t.Fatal(e)
	}
	server.FastForward(60*time.Second - time.Millisecond)
	var limited *iam.RateLimitError
	if _, e := store.Issue(ctx, "cooldown@example.com", "register", "ip"); !errors.As(e, &limited) || limited.RetryAfter != 1 {
		t.Fatal("cooldown ended early", e)
	}
	server.FastForward(time.Millisecond)
	if _, e := store.Issue(ctx, "cooldown@example.com", "register", "ip"); e != nil {
		t.Fatal(e)
	}
}
