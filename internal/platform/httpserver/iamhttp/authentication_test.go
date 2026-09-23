package iamhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
)

type authenticationStub struct {
	iam.AuthenticationApplication
	result iam.LoginResult
	err    error
}

func (s authenticationStub) Login(context.Context, iam.LoginInput) (iam.LoginResult, error) {
	return s.result, s.err
}
func (s authenticationStub) RequestEmailCode(context.Context, string, string, string) error {
	return s.err
}
func (s authenticationStub) Register(context.Context, iam.RegisterInput) (iam.User, error) {
	return iam.User{}, s.err
}
func TestMissingEmailCodeHasAnExplicitSignalWithoutCookies(t *testing.T) {
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/register"} {
		r := chi.NewRouter()
		RegisterRoutes(r, nil, nil, HTTPConfig{Authentication: authenticationStub{err: iam.ErrEmailCodeRequired}})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"email":"a@b.com","password":"secret"}`)))
		if w.Code != 422 || !strings.Contains(w.Body.String(), `"email_code":"required"`) || len(w.Result().Cookies()) != 0 {
			t.Fatal(w.Code, w.Body.String(), w.Header())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable verification response")
		}
	}
}
func TestChallengeHasNoCookieAndOnlyChallengeFields(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, nil, nil, HTTPConfig{Authentication: authenticationStub{result: iam.LoginResult{Status: "totp_required", ChallengeID: "challenge", ChallengeExpiresIn: 300}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"a@b.com","password":"secret"}`)))
	if w.Code != 202 || len(w.Result().Cookies()) != 0 {
		t.Fatal(w.Code, w.Header())
	}
	if strings.Contains(w.Body.String(), "access_token") || !strings.Contains(w.Body.String(), `"expires_in":300`) {
		t.Fatal(w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cacheable challenge")
	}
}
func TestEmailCodeErrorsAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{iam.ErrAuthenticationForbidden, 403}, {&iam.RateLimitError{RetryAfter: 60}, 429}, {nil, 202}} {
		r := chi.NewRouter()
		RegisterRoutes(r, nil, nil, HTTPConfig{Authentication: authenticationStub{err: tc.err}})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/auth/email-code", strings.NewReader(`{"email":"a@b.com","purpose":"register"}`)))
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.status == 429 && w.Header().Get("Retry-After") != "60" {
			t.Fatal(w.Header())
		}
		if strings.Contains(w.Body.String(), "initializ") || strings.Contains(w.Body.String(), "exist") {
			t.Fatal("state leak")
		}
	}
}
