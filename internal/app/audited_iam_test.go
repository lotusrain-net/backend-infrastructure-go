package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/httpserver/iamhttp"
	"backend-infrastructure-go/internal/shared/pagination"
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
func (s iamStub) Logout(context.Context, string) error { return s.err }
func (s iamStub) CurrentUser(context.Context, string) (iam.AuthenticatedUser, error) {
	return iam.AuthenticatedUser{}, s.err
}
func (s iamStub) Users(context.Context, iam.UserQuery) (pagination.Page[iam.User], error) {
	return pagination.Page[iam.User]{}, s.err
}
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

func TestAuditedIAMRecordsAuthenticatedActorAndRequestMetadata(t *testing.T) {
	recorder := &preserveStub{}
	app := newAuditedIAM(iamStub{}, recorder)
	ip := netip.MustParseAddr("203.0.113.10")
	jwt, _ := iam.NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "test", time.Minute)
	token, _ := jwt.Issue("administrator-1")
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request = request.WithContext(audit.WithRequestMetadata(request.Context(), audit.RequestMetadata{IPAddress: &ip, UserAgent: "admin-client/1.0"}))
	iamhttp.Authenticate(jwt)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		if _, err := app.CreateUser(request.Context(), iam.CreateUserInput{}); err != nil {
			t.Error(err)
		}
	})).ServeHTTP(httptest.NewRecorder(), request)
	if recorder.event.ActorID == nil || *recorder.event.ActorID != "administrator-1" || recorder.event.IPAddress == nil || *recorder.event.IPAddress != ip || recorder.event.UserAgent != "admin-client/1.0" {
		t.Fatalf("event = %+v", recorder.event)
	}
}
