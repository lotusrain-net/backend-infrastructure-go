package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
)

type authenticationAuditStub struct{ iam.AuthenticationApplication }

func (authenticationAuditStub) Login(context.Context, iam.LoginInput) (iam.LoginResult, error) {
	return iam.LoginResult{Status: "totp_required", ChallengeID: "secret-challenge", ChallengeExpiresIn: 300}, nil
}
func (authenticationAuditStub) VerifyLogin(context.Context, iam.VerifyInput) (iam.TokenPair, error) {
	return iam.TokenPair{}, iam.ErrInvalidCode
}
func (authenticationAuditStub) RequestEmailCode(context.Context, string, string, string) error {
	return nil
}
func (authenticationAuditStub) Register(context.Context, iam.RegisterInput) (iam.User, error) {
	return iam.User{ID: "user"}, nil
}
func (authenticationAuditStub) PutSettings(context.Context, iam.AuthenticationSettings) error {
	return nil
}
func (authenticationAuditStub) PutSecurity(context.Context, string, string) (iam.SecuritySettings, error) {
	return iam.SecuritySettings{Mode: "email"}, nil
}
func (authenticationAuditStub) Enroll(context.Context, string, iam.SecurityProof) (iam.Enrollment, error) {
	return iam.Enrollment{Secret: "secret-key"}, nil
}
func (authenticationAuditStub) Confirm(context.Context, string, string) ([]string, error) {
	return []string{"secret-recovery"}, nil
}
func (authenticationAuditStub) Disable(context.Context, string, iam.SecurityProof) error { return nil }
func TestAuthenticationAuditDoesNotRecordCredentials(t *testing.T) {
	recorder := &preserveStub{}
	service := auditedAuthentication{AuthenticationApplication: authenticationAuditStub{}, recorder: newAuditedIAM(iamStub{}, recorder)}
	ctx := iam.WithSubject(context.Background(), "user")
	cases := []struct {
		action string
		run    func()
	}{
		{"auth.login.challenge", func() {
			_, _ = service.Login(ctx, iam.LoginInput{Email: "private@example.com", Password: "secret-password", EmailCode: "123456"})
		}},
		{"auth.login.totp.verify", func() {
			_, _ = service.VerifyLogin(ctx, iam.VerifyInput{ChallengeID: "secret-challenge", RecoveryCode: "secret-recovery"})
		}},
		{"auth.email_code.request", func() { _ = service.RequestEmailCode(ctx, "private@example.com", "login", "127.0.0.1") }},
		{"auth.register", func() {
			_, _ = service.Register(ctx, iam.RegisterInput{Password: "secret-password", EmailCode: "123456"})
		}},
		{"authentication.settings.update", func() { _ = service.PutSettings(ctx, iam.DefaultAuthenticationSettings()) }},
		{"auth.security.update", func() { _, _ = service.PutSecurity(ctx, "user", "email") }},
		{"auth.totp.enroll", func() { _, _ = service.Enroll(ctx, "user", iam.SecurityProof{Password: "secret-password"}) }},
		{"auth.totp.confirm", func() { _, _ = service.Confirm(ctx, "user", "123456") }},
		{"auth.totp.disable", func() {
			_ = service.Disable(ctx, "user", iam.SecurityProof{Password: "secret-password", RecoveryCode: "secret-recovery"})
		}},
	}
	for _, tc := range cases {
		tc.run()
		if recorder.event.Action != tc.action {
			t.Fatal(tc.action, recorder.event.Action)
		}
		raw, _ := json.Marshal(recorder.event)
		for _, secret := range []string{"private@example.com", "secret-password", "123456", "secret-challenge", "secret-recovery", "secret-key"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("audit %s exposed credential", tc.action)
			}
		}
	}
}
