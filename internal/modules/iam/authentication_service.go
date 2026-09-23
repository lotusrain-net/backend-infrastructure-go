package iam

import (
	"context"
	"errors"
	"strings"
	"time"
)

const DefaultRecoveryCodeTTL = 30 * 24 * time.Hour

type AuthenticationService struct {
	recoveryTTL time.Duration
	base        *Service
	repo        AuthenticationRepository
	codes       EmailCodes
	mail        Mailer
	crypto      SecondFactorCrypto
	challenges  Challenges
	now         func() time.Time
}

func NewAuthenticationService(base *Service, repo AuthenticationRepository, codes EmailCodes, mailer Mailer, crypto SecondFactorCrypto, challenges Challenges, recoveryTTL ...time.Duration) *AuthenticationService {
	service := &AuthenticationService{base: base, repo: repo, codes: codes, mail: mailer, crypto: crypto, challenges: challenges, now: time.Now}
	service.recoveryTTL = DefaultRecoveryCodeTTL
	if len(recoveryTTL) > 0 && recoveryTTL[0] > 0 {
		service.recoveryTTL = recoveryTTL[0]
	}
	base.authentication = service
	return service
}
func (s *AuthenticationService) credentials(ctx context.Context, in LoginInput) (User, error) {
	user, e := s.base.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(in.Email)))
	hash := dummyPasswordHash
	if e == nil {
		hash = user.PasswordHash
	}
	ok, verifyErr := s.base.passwords.Verify(in.Password, hash)
	if e != nil && !errors.Is(e, ErrNotFound) {
		return User{}, e
	}
	if e != nil || verifyErr != nil || !ok || !user.Active {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}
