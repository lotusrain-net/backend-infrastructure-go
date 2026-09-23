package authcache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/redis/go-redis/v9"
)

func TestAtomicEmailLoginAndReceiptCleanup(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	ctx := context.Background()
	code, err := s.Issue(ctx, "u@example.com", iam.EmailCodeLogin, "ip")
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if s.VerifyAndConsume(ctx, "u@example.com", iam.EmailCodeLogin, code) == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("concurrent login consumption", successes.Load())
	}
	if err := s.VerifyAndConsume(ctx, "u@example.com", iam.EmailCodeLogin, code); !errors.Is(err, iam.ErrInvalidCode) {
		t.Fatal(err)
	}
	code, _ = s.Issue(ctx, "u@example.com", iam.EmailCodeRegister, "ip")
	receipt, err := s.Verify(ctx, "u@example.com", iam.EmailCodeRegister, code)
	if err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	newCode, err := s.Issue(ctx, "u@example.com", iam.EmailCodeRegister, "ip")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Consume(ctx, "u@example.com", iam.EmailCodeRegister, receipt)
	if _, err := s.Verify(ctx, "u@example.com", iam.EmailCodeRegister, newCode); err != nil {
		t.Fatal("cleanup deleted replacement", err)
	}
}

func TestRedisFailuresAreUnavailableAndSanitized(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	defer client.Close()
	s := New(client, "test", []byte(strings.Repeat("p", 32)))
	server.SetError("backend diagnostic containing private@example.com 123456")
	ctx := context.Background()
	_, issue := s.Issue(ctx, "a@example.com", iam.EmailCodeLogin, "ip")
	_, verify := s.Verify(ctx, "a@example.com", iam.EmailCodeLogin, "123456")
	consume := s.Consume(ctx, "a@example.com", iam.EmailCodeLogin, iam.CodeReceipt{ID: "id"})
	atomic := s.VerifyAndConsume(ctx, "a@example.com", iam.EmailCodeLogin, "123456")
	_, challenge := s.Challenges().Issue(ctx, iam.Challenge{})
	_, attempt := s.Challenges().Attempt(ctx, strings.Repeat("a", 64))
	challengeConsume := s.Challenges().Consume(ctx, strings.Repeat("a", 64))
	for _, err := range []error{issue, verify, consume, atomic, challenge, attempt, challengeConsume} {
		if !errors.Is(err, iam.ErrAuthenticationUnavailable) || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "123456") {
			t.Fatal(err)
		}
	}
}
