package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

type RefreshCache interface {
	Set(context.Context, string, any, time.Duration) error
	Get(context.Context, string) (string, error)
	Delete(context.Context, ...string) (int64, error)
}

type RefreshStore struct {
	cache  RefreshCache
	prefix string
	ttl    time.Duration
}

func NewRefreshStore(cache RefreshCache, prefix string, ttl time.Duration) *RefreshStore {
	return &RefreshStore{cache: cache, prefix: prefix, ttl: ttl}
}

func (s *RefreshStore) Issue(ctx context.Context, userID string) (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(random)
	key := s.tokenKey(raw)
	if previous, err := s.cache.Get(ctx, s.userKey(userID)); err == nil {
		_, _ = s.cache.Delete(ctx, s.prefix+":token:"+previous)
	}
	if err := s.cache.Set(ctx, key, userID, s.ttl); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	if err := s.cache.Set(ctx, s.userKey(userID), s.hash(raw), s.ttl); err != nil {
		_, _ = s.cache.Delete(ctx, key)
		return "", fmt.Errorf("index refresh token: %w", err)
	}
	return raw, nil
}

func (s *RefreshStore) Consume(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", ErrInvalidRefreshToken
	}
	key := s.tokenKey(raw)
	userID, err := s.cache.Get(ctx, key)
	if err != nil {
		return "", ErrInvalidRefreshToken
	}
	deleted, err := s.cache.Delete(ctx, key)
	if err != nil || deleted != 1 {
		return "", ErrInvalidRefreshToken
	}
	_, _ = s.cache.Delete(ctx, s.userKey(userID))
	return userID, nil
}

func (s *RefreshStore) Revoke(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	_, err := s.cache.Delete(ctx, s.tokenKey(raw))
	return err
}

func (s *RefreshStore) RevokeAll(ctx context.Context, userID string) error {
	hash, err := s.cache.Get(ctx, s.userKey(userID))
	if err != nil {
		return nil
	}
	_, err = s.cache.Delete(ctx, s.prefix+":token:"+hash, s.userKey(userID))
	return err
}

func (s *RefreshStore) tokenKey(raw string) string { return s.prefix + ":token:" + s.hash(raw) }
func (s *RefreshStore) userKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return s.prefix + ":user:" + hex.EncodeToString(sum[:])
}
func (s *RefreshStore) hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
