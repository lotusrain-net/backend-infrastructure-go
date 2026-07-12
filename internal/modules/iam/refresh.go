package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type RefreshCache interface {
	Get(context.Context, string) (string, error)
	Eval(context.Context, string, []string, ...any) (any, error)
}

const replaceRefreshScript = `
local previous = redis.call("GET", KEYS[2])
if previous then
    redis.call("DEL", ARGV[1] .. previous)
end
redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[4])
redis.call("SET", KEYS[2], ARGV[3], "PX", ARGV[4])
return 1
`

const rotateRefreshScript = `
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
    return 0
end
if redis.call("GET", KEYS[3]) ~= ARGV[2] then
    return 0
end
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[4])
redis.call("SET", KEYS[3], ARGV[3], "PX", ARGV[4])
return 1
`

const consumeRefreshScript = `
local user_id = redis.call("GET", KEYS[1])
if not user_id or redis.call("GET", KEYS[2]) ~= ARGV[1] then
    return false
end
redis.call("DEL", KEYS[1], KEYS[2])
return user_id
`

const revokeRefreshScript = `
if not redis.call("GET", KEYS[1]) then
    return 0
end
redis.call("DEL", KEYS[1])
if redis.call("GET", KEYS[2]) == ARGV[1] then
    redis.call("DEL", KEYS[2])
end
return 1
`

const revokeAllRefreshScript = `
local token_hash = redis.call("GET", KEYS[1])
if not token_hash then
    return 0
end
redis.call("DEL", ARGV[1] .. token_hash, KEYS[1])
return 1
`

type RefreshStore struct {
	cache  RefreshCache
	prefix string
	ttl    time.Duration
}

func NewRefreshStore(cache RefreshCache, prefix string, ttl time.Duration) *RefreshStore {
	return &RefreshStore{cache: cache, prefix: prefix, ttl: ttl}
}

func (s *RefreshStore) Issue(ctx context.Context, userID string) (string, error) {
	raw, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	hash := s.hash(raw)
	if _, err := s.cache.Eval(ctx, replaceRefreshScript, []string{s.tokenKey(raw), s.userKey(userID)}, s.tokenPrefix(), userID, hash, s.ttl.Milliseconds()); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return raw, nil
}

func (s *RefreshStore) Lookup(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidRefreshToken
	}
	userID, err := s.cache.Get(ctx, s.tokenKey(raw))
	if err != nil || userID == "" {
		return "", ErrInvalidRefreshToken
	}
	return userID, nil
}

func (s *RefreshStore) Rotate(ctx context.Context, raw, userID string) (string, error) {
	if raw == "" || userID == "" {
		return "", ErrInvalidRefreshToken
	}
	next, err := newRefreshToken()
	if err != nil {
		return "", err
	}
	oldHash := s.hash(raw)
	nextHash := s.hash(next)
	result, err := s.cache.Eval(ctx, rotateRefreshScript, []string{s.tokenKey(raw), s.tokenKey(next), s.userKey(userID)}, userID, oldHash, nextHash, s.ttl.Milliseconds())
	if err != nil {
		return "", fmt.Errorf("rotate refresh token: %w", err)
	}
	if result != int64(1) {
		return "", ErrInvalidRefreshToken
	}
	return next, nil
}

func (s *RefreshStore) Consume(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidRefreshToken
	}
	userID, err := s.Lookup(ctx, raw)
	if err != nil {
		return "", ErrInvalidRefreshToken
	}
	result, err := s.cache.Eval(ctx, consumeRefreshScript, []string{s.tokenKey(raw), s.userKey(userID)}, s.hash(raw))
	if err != nil {
		return "", fmt.Errorf("consume refresh token: %w", err)
	}
	consumed, ok := result.(string)
	if !ok || consumed != userID {
		return "", ErrInvalidRefreshToken
	}
	return consumed, nil
}

func (s *RefreshStore) Revoke(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	userID, err := s.Lookup(ctx, raw)
	if errors.Is(err, ErrInvalidRefreshToken) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.cache.Eval(ctx, revokeRefreshScript, []string{s.tokenKey(raw), s.userKey(userID)}, s.hash(raw))
	return err
}

func (s *RefreshStore) RevokeAll(ctx context.Context, userID string) error {
	_, err := s.cache.Eval(ctx, revokeAllRefreshScript, []string{s.userKey(userID)}, s.tokenPrefix())
	return err
}

func (s *RefreshStore) tokenKey(raw string) string { return s.prefix + ":token:" + s.hash(raw) }
func (s *RefreshStore) tokenPrefix() string        { return s.prefix + ":token:" }
func (s *RefreshStore) userKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return s.prefix + ":user:" + hex.EncodeToString(sum[:])
}
func (s *RefreshStore) hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newRefreshToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}
