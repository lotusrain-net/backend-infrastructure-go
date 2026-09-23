package iamstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jyysy/backend-infrastructure-go/pkg/postgres"
)

type AuthenticationStore struct {
	q        *dbgen.Queries
	beginner postgres.Beginner
}

func NewAuthentication(q *dbgen.Queries, b postgres.Beginner) *AuthenticationStore {
	return &AuthenticationStore{q: q, beginner: b}
}
func (s *AuthenticationStore) Authentication(ctx context.Context) (iam.AuthenticationState, error) {
	r, e := s.q.GetAuthenticationSettings(ctx)
	if e != nil {
		return iam.AuthenticationState{}, e
	}
	return iam.AuthenticationState{AuthenticationSettings: iam.AuthenticationSettings{PasswordLoginEnabled: r.PasswordLoginEnabled, RegistrationEnabled: r.RegistrationEnabled, RegistrationEmailVerificationRequired: r.RegistrationEmailVerificationRequired, AllowedEmailDomains: r.AllowedEmailDomains}, BootstrapAdminUserID: uuidString(r.BootstrapAdminUserID), InitializedAt: timePointer(r.InitializedAt)}, nil
}
func (s *AuthenticationStore) PutAuthentication(ctx context.Context, v iam.AuthenticationSettings) error {
	return s.q.PutAuthenticationSettings(ctx, dbgen.PutAuthenticationSettingsParams{PasswordLoginEnabled: v.PasswordLoginEnabled, RegistrationEnabled: v.RegistrationEnabled, RegistrationEmailVerificationRequired: v.RegistrationEmailVerificationRequired, AllowedEmailDomains: v.AllowedEmailDomains})
}
func (s *AuthenticationStore) Initialize(ctx context.Context, id string) error {
	u, e := parseUUID(id)
	if e != nil {
		return e
	}
	n, e := s.q.InitializeAuthentication(ctx, u)
	if e != nil {
		return e
	}
	if n != 1 {
		return iam.ErrInvalidCredentials
	}
	return nil
}
func (s *AuthenticationStore) Security(ctx context.Context, id string) (iam.SecurityState, error) {
	u, e := parseUUID(id)
	if e != nil {
		return iam.SecurityState{}, e
	}
	if e = s.q.EnsureSecuritySettings(ctx, u); e != nil {
		return iam.SecurityState{}, e
	}
	v, e := s.q.GetSecuritySettings(ctx, u)
	if e != nil {
		return iam.SecurityState{}, e
	}
	return securityFromDB(v), nil
}
func securityFromDB(v dbgen.UserSecuritySetting) iam.SecurityState {
	return iam.SecurityState{SecuritySettings: iam.SecuritySettings{RecoveryCodesExpiresAt: timePointer(v.RecoveryCodesExpiresAt), Mode: v.Mode, TOTPEnabled: len(v.TotpSecret) > 0, RecoveryCodesRemaining: len(v.RecoveryHashes)}, Version: v.Version, Secret: v.TotpSecret, PendingSecret: v.PendingSecret, PendingExpiresAt: timePointer(v.PendingExpiresAt), LastTOTPStep: v.LastTotpStep, RecoveryHashes: v.RecoveryHashes}
}
func securityParams(id pgtype.UUID, v iam.SecurityState, version int64) dbgen.SaveSecuritySettingsParams {
	expiry := pgtype.Timestamptz{}
	if v.PendingExpiresAt != nil {
		expiry = pgtype.Timestamptz{Time: *v.PendingExpiresAt, Valid: true}
	}
	recoveryExpiry := pgtype.Timestamptz{}
	if v.RecoveryCodesExpiresAt != nil {
		recoveryExpiry = pgtype.Timestamptz{Time: *v.RecoveryCodesExpiresAt, Valid: true}
	}
	hashes := v.RecoveryHashes
	if hashes == nil {
		hashes = []string{}
	}
	return dbgen.SaveSecuritySettingsParams{RecoveryCodesExpiresAt: recoveryExpiry, UserID: id, Mode: v.Mode, TotpSecret: v.Secret, PendingSecret: v.PendingSecret, PendingExpiresAt: expiry, LastTotpStep: v.LastTOTPStep, RecoveryHashes: hashes, Version: version}
}
func (s *AuthenticationStore) SaveSecurity(ctx context.Context, id string, v iam.SecurityState, version int64) error {
	u, e := parseUUID(id)
	if e != nil {
		return e
	}
	n, e := s.q.SaveSecuritySettings(ctx, securityParams(u, v, version))
	if e != nil {
		return e
	}
	if n != 1 {
		return iam.ErrSecurityConflict
	}
	return nil
}
func (s *AuthenticationStore) Register(ctx context.Context, in iam.CreateUserInput, receipt *iam.CodeReceipt) (iam.User, error) {
	var result iam.User
	e := postgres.RunInTx(ctx, s.beginner, func(ctx context.Context, tx pgx.Tx) error {
		q := dbgen.New(tx)
		settings, e := q.LockAuthenticationSettings(ctx)
		if e != nil {
			return e
		}
		if !settings.InitializedAt.Valid || !settings.RegistrationEnabled {
			return iam.ErrAuthenticationForbidden
		}
		if settings.RegistrationEmailVerificationRequired && receipt == nil {
			return iam.ErrInvalidCode
		}
		if len(settings.AllowedEmailDomains) > 0 {
			allowed := false
			for _, d := range settings.AllowedEmailDomains {
				if strings.HasSuffix(in.Email, "@"+d) {
					allowed = true
				}
			}
			if !allowed {
				return iam.ErrAuthenticationForbidden
			}
		}
		// Verification and registration policy must still match the locked settings.
		if !settings.RegistrationEmailVerificationRequired {
			receipt = nil
		}
		if receipt != nil {
			if !receipt.ExpiresAt.After(time.Now()) {
				return iam.ErrInvalidCode
			}
			if e = q.PruneAuthenticationConsumptions(ctx); e != nil {
				return e
			}
			if e = q.InsertAuthenticationConsumption(ctx, dbgen.InsertAuthenticationConsumptionParams{CredentialID: "email:" + receipt.ID, ExpiresAt: pgtype.Timestamptz{Time: receipt.ExpiresAt, Valid: true}}); e != nil {
				if errors.Is(mapDBError(e), iam.ErrDuplicateIdentity) {
					return iam.ErrInvalidCode
				}
				return e
			}
		}
		user, e := q.CreateUser(ctx, dbgen.CreateUserParams{Email: in.Email, Username: in.Username, PasswordHash: in.Password, DisplayName: in.DisplayName})
		if e != nil {
			return mapDBError(e)
		}
		if receipt != nil {
			user, e = q.MarkEmailVerified(ctx, user.ID)
			if e != nil {
				return e
			}
		}
		role, e := q.GetRoleByName(ctx, "user")
		if e != nil {
			return e
		}
		if e = q.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: user.ID, RoleID: role.ID}); e != nil {
			return e
		}
		result = userFromDB(user)
		return nil
	})
	return result, e
}
func (s *AuthenticationStore) ConsumeCredential(ctx context.Context, id, credential string, version, step int64, recovery string) error {
	u, e := parseUUID(id)
	if e != nil {
		return e
	}
	return postgres.RunInTx(ctx, s.beginner, func(ctx context.Context, tx pgx.Tx) error {
		q := dbgen.New(tx)
		r, e := q.LockSecuritySettings(ctx, u)
		if e != nil {
			return e
		}
		if r.Version != version || len(r.TotpSecret) == 0 {
			return iam.ErrInvalidCode
		}
		if credential != "" {
			hash := sha256.Sum256([]byte(credential))
			if e = q.PruneAuthenticationConsumptions(ctx); e != nil {
				return e
			}
			if e = q.InsertAuthenticationConsumption(ctx, dbgen.InsertAuthenticationConsumptionParams{CredentialID: "challenge:" + hex.EncodeToString(hash[:]), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true}}); e != nil {
				return iam.ErrInvalidCode
			}
		}
		if recovery != "" {
			if !r.RecoveryCodesExpiresAt.Valid || !r.RecoveryCodesExpiresAt.Time.After(time.Now()) {
				return iam.ErrInvalidCode
			}
			found := false
			remaining := make([]string, 0, len(r.RecoveryHashes))
			for _, h := range r.RecoveryHashes {
				if h == recovery {
					found = true
				} else {
					remaining = append(remaining, h)
				}
			}
			if !found {
				return iam.ErrInvalidCode
			}
			r.RecoveryHashes = remaining
		} else {
			if step <= r.LastTotpStep {
				return iam.ErrInvalidCode
			}
			r.LastTotpStep = step
		}
		// Advance the version to invalidate outstanding challenges and stale writes
		// that could otherwise restore an already consumed recovery code.
		e = q.SaveConsumedSecurityCredential(ctx, dbgen.SaveConsumedSecurityCredentialParams{UserID: u, LastTotpStep: r.LastTotpStep, RecoveryHashes: r.RecoveryHashes})
		return e
	})
}
func timePointer(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
