package iam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type fakeApplication struct {
	login       TokenPair
	user        User
	authErr     error
	logoutToken string
}

func (f *fakeApplication) Login(context.Context, string, string) (TokenPair, error) {
	return f.login, f.authErr
}
func (f *fakeApplication) Refresh(context.Context, string) (TokenPair, error) {
	return f.login, f.authErr
}
func (f *fakeApplication) Logout(_ context.Context, token string) error {
	f.logoutToken = token
	return f.authErr
}
func (f *fakeApplication) CurrentUser(context.Context, string) (User, error) {
	return f.user, f.authErr
}
func (f *fakeApplication) CreateUser(context.Context, CreateUserInput) (User, error) {
	return f.user, f.authErr
}
func (f *fakeApplication) SetUserActive(context.Context, string, bool) error { return f.authErr }
func (f *fakeApplication) Roles(context.Context) ([]Role, error)             { return []Role{}, f.authErr }
func (f *fakeApplication) Permissions(context.Context) ([]Permission, error) {
	return []Permission{}, f.authErr
}
func (f *fakeApplication) AssignRole(context.Context, string, string) error      { return f.authErr }
func (f *fakeApplication) GrantPermission(context.Context, string, string) error { return f.authErr }
func (f *fakeApplication) Authorize(context.Context, string, string) error       { return f.authErr }

func TestExtractAccessTokenBearerThenCookie(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "cookie-token"})
	r.Header.Set("Authorization", "Bearer header-token")
	if got, err := ExtractAccessToken(r); err != nil || got != "header-token" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	r.Header.Del("Authorization")
	if got, err := ExtractAccessToken(r); err != nil || got != "cookie-token" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestLoginSetsSecureRefreshCookie(t *testing.T) {
	app := &fakeApplication{login: TokenPair{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 60}}
	router := chi.NewRouter()
	RegisterRoutes(router, app, nil, HTTPConfig{SecureCookies: true, RefreshTTL: 24 * time.Hour})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@example.com","password":"password"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d", len(cookies))
	}
	c := cookies[0]
	if c.Name != RefreshCookieName || c.Value != "refresh" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/v1/auth" {
		t.Fatalf("unsafe cookie: %+v", c)
	}
}

func TestLogoutRevokesCookieAndClearsIt(t *testing.T) {
	app := &fakeApplication{}
	router := chi.NewRouter()
	RegisterRoutes(router, app, nil, HTTPConfig{SecureCookies: true, RefreshTTL: time.Hour})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "refresh"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if app.logoutToken != "refresh" {
		t.Fatalf("revoked=%q", app.logoutToken)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Fatalf("cookie not cleared: %+v", cookies)
	}
}

func TestMeRejectsTamperedTokenAndInactiveUser(t *testing.T) {
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	app := &fakeApplication{authErr: ErrInactiveUser}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer tampered")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered status=%d", rec.Code)
	}
	raw, _ := jwt.Issue("u1")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("inactive status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPermissionMiddlewareDeniesMissingPermission(t *testing.T) {
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{authErr: ErrPermissionDenied}
	router := chi.NewRouter()
	router.With(Authenticate(jwt), RequirePermission(app, "users:write")).Get("/protected", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["msg"] != "permission denied" {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
}

func TestExtractAccessTokenRejectsMalformedAuthorization(t *testing.T) {
	for _, value := range []string{"Basic abc", "Bearer", "Bearer one two"} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", value)
		if _, err := ExtractAccessToken(r); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("%q err=%v", value, err)
		}
	}
}

func TestAuthenticatedManagementRoutes(t *testing.T) {
	jwt, _ := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{user: User{ID: "u1", Active: true}}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})
	tests := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"refresh"}`, http.StatusOK},
		{http.MethodPost, "/api/v1/users", `{"Email":"a@example.com","Username":"alice","Password":"long-password"}`, http.StatusCreated},
		{http.MethodPatch, "/api/v1/users/u2/active", `{"is_active":true}`, http.StatusOK},
		{http.MethodGet, "/api/v1/roles", "", http.StatusOK},
		{http.MethodGet, "/api/v1/permissions", "", http.StatusOK},
		{http.MethodPost, "/api/v1/users/u2/roles/r1", "", http.StatusOK},
		{http.MethodPost, "/api/v1/roles/r1/permissions/p1", "", http.StatusOK},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		if !strings.HasPrefix(tt.path, "/api/v1/auth/") {
			req.Header.Set("Authorization", "Bearer "+raw)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tt.status {
			t.Errorf("%s %s status=%d body=%s", tt.method, tt.path, rec.Code, rec.Body.String())
		}
	}
}
