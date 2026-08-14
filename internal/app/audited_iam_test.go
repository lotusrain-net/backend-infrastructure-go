package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/jyysy/backend-infrastructure-go/internal/modules/audit"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/httpserver/iamhttp"
	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
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
func (s iamStub) Preferences(context.Context, string) (iam.Preferences, error) {
	return iam.Preferences{}, s.err
}
func (s iamStub) PutPreferences(context.Context, string, iam.Preferences) error { return s.err }
func (s iamStub) Users(context.Context, iam.UserQuery) (pagination.Page[iam.User], error) {
	return pagination.Page[iam.User]{}, s.err
}
func (s iamStub) CreateUser(context.Context, iam.CreateUserInput) (iam.User, error) {
	return iam.User{}, s.err
}
func (s iamStub) UpdateUser(context.Context, string, iam.UpdateUserInput) (iam.User, error) {
	return iam.User{}, s.err
}
func (s iamStub) ResetUserPassword(context.Context, string, string) error  { return s.err }
func (s iamStub) SetUserActive(context.Context, string, bool) error        { return s.err }
func (s iamStub) ReplaceUserRoles(context.Context, string, []string) error { return s.err }
func (s iamStub) Roles(context.Context) ([]iam.Role, error)                { return nil, s.err }
func (s iamStub) Role(context.Context, string) (iam.RoleDetail, error) {
	return iam.RoleDetail{}, s.err
}
func (s iamStub) CreateRole(context.Context, iam.RoleInput) (iam.Role, error) {
	return iam.Role{}, s.err
}
func (s iamStub) UpdateRole(context.Context, string, iam.RoleInput) (iam.Role, error) {
	return iam.Role{}, s.err
}
func (s iamStub) DeleteRole(context.Context, string) error { return s.err }
func (s iamStub) ReplaceRolePermissions(context.Context, string, []string) error {
	return s.err
}
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

func TestAuditedIAMRecordsEveryManagementMutation(t *testing.T) {
	recorder := &preserveStub{}
	app := newAuditedIAM(iamStub{}, recorder)
	tests := []struct {
		name   string
		call   func() error
		action string
	}{
		{name: "update user", call: func() error {
			_, err := app.UpdateUser(context.Background(), "user-1", iam.UpdateUserInput{})
			return err
		}, action: "administration.user.update"},
		{name: "reset password", call: func() error { return app.ResetUserPassword(context.Background(), "user-1", "new-password-123") }, action: "administration.user.password.reset"},
		{name: "replace user roles", call: func() error { return app.ReplaceUserRoles(context.Background(), "user-1", []string{"role-1"}) }, action: "administration.user.roles.replace"},
		{name: "create role", call: func() error {
			_, err := app.CreateRole(context.Background(), iam.RoleInput{Name: "operators"})
			return err
		}, action: "administration.role.create"},
		{name: "update role", call: func() error {
			_, err := app.UpdateRole(context.Background(), "role-1", iam.RoleInput{Name: "operators"})
			return err
		}, action: "administration.role.update"},
		{name: "delete role", call: func() error { return app.DeleteRole(context.Background(), "role-1") }, action: "administration.role.delete"},
		{name: "replace role permissions", call: func() error {
			return app.ReplaceRolePermissions(context.Background(), "role-1", []string{"permission-1"})
		}, action: "administration.role.permissions.replace"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err != nil {
				t.Fatal(err)
			}
			if recorder.event.Action != test.action || recorder.event.Result != audit.ResultSuccess {
				t.Fatalf("event = %+v, want action %q", recorder.event, test.action)
			}
		})
	}
}
