package contract_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
)

type contractAuthentication struct{ app *contractApplication }

func (a contractAuthentication) Login(_ context.Context, in iam.LoginInput) (iam.LoginResult, error) {
	if in.Email == "totp@example.com" {
		return iam.LoginResult{Status: "totp_required", ChallengeID: strings.Repeat("a", 64), ChallengeExpiresIn: 300}, nil
	}
	return iam.LoginResult{TokenPair: a.app.pair}, nil
}
func (a contractAuthentication) VerifyLogin(context.Context, iam.VerifyInput) (iam.TokenPair, error) {
	return a.app.pair, nil
}
func (a contractAuthentication) RequestEmailCode(context.Context, string, string, string) error {
	return nil
}
func (a contractAuthentication) Register(context.Context, iam.RegisterInput) (iam.User, error) {
	return a.app.user.User, nil
}
func (a contractAuthentication) Settings(context.Context) (iam.AuthenticationSettings, error) {
	return iam.DefaultAuthenticationSettings(), nil
}
func (a contractAuthentication) PutSettings(context.Context, iam.AuthenticationSettings) error {
	return nil
}
func (a contractAuthentication) Security(context.Context, string) (iam.SecuritySettings, error) {
	return iam.SecuritySettings{Mode: "default"}, nil
}
func (a contractAuthentication) PutSecurity(_ context.Context, _ string, mode string) (iam.SecuritySettings, error) {
	return iam.SecuritySettings{Mode: mode}, nil
}
func (a contractAuthentication) Enroll(context.Context, string, iam.SecurityProof) (iam.Enrollment, error) {
	return iam.Enrollment{Secret: "TESTSECRET", OTPAuthURI: "otpauth://totp/test?secret=TESTSECRET", ExpiresIn: 600}, nil
}
func (a contractAuthentication) Confirm(context.Context, string, string) ([]string, error) {
	return []string{"code-1", "code-2", "code-3", "code-4", "code-5", "code-6", "code-7", "code-8", "code-9", "code-10"}, nil
}
func (a contractAuthentication) Disable(context.Context, string, iam.SecurityProof) error { return nil }

func TestAuthenticationContractsUseAcceptedChallengeAndCreatedUserWithoutCookies(t *testing.T) {
	h := newContractHarness(t)
	for _, tc := range []struct {
		path, body string
		status     int
		field      string
	}{{"/api/v1/auth/login", `{"email":"totp@example.com","password":"password"}`, 202, "challenge_id"}, {"/api/v1/auth/email-code", `{"email":"unknown@example.com","purpose":"login"}`, 202, "resend_after_seconds"}, {"/api/v1/auth/register", `{"email":"new@example.com","username":"new","password":"long-password"}`, 201, "username"}} {
		r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		h.handler.ServeHTTP(w, r)
		if w.Code != tc.status || len(w.Result().Cookies()) != 0 || !strings.Contains(w.Body.String(), tc.field) {
			t.Fatal(tc.path, w.Code, w.Body.String(), w.Header())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable credentials")
		}
	}
}
func TestSettingsContractRejectsBootstrapMutation(t *testing.T) {
	h := newContractHarness(t)
	r := httptest.NewRequest("PUT", "/api/v1/system-settings/basic-auth", strings.NewReader(`{"password_login_enabled":true,"registration_enabled":false,"registration_email_verification_required":true,"allowed_email_domains":[],"initialized_at":"2026-01-01T00:00:00Z"}`))
	r.Header.Set("Authorization", "Bearer "+h.accessToken)
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
}
