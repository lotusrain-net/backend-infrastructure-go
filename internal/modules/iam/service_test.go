package iam

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryRefreshCache struct {
	values  map[string]string
	deleted []string
}

func (m *memoryRefreshCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value.(string)
	return nil
}
func (m *memoryRefreshCache) Get(_ context.Context, key string) (string, error) {
	v, ok := m.values[key]
	if !ok {
		return "", errors.New("missing")
	}
	return v, nil
}
func (m *memoryRefreshCache) Delete(_ context.Context, keys ...string) (int64, error) {
	var n int64
	for _, k := range keys {
		if _, ok := m.values[k]; ok {
			delete(m.values, k)
			n++
		}
		m.deleted = append(m.deleted, k)
	}
	return n, nil
}

func TestRefreshTokensStoredHashedConsumedAndCannotReplay(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, err := store.Issue(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(mapKeys(cache.values), ""), raw) {
		t.Fatal("raw refresh token used as cache key")
	}
	if strings.Contains(strings.Join(mapValues(cache.values), ""), raw) {
		t.Fatal("raw refresh token stored as cache value")
	}
	userID, err := store.Consume(context.Background(), raw)
	if err != nil || userID != "user-1" {
		t.Fatalf("user=%q err=%v", userID, err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay error=%v", err)
	}
}

func TestRefreshRevokeInvalidatesToken(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, _ := store.Issue(context.Background(), "user-1")
	if err := store.Revoke(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("revoked error=%v", err)
	}
}

type fakeUsers struct {
	byEmail   map[string]User
	byID      map[string]User
	createErr error
}

type fakeRBAC struct {
	permissions []string
}

func (f *fakeRBAC) PermissionsForUser(context.Context, string) ([]string, error) {
	return f.permissions, nil
}
func (f *fakeRBAC) Roles(context.Context) ([]Role, error) { return []Role{{Name: "admin"}}, nil }
func (f *fakeRBAC) Permissions(context.Context) ([]Permission, error) {
	return []Permission{{Name: "users:read"}}, nil
}
func (f *fakeRBAC) AssignRole(context.Context, string, string) error      { return nil }
func (f *fakeRBAC) GrantPermission(context.Context, string, string) error { return nil }

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (f *fakeUsers) FindByID(_ context.Context, id string) (User, error) {
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (f *fakeUsers) Create(_ context.Context, input CreateUserInput) (User, error) {
	if f.createErr != nil {
		return User{}, f.createErr
	}
	return User{ID: "new", Email: input.Email, Username: input.Username, Active: true}, nil
}
func (f *fakeUsers) SetActive(context.Context, string, bool) error { return nil }

func TestLoginRejectsInactiveUserAndBadPassword(t *testing.T) {
	hasher := NewPasswordHasher(DefaultArgon2Params())
	hash, _ := hasher.Hash("secret-password")
	repo := &fakeUsers{byEmail: map[string]User{"inactive@example.com": {ID: "u1", Email: "inactive@example.com", PasswordHash: hash, Active: false}, "active@example.com": {ID: "u2", Email: "active@example.com", PasswordHash: hash, Active: true}}}
	auth := newTestAuth(t, repo)
	if _, err := auth.Login(context.Background(), "inactive@example.com", "secret-password"); !errors.Is(err, ErrInactiveUser) {
		t.Fatalf("inactive error=%v", err)
	}
	if _, err := auth.Login(context.Background(), "active@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("bad password error=%v", err)
	}
}

func TestCreateUserSurfacesDuplicateIdentity(t *testing.T) {
	auth := newTestAuth(t, &fakeUsers{createErr: ErrDuplicateIdentity})
	_, err := auth.CreateUser(context.Background(), CreateUserInput{Email: "taken@example.com", Username: "taken", Password: "password-long-enough"})
	if !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("error=%v", err)
	}
}

func TestServiceLoginRefreshLogoutAndAuthorization(t *testing.T) {
	hasher := NewPasswordHasher(DefaultArgon2Params())
	hash, _ := hasher.Hash("secret-password")
	user := User{ID: "u1", Email: "active@example.com", PasswordHash: hash, Active: true}
	users := &fakeUsers{byEmail: map[string]User{user.Email: user}, byID: map[string]User{user.ID: user}}
	cache := &memoryRefreshCache{}
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	rbac := &fakeRBAC{permissions: []string{"users:*"}}
	service := NewService(users, rbac, hasher, jwt, NewRefreshStore(cache, "iam:test", time.Hour))

	pair, err := service.Login(context.Background(), " ACTIVE@EXAMPLE.COM ", "secret-password")
	if err != nil || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("pair=%+v err=%v", pair, err)
	}
	rotated, err := service.Refresh(context.Background(), pair.RefreshToken)
	if err != nil || rotated.RefreshToken == pair.RefreshToken {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
	if _, err := service.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay=%v", err)
	}
	if err := service.Authorize(context.Background(), user.ID, "users:write"); err != nil {
		t.Fatal(err)
	}
	rbac.permissions = []string{"users:read"}
	if err := service.Authorize(context.Background(), user.ID, "users:write"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("deny=%v", err)
	}
	if current, err := service.CurrentUser(context.Background(), user.ID); err != nil || current.ID != user.ID {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	if err := service.Logout(context.Background(), rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
}

func TestRevokeAllInvalidatesLatestRefreshToken(t *testing.T) {
	cache := &memoryRefreshCache{}
	store := NewRefreshStore(cache, "iam:test", time.Hour)
	raw, _ := store.Issue(context.Background(), "u1")
	if err := store.RevokeAll(context.Background(), "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(context.Background(), raw); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("consume=%v", err)
	}
}

func newTestAuth(t *testing.T, users UserRepository) *Service {
	t.Helper()
	jwt, err := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(users, nil, NewPasswordHasher(DefaultArgon2Params()), jwt, NewRefreshStore(&memoryRefreshCache{}, "iam:test", time.Hour))
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
