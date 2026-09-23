package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

type deliveryResult struct{ err error }

func (m deliveryResult) SendCode(context.Context, string, EmailCodePurpose, string) error {
	return m.err
}

func TestEmailDeliveryClassifiesServiceFailureWithoutLookingUpAccount(t *testing.T) {
	now := time.Now()
	for _, failure := range []error{nil, ErrMailRejected, ErrAuthenticationUnavailable} {
		repo := &authRepoStub{state: AuthenticationState{InitializedAt: &now}}
		s, codes := testAuthentication(t, repo)
		s.mail = deliveryResult{failure}
		s.base.users = failingUsers{err: errors.New("account lookup must not happen")}
		for _, email := range []string{"known@example.com", "unknown@example.com"} {
			err := s.RequestEmailCode(context.Background(), email, "login", "ip")
			if errors.Is(failure, ErrAuthenticationUnavailable) {
				if !errors.Is(err, ErrAuthenticationUnavailable) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
		if codes.issued != 2 {
			t.Fatal("delivery must follow credential storage")
		}
	}
}

func TestRegistrationReportsFieldErrorsBeforeVerification(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}}
	repo.state.RegistrationEnabled = true
	s, codes := testAuthentication(t, repo)
	_, err := s.Register(context.Background(), RegisterInput{Email: "bad", Password: "short"})
	var fields *FieldValidationError
	if !errors.As(err, &fields) || fields.Fields["email"] == "" || fields.Fields["username"] == "" || fields.Fields["password"] == "" {
		t.Fatalf("missing field errors: %v", err)
	}
	if codes.verified != 0 || repo.registered != 0 {
		t.Fatal("invalid fields reached credential store")
	}
	_, err = s.Register(context.Background(), RegisterInput{Username: "new", Email: "a@example.com", Password: "correct-password", EmailCode: "abc"})
	if !errors.As(err, &fields) || fields.Fields["email_code"] == "" || codes.verified != 0 {
		t.Fatal(err)
	}
	// A disabled verification policy ignores even a supplied optional code.
	repo.state.RegistrationEmailVerificationRequired = false
	if _, err := s.Register(context.Background(), RegisterInput{Username: "new", Email: "a@example.com", Password: "correct-password", EmailCode: "abc"}); err != nil || repo.receipt != nil || codes.verified != 0 {
		t.Fatal(err)
	}
}

func TestEmailRequestAndChallengeValidation(t *testing.T) {
	s, codes := testAuthentication(t, &authRepoStub{})
	err := s.RequestEmailCode(context.Background(), "bad", "other", "ip")
	var fields *FieldValidationError
	if !errors.As(err, &fields) || fields.Fields["email"] == "" || fields.Fields["purpose"] == "" || codes.issued != 0 {
		t.Fatal(err)
	}
	for _, in := range []VerifyInput{{}, {ChallengeID: "id", Code: "abc"}, {ChallengeID: "id", Code: "123456", RecoveryCode: "recovery"}} {
		if _, err := s.VerifyLogin(context.Background(), in); !errors.As(err, &fields) {
			t.Fatalf("invalid shape: %v", err)
		}
	}
}
package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

type deliveryResult struct{ err error }

func (m deliveryResult) SendCode(context.Context, string, EmailCodePurpose, string) error {
	return m.err
}

func TestEmailDeliveryClassifiesServiceFailureWithoutLookingUpAccount(t *testing.T) {
	now := time.Now()
	for _, failure := range []error{nil, ErrMailRejected, ErrAuthenticationUnavailable} {
		repo := &authRepoStub{state: AuthenticationState{InitializedAt: &now}}
		s, codes := testAuthentication(t, repo)
		s.mail = deliveryResult{failure}
		s.base.users = failingUsers{err: errors.New("account lookup must not happen")}
		for _, email := range []string{"known@example.com", "unknown@example.com"} {
			err := s.RequestEmailCode(context.Background(), email, "login", "ip")
			if errors.Is(failure, ErrAuthenticationUnavailable) {
				if !errors.Is(err, ErrAuthenticationUnavailable) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
		if codes.issued != 2 {
			t.Fatal("delivery must follow credential storage")
		}
	}
}

func TestRegistrationReportsFieldErrorsBeforeVerification(t *testing.T) {
	now := time.Now()
	repo := &authRepoStub{state: AuthenticationState{AuthenticationSettings: DefaultAuthenticationSettings(), InitializedAt: &now}}
	repo.state.RegistrationEnabled = true
	s, codes := testAuthentication(t, repo)
	_, err := s.Register(context.Background(), RegisterInput{Email: "bad", Password: "short"})
	var fields *FieldValidationError
	if !errors.As(err, &fields) || fields.Fields["email"] == "" || fields.Fields["username"] == "" || fields.Fields["password"] == "" {
		t.Fatalf("missing field errors: %v", err)
	}
	if codes.verified != 0 || repo.registered != 0 {
		t.Fatal("invalid fields reached credential store")
	}
	_, err = s.Register(context.Background(), RegisterInput{Username: "new", Email: "a@example.com", Password: "correct-password", EmailCode: "abc"})
	if !errors.As(err, &fields) || fields.Fields["email_code"] == "" || codes.verified != 0 {
		t.Fatal(err)
	}
	// A disabled verification policy ignores even a supplied optional code.
	repo.state.RegistrationEmailVerificationRequired = false
	if _, err := s.Register(context.Background(), RegisterInput{Username: "new", Email: "a@example.com", Password: "correct-password", EmailCode: "abc"}); err != nil || repo.receipt != nil || codes.verified != 0 {
		t.Fatal(err)
	}
}

func TestEmailRequestAndChallengeValidation(t *testing.T) {
	s, codes := testAuthentication(t, &authRepoStub{})
	err := s.RequestEmailCode(context.Background(), "bad", "other", "ip")
	var fields *FieldValidationError
	if !errors.As(err, &fields) || fields.Fields["email"] == "" || fields.Fields["purpose"] == "" || codes.issued != 0 {
		t.Fatal(err)
	}
	for _, in := range []VerifyInput{{}, {ChallengeID: "id", Code: "abc"}, {ChallengeID: "id", Code: "123456", RecoveryCode: "recovery"}} {
		if _, err := s.VerifyLogin(context.Background(), in); !errors.As(err, &fields) {
			t.Fatalf("invalid shape: %v", err)
		}
	}
}
