package contract_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/httpserver/iamhttp"
	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
)

func TestHealthHTTPContractUsesSharedEnvelopeAndRequestID(t *testing.T) {
	t.Parallel()

	harness := newContractHarness(t)
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	recorder := httptest.NewRecorder()
	harness.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is missing")
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	decodeContractJSON(t, recorder, &envelope)
	if envelope.Code != http.StatusOK || envelope.Msg != "success" || envelope.Data.Status != "ok" {
		t.Fatalf("health envelope = %+v", envelope)
	}
}

func TestLoginHTTPContractReturnsTokenEnvelopeAndRefreshCookie(t *testing.T) {
	t.Parallel()

	harness := newContractHarness(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"password"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	harness.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Code int           `json:"code"`
		Msg  string        `json:"msg"`
		Data iam.TokenPair `json:"data"`
	}
	decodeContractJSON(t, recorder, &envelope)
	if envelope.Code != http.StatusOK || envelope.Msg != "success" || envelope.Data.AccessToken == "" {
		t.Fatalf("login envelope = %+v", envelope)
	}
	if envelope.Data.RefreshToken != "" {
		t.Fatal("refresh token must not be exposed in the JSON response")
	}
	cookies := make(map[string]*http.Cookie)
	for _, cookie := range recorder.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}
	if cookies[iamhttp.AccessCookieName] == nil || cookies[iamhttp.RefreshCookieName] == nil || !cookies[iamhttp.AccessCookieName].HttpOnly || !cookies[iamhttp.RefreshCookieName].HttpOnly {
		t.Fatalf("authentication cookies = %+v", cookies)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("authentication cache headers = %v", recorder.Header())
	}
}

func TestCurrentUserHTTPContractSupportsBearerAndRejectsAnonymous(t *testing.T) {
	t.Parallel()

	harness := newContractHarness(t)
	authorized := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	authorized.Header.Set("Authorization", "Bearer "+harness.accessToken)
	recorder := httptest.NewRecorder()
	harness.handler.ServeHTTP(recorder, authorized)
	if recorder.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var success struct {
		Code int                   `json:"code"`
		Msg  string                `json:"msg"`
		Data iam.AuthenticatedUser `json:"data"`
	}
	decodeContractJSON(t, recorder, &success)
	if success.Data.ID != contractUserID {
		t.Fatalf("current user = %+v", success.Data)
	}
	if got := strings.Join(success.Data.Permissions, ","); got != "users:read,tasks:read" {
		t.Fatalf("current user permissions = %q", got)
	}

	recorder = httptest.NewRecorder()
	harness.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var failure map[string]any
	decodeContractJSON(t, recorder, &failure)
	if failure["code"] != float64(http.StatusUnauthorized) || failure["msg"] == "" {
		t.Fatalf("error envelope = %#v", failure)
	}
}

func TestUsersListHTTPContractReturnsPaginatedUsersWithoutPasswordHashes(t *testing.T) {
	t.Parallel()

	harness := newContractHarness(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1&size=20&query=admin&is_active=true", nil)
	request.Header.Set("Authorization", "Bearer "+harness.accessToken)
	recorder := httptest.NewRecorder()
	harness.handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []iam.User      `json:"items"`
			Meta  pagination.Meta `json:"meta"`
		} `json:"data"`
	}
	decodeContractJSON(t, recorder, &envelope)
	if envelope.Code != http.StatusOK || envelope.Msg != "success" || len(envelope.Data.Items) != 1 {
		t.Fatalf("users envelope = %+v", envelope)
	}
	if envelope.Data.Items[0].PasswordHash != "" {
		t.Fatalf("password hash leaked in contract response: %+v", envelope.Data.Items[0])
	}
	if strings.Contains(recorder.Body.String(), "\"permissions\"") {
		t.Fatalf("list users response leaked caller permissions: %s", recorder.Body.String())
	}
	if envelope.Data.Meta.Page != 1 || envelope.Data.Meta.Size != 20 || envelope.Data.Meta.Total != 1 {
		t.Fatalf("pagination meta = %+v", envelope.Data.Meta)
	}
}

func decodeContractJSON(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
}
