package app

import (
	"context"
	"errors"
	"testing"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/iam"
)

type preserveStub struct{ event audit.NewEvent }

func (p *preserveStub) Preserve(_ context.Context, event audit.NewEvent, primary error) error {
	p.event = event
	return primary
}

type iamStub struct{ err error }

func (s iamStub) Login(context.Context, string, string) (iam.TokenPair, error) {
	return iam.TokenPair{}, s.err
}
func (s iamStub) Refresh(context.Context, string) (iam.TokenPair, error) {
	return iam.TokenPair{}, s.err
}
func (s iamStub) Logout(context.Context, string) error                  { return s.err }
func (s iamStub) CurrentUser(context.Context, string) (iam.User, error) { return iam.User{}, s.err }
func (s iamStub) CreateUser(context.Context, iam.CreateUserInput) (iam.User, error) {
	return iam.User{}, s.err
}
func (s iamStub) SetUserActive(context.Context, string, bool) error     { return s.err }
func (s iamStub) Roles(context.Context) ([]iam.Role, error)             { return nil, s.err }
func (s iamStub) Permissions(context.Context) ([]iam.Permission, error) { return nil, s.err }
func (s iamStub) AssignRole(context.Context, string, string) error      { return s.err }
func (s iamStub) GrantPermission(context.Context, string, string) error { return s.err }
func (s iamStub) Authorize(context.Context, string, string) error       { return s.err }

func TestAuditedIAMPreservesAuthorizationFailureAndRecordsIt(t *testing.T) {
	want := errors.New("denied")
	recorder := &preserveStub{}
	app := newAuditedIAM(iamStub{err: want}, recorder)
	err := app.Authorize(context.Background(), "user-1", "users:write")
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
	if recorder.event.Action != "authorization.check" || recorder.event.Result != audit.ResultFailure || recorder.event.ActorID == nil || *recorder.event.ActorID != "user-1" {
		t.Fatalf("event=%+v", recorder.event)
	}
}
