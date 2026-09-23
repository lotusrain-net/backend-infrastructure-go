package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

type authRepoStub struct {
	AuthenticationRepository
	state       AuthenticationState
	security    SecurityState
	initialized int
	registered  int
	receipt     *CodeReceipt
}

func (r *authRepoStub) Authentication(context.Context) (AuthenticationState, error) {
	return r.state, nil
}
func (r *authRepoStub) Initialize(_ context.Context, id string) error {
	if id != r.state.BootstrapAdminUserID {
		return ErrInvalidCredentials
	}
	r.initialized++
	now := time.Now()
	r.state.InitializedAt = &now
	return nil
}
func (r *authRepoStub) Security(context.Context, string) (SecurityState, error) {
	s := r.security
	if s.Mode == "" {
		s.Mode = "default"
	}
	return s, nil
}
func (r *authRepoStub) Register(_ context.Context, in CreateUserInput, receipt *CodeReceipt) (User, error) {
	r.registered++
	r.receipt = receipt
	return User{ID: "new", Email: in.Email}, nil
}

type authUsersStub struct {
	UserRepository
	user User
}

func (r authUsersStub) FindByEmail(context.Context, string) (User, error) { return r.user, nil }
func (r authUsersStub) FindByID(context.Context, string) (User, error)    { return r.user, nil }

type authPasswords struct{}

func (authPasswords) Hash(string) (string, error)      { return "hashed", nil }
func (authPasswords) Verify(p, h string) (bool, error) { return p == "correct-password", nil }

type authCodesStub struct {
	EmailCodes
	issued   int
	verified int
	consumed int
}

func (c *authCodesStub) Issue(context.Context, string, string, string) (string, error) {
	c.issued++
	return "123456", nil
}
func (c *authCodesStub) Verify(_ context.Context, _, _, code string) (CodeReceipt, error) {
	c.verified++
	if code != "123456" {
		return CodeReceipt{}, ErrInvalidCode
	}
	return CodeReceipt{ID: "receipt", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (c *authCodesStub) Consume(context.Context, string, string, CodeReceipt) error {
	c.consumed++
	return nil
}

type authMailStub struct{}

func (authMailStub) SendCode(context.Context, string, string, string) error { return nil }
func testAuthentication(t *testing.T, repo *authRepoStub) (*AuthenticationService, *authCodesStub) {
	t.Helper()
	jwt, _ := NewJWTManager([]byte("01234567890123456789012345678901"), "test", time.Minute)
	base := NewService(authUsersStub{user: User{ID: "user", Email: "u@example.com", Active: true}}, nil, authPasswords{}, jwt, NewRefreshStore(&memoryRefreshCache{}, "test", time.Hour))
	codes := &authCodesStub{}
	return NewAuthenticationService(base, repo, codes, authMailStub{}, nil, nil), codes
}
func TestBootstrapGateAndFirstLogin(t *testing.T) {
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), BootstrapAdminUserID: "admin"}}
	s, codes := testAuthentication(t, repo)
	ctx := context.Background()
	if _, e := s.Login(ctx, LoginInput{Email: "u@example.com", Password: "correct-password"}); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal(e)
	}
	if e := s.RequestEmailCode(ctx, "u@example.com", "register", "ip"); !errors.Is(e, ErrAuthenticationForbidden) || codes.issued != 0 {
		t.Fatal(e)
	}
	if _, e := s.Register(ctx, RegisterInput{}); !errors.Is(e, ErrAuthenticationForbidden) {
		t.Fatal(e)
	}
	repo.state.BootstrapAdminUserID = "user"
	result, e := s.Login(ctx, LoginInput{Email: "u@example.com", Password: "correct-password"})
	if e != nil || result.AccessToken == "" || repo.initialized != 1 {
		t.Fatal(result, e)
	}
}
func TestRegistrationPolicyCombinations(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		enabled, verify bool
		code            string
		want            error
		registered      int
	}{{false, true, "", ErrAuthenticationForbidden, 0}, {true, true, "", ErrEmailCodeRequired, 0}, {true, true, "000000", ErrInvalidCode, 0}, {true, true, "123456", nil, 1}, {true, false, "", nil, 1}} {
		repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}}
		repo.state.RegistrationEnabled = tc.enabled
		repo.state.RegistrationEmailVerificationRequired = tc.verify
		s, codes := testAuthentication(t, repo)
		_, e := s.Register(context.Background(), RegisterInput{Username: "new", Email: "n@example.com", Password: "correct-password", EmailCode: tc.code})
		if !errors.Is(e, tc.want) || repo.registered != tc.registered {
			t.Fatalf("%+v: %v", tc, e)
		}
		if tc.registered == 1 && ((repo.receipt != nil) != tc.verify) {
			t.Fatal("verification marker differs from policy")
		}
		if tc.code == "" && codes.verified != 0 {
			t.Fatal("missing code must not count as a verification attempt")
		}
	}
}

