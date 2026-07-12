package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count >= limit then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local retry_after = window
  if oldest[2] then
    retry_after = math.max(1, window - (now - tonumber(oldest[2])))
  end
  redis.call('PEXPIRE', key, window)
  return {0, 0, retry_after}
end

redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, window)
return {1, limit - count - 1, 0}
`)

type RedisRateLimiter struct {
	client   redis.Scripter
	prefix   string
	now      func() time.Time
	sequence atomic.Uint64
	nonce    string
}

var fallbackLimiterNonce atomic.Uint64

func NewRedisRateLimiter(client redis.Scripter, prefix string, now func() time.Time) *RedisRateLimiter {
	if prefix == "" {
		prefix = "rate_limit"
	}
	if now == nil {
		now = time.Now
	}
	return &RedisRateLimiter{client: client, prefix: prefix, now: now, nonce: newLimiterNonce()}
}

func (limiter *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error) {
	if limiter == nil || limiter.client == nil {
		return RateLimitDecision{}, fmt.Errorf("redis rate limiter client is required")
	}
	if limit <= 0 || window <= 0 {
		return RateLimitDecision{}, fmt.Errorf("limit and window must be positive")
	}
	nowMillis := limiter.now().UnixMilli()
	windowMillis := window.Milliseconds()
	member := limiter.nonce + "-" + strconv.FormatInt(nowMillis, 10) + "-" + strconv.FormatUint(limiter.sequence.Add(1), 10)
	result, err := slidingWindowScript.Run(
		ctx,
		limiter.client,
		[]string{limiter.prefix + ":" + key},
		nowMillis,
		windowMillis,
		limit,
		member,
	).Slice()
	if err != nil {
		return RateLimitDecision{}, fmt.Errorf("execute rate limit script: %w", err)
	}
	if len(result) != 3 {
		return RateLimitDecision{}, fmt.Errorf("unexpected rate limit script result length %d", len(result))
	}
	allowed, err := redisInt(result[0])
	if err != nil {
		return RateLimitDecision{}, err
	}
	remaining, err := redisInt(result[1])
	if err != nil {
		return RateLimitDecision{}, err
	}
	retryMillis, err := redisInt(result[2])
	if err != nil {
		return RateLimitDecision{}, err
	}
	return RateLimitDecision{
		Allowed:    allowed == 1,
		Remaining:  int(remaining),
		RetryAfter: time.Duration(retryMillis) * time.Millisecond,
	}, nil
}

func newLimiterNonce() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return "fallback-" + strconv.FormatUint(fallbackLimiterNonce.Add(1), 10)
}

func redisInt(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse Redis integer %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}
