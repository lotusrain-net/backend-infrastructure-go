package iamhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend-infrastructure-go/internal/modules/iam"
	"github.com/go-chi/chi/v5"
)

type fakeApplication struct {
	login          iam.TokenPair
	user           iam.User
	authErr        error
	logoutToken    string
	loginCalls     int
	setActiveCalls int
}

func (f *fakeApplication) Login(context.Context, string, string) (iam.TokenPair, error) {
	f.loginCalls++
	return f.login, f.authErr
}
func (f *fakeApplication) Refresh(context.Context, string) (iam.TokenPair, error) {
	return f.login, f.authErr
}
func (f *fakeApplication) Logout(_ context.Context, token string) error {
	f.logoutToken = token
	return f.authErr
}
func (f *fakeApplication) CurrentUser(context.Context, string) (iam.User, error) {
	return f.user, f.authErr
}
func (f *fakeApplication) CreateUser(context.Context, iam.CreateUserInput) (iam.User, error) {
	return f.user, f.authErr
}
func (f *fakeApplication) SetUserActive(context.Context, string, bool) error {
	f.setActiveCalls++
	return f.authErr
}
func (f *fakeApplication) Roles(context.Context) ([]iam.Role, error) { return []iam.Role{}, f.authErr }
func (f *fakeApplication) Permissions(context.Context) ([]iam.Permission, error) {
	return []iam.Permission{}, f.authErr
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

func TestLoginAndRefreshSetSecureCookiesWithoutExposingRefreshToken(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "login", path: "/api/v1/auth/login", body: `{"email":"a@example.com","password":"password"}`},
		{name: "refresh", path: "/api/v1/auth/refresh", body: `{"refresh_token":"old-refresh"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := &fakeApplication{login: iam.TokenPair{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 60}}
			router := chi.NewRouter()
			RegisterRoutes(router, app, nil, HTTPConfig{SecureCookies: true, RefreshTTL: 24 * time.Hour})
			req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			cookies := map[string]*http.Cookie{}
			for _, cookie := range rec.Result().Cookies() {
				cookies[cookie.Name] = cookie
			}
			access := cookies[AccessCookieName]
			if access == nil || access.Value != "access" || !access.HttpOnly || !access.Secure || access.SameSite != http.SameSiteStrictMode || access.Path != "/api/v1" || access.MaxAge != 60 {
				t.Fatalf("unsafe access cookie: %+v", access)
			}
			refresh := cookies[RefreshCookieName]
			if refresh == nil || refresh.Value != "refresh" || !refresh.HttpOnly || !refresh.Secure || refresh.SameSite != http.SameSiteStrictMode || refresh.Path != "/api/v1/auth" || refresh.MaxAge != int((24*time.Hour).Seconds()) {
				t.Fatalf("unsafe refresh cookie: %+v", refresh)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
				t.Fatalf("cache headers = Cache-Control:%q Pragma:%q", rec.Header().Get("Cache-Control"), rec.Header().Get("Pragma"))
			}
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Data["access_token"] != "access" {
				t.Fatalf("access token response = %v", envelope.Data)
			}
			if _, exposed := envelope.Data["refresh_token"]; exposed {
				t.Fatalf("refresh token exposed in response: %v", envelope.Data)
			}
		})
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
	cookies := map[string]*http.Cookie{}
	for _, cookie := range rec.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	if access, refresh := cookies[AccessCookieName], cookies[RefreshCookieName]; access == nil || refresh == nil || access.MaxAge != -1 || refresh.MaxAge != -1 {
		t.Fatalf("cookies not cleared: %+v", cookies)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers = Cache-Control:%q Pragma:%q", rec.Header().Get("Cache-Control"), rec.Header().Get("Pragma"))
	}
}

func TestSetUserActiveRequiresExplicitBoolean(t *testing.T) {
	app := &fakeApplication{}
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/user-1/active", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()

	(handler{app: app}).setUserActive(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if app.setActiveCalls != 0 {
		t.Fatalf("SetUserActive() calls = %d", app.setActiveCalls)
	}
}

func TestMeRejectsTamperedTokenAndInactiveUser(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	app := &fakeApplication{authErr: iam.ErrInactiveUser}
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
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{authErr: iam.ErrPermissionDenied}
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
		if _, err := ExtractAccessToken(r); !errors.Is(err, iam.ErrInvalidCredentials) {
			t.Fatalf("%q err=%v", value, err)
		}
	}
}

func TestAuthenticatedManagementRoutes(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{user: iam.User{ID: "u1", Active: true}}
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

func TestCreateUserRejectsPasswordsBelowServiceMinimumWithout500(t *testing.T) {
	service := newTestAuthService(t)
	for _, password := range []string{"12345678", "12345678901"} {
		t.Run(password, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{"email":"a@example.com","username":"alice","password":"`+password+`"}`))
			rec := httptest.NewRecorder()
			(handler{app: service}).createUser(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLoginRejectsBodyLargerThanOneMiBIncludingTrailingWhitespace(t *testing.T) {
	app := &fakeApplication{}
	router := chi.NewRouter()
	RegisterRoutes(router, app, nil, HTTPConfig{RefreshTTL: time.Hour})
	body := `{"email":"a@example.com","password":"password"}` + strings.Repeat(" ", (1<<20)-len(`{"email":"a@example.com","password":"password"}`)+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if app.loginCalls != 0 {
		t.Fatalf("Login() calls = %d", app.loginCalls)
	}
}

type fakeUsers struct{}

func (f *fakeUsers) FindByEmail(context.Context, string) (iam.User, error) {
	return iam.User{}, iam.ErrNotFound
}
func (f *fakeUsers) FindByID(context.Context, string) (iam.User, error) {
	return iam.User{}, iam.ErrNotFound
}
func (f *fakeUsers) Create(_ context.Context, input iam.CreateUserInput) (iam.User, error) {
	return iam.User{ID: "new", Email: input.Email, Username: input.Username, Active: true}, nil
}
func (f *fakeUsers) SetActive(context.Context, string, bool) error { return nil }

func newTestAuthService(t *testing.T) *iam.Service {
	t.Helper()
	jwt, err := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return iam.NewService(&fakeUsers{}, nil, iam.NewPasswordHasher(iam.DefaultArgon2Params()), jwt, iam.NewRefreshStore(memoryRefreshCache{}, "iam:test", time.Hour))
}

type memoryRefreshCache struct{}

func (cache memoryRefreshCache) Get(context.Context, string) (string, error) {
	return "", iam.ErrInvalidRefreshToken
}

func (cache memoryRefreshCache) Eval(context.Context, string, []string, ...any) (any, error) {
	return "ok", nil
}
