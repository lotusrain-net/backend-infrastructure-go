package iamhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/shared/pagination"
	"github.com/go-chi/chi/v5"
)

type fakeApplication struct {
	login                    iam.TokenPair
	user                     iam.AuthenticatedUser
	users                    pagination.Page[iam.User]
	userQuery                iam.UserQuery
	authorizationPermissions []string
	authorizationErr         error
	authErr                  error
	logoutToken              string
	loginCalls               int
	usersCalls               int
	setActiveCalls           int
	preferences              iam.Preferences
	preferenceUserID         string
	preferenceErr            error
	updateUserInput          iam.UpdateUserInput
	resetPassword            string
	userRoleIDs              []string
	roleDetail               iam.RoleDetail
	roleInput                iam.RoleInput
	roleID                   string
	permissionIDs            []string
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
func (f *fakeApplication) CurrentUser(context.Context, string) (iam.AuthenticatedUser, error) {
	return f.user, f.authErr
}
func (f *fakeApplication) Users(_ context.Context, query iam.UserQuery) (pagination.Page[iam.User], error) {
	f.usersCalls++
	f.userQuery = query
	return f.users, f.authErr
}
func (f *fakeApplication) CreateUser(context.Context, iam.CreateUserInput) (iam.User, error) {
	return f.user.User, f.authErr
}
func (f *fakeApplication) SetUserActive(context.Context, string, bool) error {
	f.setActiveCalls++
	return f.authErr
}
func (f *fakeApplication) UpdateUser(_ context.Context, _ string, input iam.UpdateUserInput) (iam.User, error) {
	f.updateUserInput = input
	return f.user.User, f.authErr
}
func (f *fakeApplication) ResetUserPassword(_ context.Context, _ string, password string) error {
	f.resetPassword = password
	return f.authErr
}
func (f *fakeApplication) ReplaceUserRoles(_ context.Context, _ string, roleIDs []string) error {
	f.userRoleIDs = append([]string(nil), roleIDs...)
	return f.authErr
}
func (f *fakeApplication) Preferences(_ context.Context, userID string) (iam.Preferences, error) {
	f.preferenceUserID = userID
	return f.preferences, f.preferenceErr
}
func (f *fakeApplication) PutPreferences(_ context.Context, userID string, preferences iam.Preferences) error {
	f.preferenceUserID = userID
	f.preferences = preferences
	return f.preferenceErr
}
func (f *fakeApplication) Roles(context.Context) ([]iam.Role, error) { return []iam.Role{}, f.authErr }
func (f *fakeApplication) Role(_ context.Context, roleID string) (iam.RoleDetail, error) {
	f.roleID = roleID
	return f.roleDetail, f.authErr
}
func (f *fakeApplication) CreateRole(_ context.Context, input iam.RoleInput) (iam.Role, error) {
	f.roleInput = input
	return iam.Role{ID: "role-1", Name: input.Name, Description: input.Description}, f.authErr
}
func (f *fakeApplication) UpdateRole(_ context.Context, roleID string, input iam.RoleInput) (iam.Role, error) {
	f.roleID, f.roleInput = roleID, input
	return iam.Role{ID: roleID, Name: input.Name, Description: input.Description}, f.authErr
}
func (f *fakeApplication) DeleteRole(_ context.Context, roleID string) error {
	f.roleID = roleID
	return f.authErr
}
func (f *fakeApplication) ReplaceRolePermissions(_ context.Context, roleID string, permissionIDs []string) error {
	f.roleID = roleID
	f.permissionIDs = append([]string(nil), permissionIDs...)
	return f.authErr
}
func (f *fakeApplication) Permissions(context.Context) ([]iam.Permission, error) {
	return []iam.Permission{}, f.authErr
}
func (f *fakeApplication) AssignRole(context.Context, string, string) error      { return f.authErr }
func (f *fakeApplication) GrantPermission(context.Context, string, string) error { return f.authErr }
func (f *fakeApplication) Authorize(_ context.Context, _ string, permission string) error {
	f.authorizationPermissions = append(f.authorizationPermissions, permission)
	if f.authorizationErr != nil {
		return f.authorizationErr
	}
	return f.authErr
}

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

func TestPreferencesRoutesUseOnlyAuthenticatedSubjectAndFixedPayload(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("user-1")
	app := &fakeApplication{preferences: iam.DefaultPreferences()}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/preferences", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.preferenceUserID != "user-1" {
		t.Fatalf("GET status=%d subject=%q body=%s", recorder.Code, app.preferenceUserID, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/users/me/preferences", strings.NewReader(`{"theme":"cyberpunk","color_mode":"dark","accent_color":"#1A2B3C","font_scale":"large","radius_scale":"rounded"}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.preferenceUserID != "user-1" {
		t.Fatalf("PUT status=%d subject=%q body=%s", recorder.Code, app.preferenceUserID, recorder.Body.String())
	}
	if app.preferences.Theme != iam.ThemeCyberpunk || app.preferences.AccentColor == nil || *app.preferences.AccentColor != "#1A2B3C" {
		t.Fatalf("PUT preferences = %+v", app.preferences)
	}
}

func TestManagementRoutesForwardAtomicPayloads(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("administrator")
	app := &fakeApplication{user: iam.AuthenticatedUser{User: iam.User{ID: "target", Active: true}}}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/target", strings.NewReader(`{"email":"target@example.com","username":"target","display_name":"Target"}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.updateUserInput.Username != "target" {
		t.Fatalf("PATCH user status=%d input=%+v body=%s", recorder.Code, app.updateUserInput, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/users/target/password/reset", strings.NewReader(`{"new_password":"new-password-123"}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.resetPassword != "new-password-123" {
		t.Fatalf("reset status=%d password=%q body=%s", recorder.Code, app.resetPassword, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/users/target/roles", strings.NewReader(`{"role_ids":["role-a","role-b"]}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !slices.Equal(app.userRoleIDs, []string{"role-a", "role-b"}) {
		t.Fatalf("replace user roles status=%d roles=%v body=%s", recorder.Code, app.userRoleIDs, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/roles", strings.NewReader(`{"name":"operators","description":"Operators"}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || app.roleInput.Name != "operators" {
		t.Fatalf("create role status=%d input=%+v body=%s", recorder.Code, app.roleInput, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/roles/role-a/permissions", strings.NewReader(`{"permission_ids":["permission-a"]}`))
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || app.roleID != "role-a" || !slices.Equal(app.permissionIDs, []string{"permission-a"}) {
		t.Fatalf("replace role permissions status=%d role=%q permissions=%v body=%s", recorder.Code, app.roleID, app.permissionIDs, recorder.Body.String())
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
	app := &fakeApplication{user: iam.AuthenticatedUser{User: iam.User{ID: "u1", Active: true}}}
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

func TestUsersListReturnsPaginatedEnvelopeAndRejectsInvalidBooleanFilter(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{users: pagination.New([]iam.User{{
		ID: "u1", Email: "alice@example.com", Username: "alice", DisplayName: "Alice", Active: true,
	}}, 2, 5, 11)}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=2&size=5&query=Ali&is_active=true", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Items []iam.User      `json:"items"`
			Meta  pagination.Meta `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].PasswordHash != "" {
		t.Fatalf("items=%+v", envelope.Data.Items)
	}
	if strings.Contains(recorder.Body.String(), "\"permissions\"") {
		t.Fatalf("list response leaked permissions: %s", recorder.Body.String())
	}
	if envelope.Data.Meta.Page != 2 || envelope.Data.Meta.Size != 5 || envelope.Data.Meta.Total != 11 {
		t.Fatalf("meta=%+v", envelope.Data.Meta)
	}
	if app.userQuery.Filter.Query != "Ali" || app.userQuery.Filter.Active == nil || !*app.userQuery.Filter.Active || app.userQuery.Page != 2 || app.userQuery.Size != 5 {
		t.Fatalf("forwarded query=%+v", app.userQuery)
	}

	bad := httptest.NewRequest(http.MethodGet, "/api/v1/users?is_active=definitely", nil)
	bad.Header.Set("Authorization", "Bearer "+raw)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, bad)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid bool status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUsersListRouteIsPartOfAuthenticatedManagementSurface(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{users: pagination.New([]iam.User{}, 1, 20, 0)}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if slices.Contains([]int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code) {
		t.Fatalf("users list route missing status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUsersListRequiresUsersReadPermission(t *testing.T) {
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	raw, _ := jwt.Issue("u1")
	app := &fakeApplication{authorizationErr: iam.ErrPermissionDenied}
	router := chi.NewRouter()
	RegisterRoutes(router, app, jwt, HTTPConfig{RefreshTTL: time.Hour})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !slices.Equal(app.authorizationPermissions, []string{"users:read"}) {
		t.Fatalf("requested permissions=%v", app.authorizationPermissions)
	}
	if app.usersCalls != 0 {
		t.Fatalf("Users() calls=%d", app.usersCalls)
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
func (f *fakeUsers) List(context.Context, iam.UserFilter, int, int) ([]iam.User, int64, error) {
	return nil, 0, nil
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
