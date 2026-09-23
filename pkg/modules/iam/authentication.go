package iam

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var ErrAuthenticationForbidden = errors.New("operation not permitted")
var ErrInvalidCode = errors.New("invalid or expired credential")
var ErrEmailCodeRequired = errors.New("email verification required")
var ErrSecurityConflict = errors.New("security settings changed; retry")
var ErrAuthenticationUnavailable = errors.New("authentication service unavailable")
var ErrMailRejected = errors.New("message rejected")

type EmailCodePurpose string

const (
	EmailCodeRegister EmailCodePurpose = "register"
	EmailCodeLogin    EmailCodePurpose = "login"
)

// FieldValidationError contains public field names and safe validation messages only.
type FieldValidationError struct{ Fields map[string]string }

func (e *FieldValidationError) Error() string { return "validation failed" }

var emailCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

type RateLimitError struct{ RetryAfter int }

func (e *RateLimitError) Error() string { return "too many requests" }

type AuthenticationSettings struct {
	PasswordLoginEnabled                  bool     `json:"password_login_enabled"`
	RegistrationEnabled                   bool     `json:"registration_enabled"`
	RegistrationEmailVerificationRequired bool     `json:"registration_email_verification_required"`
	AllowedEmailDomains                   []string `json:"allowed_email_domains"`
}
type AuthenticationState struct {
	AuthenticationSettings
	BootstrapAdminUserID string     `json:"-"`
	InitializedAt        *time.Time `json:"-"`
}

func DefaultAuthenticationSettings() AuthenticationSettings {
	return AuthenticationSettings{PasswordLoginEnabled: true, RegistrationEmailVerificationRequired: true, AllowedEmailDomains: []string{}}
}

var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

func (s AuthenticationSettings) Validate() error {
	if len(s.AllowedEmailDomains) > 100 {
		return ErrInvalidUserInput
	}
	for _, d := range s.AllowedEmailDomains {
		if len(d) > 253 || !domainPattern.MatchString(d) {
			return ErrInvalidUserInput
		}
	}
	return nil
}
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 {
		return "", ErrInvalidUserInput
	}
	return email, nil
}

type SecuritySettings struct {
	RecoveryCodesExpiresAt *time.Time `json:"recovery_codes_expires_at"`
	Mode                   string     `json:"mode"`
	TOTPEnabled            bool       `json:"totp_enabled"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
}
type SecurityState struct {
	SecuritySettings
	Version          int64
	Secret           []byte
	PendingSecret    []byte
	PendingExpiresAt *time.Time
	LastTOTPStep     int64
	RecoveryHashes   []string
}
type LoginInput struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	EmailCode string `json:"email_code,omitempty"`
}
type LoginResult struct {
	TokenPair
	Status             string `json:"status,omitempty"`
	ChallengeID        string `json:"challenge_id,omitempty"`
	ChallengeExpiresIn int64  `json:"-"`
}
type RegisterInput struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	EmailCode string `json:"email_code,omitempty"`
}
type VerifyInput struct {
	ChallengeID  string `json:"challenge_id"`
	Code         string `json:"code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}
type SecurityProof struct {
	Password     string `json:"password"`
	Code         string `json:"code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}
type Enrollment struct {
	Secret     string `json:"secret"`
	OTPAuthURI string `json:"otpauth_uri"`
	ExpiresIn  int    `json:"expires_in"`
}
type CodeReceipt struct {
	ID        string
	ExpiresAt time.Time
}
type Mailer interface {
	SendCode(context.Context, string, EmailCodePurpose, string) error
}
type VerificationCodeStore interface {
	Issue(context.Context, string, EmailCodePurpose, string) (string, error)
	// Verify returns a receipt for the PostgreSQL registration transaction.
	Verify(context.Context, string, EmailCodePurpose, string) (CodeReceipt, error)
	// VerifyAndConsume atomically accepts an email login code exactly once.
	VerifyAndConsume(context.Context, string, EmailCodePurpose, string) error
	// Consume cleans up a committed registration receipt without deleting a newer code.
	Consume(context.Context, string, EmailCodePurpose, CodeReceipt) error
}
type SecondFactorCrypto interface {
	NewSecret(string) (string, string, error)
	Encrypt(string, string) ([]byte, error)
	Decrypt([]byte, string) (string, error)
	Verify(string, string, time.Time, int64) (int64, error)
	RecoveryCodes() ([]string, []string, error)
	RecoveryHash(string) string
}
type Challenge struct {
	UserID  string
	Version int64
}
type Challenges interface {
	Issue(context.Context, Challenge) (string, error)
	Attempt(context.Context, string) (Challenge, error)
	Consume(context.Context, string) error
}
type AuthenticationRepository interface {
	Authentication(context.Context) (AuthenticationState, error)
	PutAuthentication(context.Context, AuthenticationSettings) error
	Initialize(context.Context, string) error
	Security(context.Context, string) (SecurityState, error)
	SaveSecurity(context.Context, string, SecurityState, int64) error
	Register(context.Context, CreateUserInput, *CodeReceipt) (User, error)
	ConsumeCredential(context.Context, string, string, int64, int64, string) error
}
type AuthenticationApplication interface {
	Login(context.Context, LoginInput) (LoginResult, error)
	VerifyLogin(context.Context, VerifyInput) (TokenPair, error)
	RequestEmailCode(context.Context, string, string, string) error
	Register(context.Context, RegisterInput) (User, error)
	Settings(context.Context) (AuthenticationSettings, error)
	PutSettings(context.Context, AuthenticationSettings) error
	Security(context.Context, string) (SecuritySettings, error)
	PutSecurity(context.Context, string, string) (SecuritySettings, error)
	Enroll(context.Context, string, SecurityProof) (Enrollment, error)
	Confirm(context.Context, string, string) ([]string, error)
	Disable(context.Context, string, SecurityProof) error
}
