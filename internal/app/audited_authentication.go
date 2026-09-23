package app

import (
	"context"

	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
)

type auditedAuthentication struct {
	iam.AuthenticationApplication
	recorder *auditedIAM
}

func (a auditedAuthentication) Login(ctx context.Context, in iam.LoginInput) (iam.LoginResult, error) {
	v, e := a.AuthenticationApplication.Login(ctx, in)
	action := "auth.login"
	if e == nil && v.Status == "totp_required" {
		action = "auth.login.challenge"
	}
	return v, a.recorder.record(ctx, action, "session", "", nil, e)
}
func (a auditedAuthentication) VerifyLogin(ctx context.Context, in iam.VerifyInput) (iam.TokenPair, error) {
	v, e := a.AuthenticationApplication.VerifyLogin(ctx, in)
	return v, a.recorder.record(ctx, "auth.login.totp.verify", "session", "", nil, e)
}
func (a auditedAuthentication) RequestEmailCode(ctx context.Context, email, purpose, ip string) error {
	e := a.AuthenticationApplication.RequestEmailCode(ctx, email, purpose, ip)
	return a.recorder.record(ctx, "auth.email_code.request", "authentication", "", nil, e)
}
func (a auditedAuthentication) Register(ctx context.Context, in iam.RegisterInput) (iam.User, error) {
	v, e := a.AuthenticationApplication.Register(ctx, in)
	return v, a.recorder.record(ctx, "auth.register", "user", v.ID, nil, e)
}
func (a auditedAuthentication) PutSettings(ctx context.Context, v iam.AuthenticationSettings) error {
	e := a.AuthenticationApplication.PutSettings(ctx, v)
	return a.recorder.record(ctx, "authentication.settings.update", "authentication", "", nil, e)
}
func (a auditedAuthentication) PutSecurity(ctx context.Context, id, mode string) (iam.SecuritySettings, error) {
	v, e := a.AuthenticationApplication.PutSecurity(ctx, id, mode)
	return v, a.recorder.record(ctx, "auth.security.update", "user", id, nil, e)
}
func (a auditedAuthentication) Enroll(ctx context.Context, id string, proof iam.SecurityProof) (iam.Enrollment, error) {
	v, e := a.AuthenticationApplication.Enroll(ctx, id, proof)
	return v, a.recorder.record(ctx, "auth.totp.enroll", "user", id, nil, e)
}
func (a auditedAuthentication) Confirm(ctx context.Context, id, code string) ([]string, error) {
	v, e := a.AuthenticationApplication.Confirm(ctx, id, code)
	return v, a.recorder.record(ctx, "auth.totp.confirm", "user", id, nil, e)
}
func (a auditedAuthentication) Disable(ctx context.Context, id string, proof iam.SecurityProof) error {
	e := a.AuthenticationApplication.Disable(ctx, id, proof)
	return a.recorder.record(ctx, "auth.totp.disable", "user", id, nil, e)
}
