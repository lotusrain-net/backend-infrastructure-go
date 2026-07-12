package contract_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend-infrastructure-go/internal/modules/iam"
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
	if cookies[iam.AccessCookieName] == nil || cookies[iam.RefreshCookieName] == nil || !cookies[iam.AccessCookieName].HttpOnly || !cookies[iam.RefreshCookieName].HttpOnly {
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
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		Data iam.User `json:"data"`
	}
	decodeContractJSON(t, recorder, &success)
	if success.Data.ID != contractUserID {
		t.Fatalf("current user = %+v", success.Data)
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

func decodeContractJSON(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
}