func TestEmailLoginRequestsVerificationOnlyAfterValidCredentials(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}, security: SecurityState{SecuritySettings: SecuritySettings{Mode: "email"}}}
	s, codes := testAuthentication(t, repo)
	s.base.users = authUsersStub{user: User{ID: "user", Email: "u@example.com", Active: true, EmailVerifiedAt: &now}}
	for _, tc := range []struct {
		password, code string
		want           error
	}{{"wrong-password", "", ErrInvalidCredentials}, {"correct-password", "", ErrEmailCodeRequired}, {"correct-password", "000000", ErrInvalidCode}, {"correct-password", "123456", nil}} {
		result, err := s.Login(context.Background(), LoginInput{Email: "u@example.com", Password: tc.password, EmailCode: tc.code})
		if !errors.Is(err, tc.want) || ((result.AccessToken != "") != (tc.want == nil)) {
			t.Fatalf("%+v: %+v, %v", tc, result, err)
		}
	}
	if codes.verified != 2 || codes.consumed != 1 {
		t.Fatalf("missing code must not count as a verification attempt: %+v", codes)
	}
}
func TestDefaultLoginDoesNotRequireEmailCode(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}}
	s, c := testAuthentication(t, repo)
	result, e := s.Login(context.Background(), LoginInput{Email: "u@example.com", Password: "correct-password"})
	if e != nil || result.AccessToken == "" || c.verified != 0 {
		t.Fatal(result, e)
	}
}

func TestBootstrapRejectsPreviouslyIssuedSessions(t *testing.T) {
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), BootstrapAdminUserID: "admin"}}
	service, _ := testAuthentication(t, repo)
	ctx := context.Background()
	pair, e := service.base.issuePair(ctx, "user")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.base.Refresh(ctx, pair.RefreshToken); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal("refresh bypassed bootstrap gate", e)
	}
	if _, e = service.base.CurrentUser(ctx, "user"); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal("old access token bypassed bootstrap gate", e)
	}
	if e = service.base.Authorize(ctx, "user", "*"); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal("administration bypassed bootstrap gate", e)
	}
}

type unavailableMailer struct{}

func (unavailableMailer) SendCode(context.Context, string, string, string) error {
	return errors.New("SMTP unavailable")
}
func TestCodeDeliveryFailureAndUnknownMailboxHaveSameOutcome(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}}
	service, codes := testAuthentication(t, repo)
	service.mail = unavailableMailer{}
	service.base.users = failingUsers{err: ErrNotFound}
	for _, email := range []string{"known@example.com", "unknown@example.com"} {
		if e := service.RequestEmailCode(context.Background(), email, "login", "ip"); e != nil {
			t.Fatal("delivery state exposed", e)
		}
	}
	if codes.issued != 2 {
		t.Fatal("credential not stored independently of delivery")
	}
}

type resetDuringLoginUsers struct{ authUsersStub }

func (u resetDuringLoginUsers) FindByID(context.Context, string) (User, error) {
	user := u.user
	user.PasswordHash = "new-password-hash"
	return user, nil
}

type challengeRecorder struct {
	Challenges
	issued int
}

func (c *challengeRecorder) Issue(context.Context, Challenge) (string, error) {
	c.issued++
	return "challenge", nil
}

func TestLoginDoesNotBindOldPasswordToNewSecurityVersion(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}, security: SecurityState{SecuritySettings: SecuritySettings{Mode: "totp", TOTPEnabled: true}, Version: 2}}
	service, _ := testAuthentication(t, repo)
	service.base.users = resetDuringLoginUsers{authUsersStub{user: User{ID: "user", Email: "u@example.com", PasswordHash: "old-password-hash", Active: true}}}
	challenges := &challengeRecorder{}
	service.challenges = challenges
	_, err := service.Login(context.Background(), LoginInput{Email: "u@example.com", Password: "correct-password"})
	if !errors.Is(err, ErrInvalidCredentials) || challenges.issued != 0 {
		t.Fatal("password changed during login but challenge was issued", err)
	}
}