func (s *AuthenticationService) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	state, e := s.repo.Authentication(ctx)
	if e != nil {
		return LoginResult{}, e
	}
	user, e := s.credentials(ctx, in)
	if e != nil {
		return LoginResult{}, e
	}
	if state.InitializedAt == nil {
		if state.BootstrapAdminUserID == "" || user.ID != state.BootstrapAdminUserID {
			return LoginResult{}, ErrInvalidCredentials
		}
		pair, e := s.base.issuePair(ctx, user.ID)
		if e != nil {
			return LoginResult{}, e
		}
		if e = s.repo.Initialize(ctx, user.ID); e != nil {
			_ = s.base.Logout(ctx, pair.RefreshToken)
			return LoginResult{}, e
		}
		return LoginResult{TokenPair: pair}, nil
	}
	if !state.PasswordLoginEnabled {
		return LoginResult{}, ErrInvalidCredentials
	}
	security, e := s.repo.Security(ctx, user.ID)
	if e != nil {
		return LoginResult{}, e
	}
	switch security.Mode {
	case "email":
		if user.EmailVerifiedAt == nil {
			return LoginResult{}, ErrInvalidCredentials
		}
		if in.EmailCode == "" {
			return LoginResult{}, ErrEmailCodeRequired
		}
		receipt, e := s.codes.Verify(ctx, user.Email, "login", in.EmailCode)
		if e != nil {
			return LoginResult{}, e
		}
		if e = s.codes.Consume(ctx, user.Email, "login", receipt); e != nil {
			return LoginResult{}, e
		}
	case "totp":
		if !security.TOTPEnabled {
			return LoginResult{}, ErrInvalidCredentials
		}
		// A reset may have committed between password verification and reading
		// the security version. Never bind old credentials to the new version.
		current, e := s.base.users.FindByID(ctx, user.ID)
		if e != nil {
			return LoginResult{}, e
		}
		if !current.Active || current.PasswordHash != user.PasswordHash {
			return LoginResult{}, ErrInvalidCredentials
		}
		challenge, e := s.challenges.Issue(ctx, Challenge{UserID: user.ID, Version: security.Version})
		if e != nil {
			return LoginResult{}, e
		}
		return LoginResult{Status: "totp_required", ChallengeID: challenge, ChallengeExpiresIn: 300}, nil
	}
	pair, e := s.base.issuePair(ctx, user.ID)
	return LoginResult{TokenPair: pair}, e
}
func (s *AuthenticationService) VerifyLogin(ctx context.Context, in VerifyInput) (TokenPair, error) {
	if (in.Code == "") == (in.RecoveryCode == "") {
		return TokenPair{}, ErrInvalidCode
	}
	challenge, e := s.challenges.Attempt(ctx, in.ChallengeID)
	if e != nil {
		return TokenPair{}, e
	}
	user, e := s.base.users.FindByID(ctx, challenge.UserID)
	if e != nil || !user.Active {
		return TokenPair{}, ErrInvalidCredentials
	}
	state, e := s.repo.Authentication(ctx)
	if e != nil {
		return TokenPair{}, e
	}
	if state.InitializedAt == nil || !state.PasswordLoginEnabled {
		return TokenPair{}, ErrInvalidCredentials
	}
	security, e := s.repo.Security(ctx, user.ID)
	if e != nil {
		return TokenPair{}, e
	}
	if security.Mode != "totp" || !security.TOTPEnabled || security.Version != challenge.Version {
		return TokenPair{}, ErrInvalidCode
	}
	if e = s.consumeProof(ctx, user.ID, security, in.ChallengeID, in.Code, in.RecoveryCode); e != nil {
		return TokenPair{}, e
	}
	if e = s.challenges.Consume(ctx, in.ChallengeID); e != nil {
		return TokenPair{}, e
	}
	return s.base.issuePair(ctx, user.ID)
}
func (s *AuthenticationService) RequestEmailCode(ctx context.Context, email, purpose, ip string) error {
	state, e := s.repo.Authentication(ctx)
	if e != nil {
		return e
	}
	if purpose == "register" && (state.InitializedAt == nil || !state.RegistrationEnabled) {
		return ErrAuthenticationForbidden
	}
	if purpose != "register" && purpose != "login" {
		return ErrInvalidUserInput
	}
	email, e = normalizeEmail(email)
	if e != nil {
		return e
	}
	code, e := s.codes.Issue(ctx, email, purpose, ip)
	if e != nil {
		return e
	}
	// Delivery does not look up users: existing and unknown addresses take the
	// same transport path, including before initialization. Login still checks
	// credentials, initialization and the user's policy before accepting a code.
	_ = s.mail.SendCode(ctx, email, purpose, code)
	return nil
}
func (s *AuthenticationService) Register(ctx context.Context, in RegisterInput) (User, error) {
	state, e := s.repo.Authentication(ctx)
	if e != nil {
		return User{}, e
	}
	if state.InitializedAt == nil || !state.RegistrationEnabled {
		return User{}, ErrAuthenticationForbidden
	}
	email, e := normalizeEmail(in.Email)
	if e != nil {
		return User{}, e
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) == 0 || len(in.Username) > 100 || len(in.Password) < 12 || len(in.Password) > 1024 {
		return User{}, ErrInvalidUserInput
	}
	if len(state.AllowedEmailDomains) > 0 {
		domain := strings.SplitN(email, "@", 2)[1]
		allowed := false
		for _, d := range state.AllowedEmailDomains {
			if strings.EqualFold(d, domain) {
				allowed = true
			}
		}
		if !allowed {
			return User{}, ErrAuthenticationForbidden
		}
	}
	var receipt *CodeReceipt
	if state.RegistrationEmailVerificationRequired {
		if in.EmailCode == "" {
			return User{}, ErrEmailCodeRequired
		}
		r, e := s.codes.Verify(ctx, email, "register", in.EmailCode)
		if e != nil {
			return User{}, e
		}
		receipt = &r
	}
	hash, e := s.base.passwords.Hash(in.Password)
	if e != nil {
		return User{}, e
	}
	user, e := s.repo.Register(ctx, CreateUserInput{Email: email, Username: in.Username, Password: hash}, receipt)
	if e != nil {
		return User{}, e
	}
	if receipt != nil {
		_ = s.codes.Consume(ctx, email, "register", *receipt)
	} // PostgreSQL receipt is authoritative even if Redis cleanup fails.
	return user, nil
}
func (s *AuthenticationService) Settings(ctx context.Context) (AuthenticationSettings, error) {
	v, e := s.repo.Authentication(ctx)
	return v.AuthenticationSettings, e
}
func (s *AuthenticationService) PutSettings(ctx context.Context, v AuthenticationSettings) error {
	if e := v.Validate(); e != nil {
		return e
	}
	for i := range v.AllowedEmailDomains {
		v.AllowedEmailDomains[i] = strings.ToLower(v.AllowedEmailDomains[i])
	}
	if v.AllowedEmailDomains == nil {
		v.AllowedEmailDomains = []string{}
	}
	return s.repo.PutAuthentication(ctx, v)
}
func (s *AuthenticationService) Security(ctx context.Context, id string) (SecuritySettings, error) {
	if e := s.base.requireActiveUser(ctx, id); e != nil {
		return SecuritySettings{}, e
	}
	v, e := s.repo.Security(ctx, id)
	if v.RecoveryCodesExpiresAt == nil || !v.RecoveryCodesExpiresAt.After(s.now()) {
		v.RecoveryCodesRemaining = 0
	}
	return v.SecuritySettings, e
}
func (s *AuthenticationService) PutSecurity(ctx context.Context, id, mode string) (SecuritySettings, error) {
	if e := s.base.requireInitialized(ctx); e != nil {
		return SecuritySettings{}, e
	}
	user, e := s.base.users.FindByID(ctx, id)
	if e != nil {
		return SecuritySettings{}, e
	}
	if !user.Active {
		return SecuritySettings{}, ErrInvalidCredentials
	}
	security, e := s.repo.Security(ctx, id)
	if e != nil {
		return SecuritySettings{}, e
	}
	if mode != "default" && mode != "email" && mode != "totp" {
		return SecuritySettings{}, ErrInvalidUserInput
	}
	if security.TOTPEnabled && mode != "totp" {
		return SecuritySettings{}, ErrAuthenticationForbidden
	} // Disable requires a fresh second-factor proof.
	if mode == "email" && user.EmailVerifiedAt == nil {
		return SecuritySettings{}, ErrAuthenticationForbidden
	}
	if mode == "totp" && !security.TOTPEnabled {
		return SecuritySettings{}, ErrAuthenticationForbidden
	}
	security.Mode = mode
	e = s.repo.SaveSecurity(ctx, id, security, security.Version)
	return security.SecuritySettings, e
}
func (s *AuthenticationService) Enroll(ctx context.Context, id string, proof SecurityProof) (Enrollment, error) {
	if e := s.base.requireInitialized(ctx); e != nil {
		return Enrollment{}, e
	}
	user, e := s.base.users.FindByID(ctx, id)
	if e != nil {
		return Enrollment{}, e
	}
	if !user.Active {
		return Enrollment{}, ErrInvalidCredentials
	}
	ok, e := s.base.passwords.Verify(proof.Password, user.PasswordHash)
	if e != nil || !ok {
		return Enrollment{}, ErrInvalidCredentials
	}
	security, e := s.repo.Security(ctx, id)
	if e != nil {
		return Enrollment{}, e
	}
	if security.TOTPEnabled {
		return Enrollment{}, ErrSecurityConflict
	}
	secret, uri, e := s.crypto.NewSecret(user.Email)
	if e != nil {
		return Enrollment{}, e
	}
	encrypted, e := s.crypto.Encrypt(secret, id)
	if e != nil {
		return Enrollment{}, e
	}
	expiry := s.now().Add(10 * time.Minute)
	security.PendingSecret = encrypted
	security.PendingExpiresAt = &expiry
	if e = s.repo.SaveSecurity(ctx, id, security, security.Version); e != nil {
		return Enrollment{}, e
	}
	return Enrollment{Secret: secret, OTPAuthURI: uri, ExpiresIn: 600}, nil
}
func (s *AuthenticationService) Confirm(ctx context.Context, id, code string) ([]string, error) {
	if e := s.base.requireActiveUser(ctx, id); e != nil {
		return nil, e
	}
	security, e := s.repo.Security(ctx, id)
	if e != nil {
		return nil, e
	}
	if security.TOTPEnabled || security.PendingExpiresAt == nil || !security.PendingExpiresAt.After(s.now()) {
		return nil, ErrInvalidCode
	}
	secret, e := s.crypto.Decrypt(security.PendingSecret, id)
	if e != nil {
		return nil, e
	}
	step, e := s.crypto.Verify(secret, code, s.now(), -1)
	if e != nil {
		return nil, e
	}
	codes, hashes, e := s.crypto.RecoveryCodes()
	if e != nil {
		return nil, e
	}
	security.Secret = security.PendingSecret
	security.PendingSecret = nil
	security.PendingExpiresAt = nil
	expiry := s.now().Add(s.recoveryTTL)
	security.RecoveryCodesExpiresAt = &expiry
	security.RecoveryHashes = hashes
	security.RecoveryCodesRemaining = len(hashes)
	security.TOTPEnabled = true
	security.Mode = "totp"
	security.LastTOTPStep = step
	if e = s.repo.SaveSecurity(ctx, id, security, security.Version); e != nil {
		return nil, e
	}
	return codes, nil
}
func (s *AuthenticationService) consumeProof(ctx context.Context, id string, security SecurityState, credential, code, recovery string) error {
	if (code == "") == (recovery == "") {
		return ErrInvalidCode
	}
	step := int64(-1)
	hash := ""
	if code != "" {
		secret, e := s.crypto.Decrypt(security.Secret, id)
		if e != nil {
			return e
		}
		step, e = s.crypto.Verify(secret, code, s.now(), security.LastTOTPStep)
		if e != nil {
			return e
		}
	} else {
		if security.RecoveryCodesExpiresAt == nil || !security.RecoveryCodesExpiresAt.After(s.now()) {
			return ErrInvalidCode
		}
		hash = s.crypto.RecoveryHash(recovery)
		found := false
		for _, h := range security.RecoveryHashes {
			if h == hash {
				found = true
			}
		}
		if !found {
			return ErrInvalidCode
		}
	}
	return s.repo.ConsumeCredential(ctx, id, credential, security.Version, step, hash)
}
func (s *AuthenticationService) Disable(ctx context.Context, id string, proof SecurityProof) error {
	if e := s.base.requireInitialized(ctx); e != nil {
		return e
	}
	user, e := s.base.users.FindByID(ctx, id)
	if e != nil {
		return e
	}
	if !user.Active {
		return ErrInvalidCredentials
	}
	ok, e := s.base.passwords.Verify(proof.Password, user.PasswordHash)
	if e != nil || !ok {
		return ErrInvalidCredentials
	}
	security, e := s.repo.Security(ctx, id)
	if e != nil {
		return e
	}
	if !security.TOTPEnabled {
		// A previous attempt may have saved the disabled state before Redis
		// revocation failed. A retry must still complete session revocation.
		return s.base.RevokeAll(ctx, id)
	} // Idempotent only after password verification.
	if e = s.consumeProof(ctx, id, security, "", proof.Code, proof.RecoveryCode); e != nil {
		return e
	}
	security.Version++ // The consumed factor advances the version under a row lock.
	security.Secret = nil
	security.PendingSecret = nil
	security.PendingExpiresAt = nil
	security.RecoveryCodesExpiresAt = nil
	security.RecoveryHashes = []string{}
	security.RecoveryCodesRemaining = 0
	security.TOTPEnabled = false
	security.Mode = "default"
	security.LastTOTPStep = -1
	if e = s.repo.SaveSecurity(ctx, id, security, security.Version); e != nil {
		return e
	}
	return s.base.RevokeAll(ctx, id)
}
