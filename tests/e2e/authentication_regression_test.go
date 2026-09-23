package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lotusrain-net/backend-infrastructure-go/internal/testutil"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcache"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/authcrypto"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/iamstore"
	"github.com/redis/go-redis/v9"
)

type securityFixture struct {
	pool    *pgxpool.Pool
	users   *iamstore.Store
	repo    *iamstore.AuthenticationStore
	base    *iam.Service
	auth    *iam.AuthenticationService
	redis   *miniredis.Miniredis
	user    iam.User
	session iam.TokenPair
	secret  string
	now     time.Time
}

func newSecurityFixture(t *testing.T) securityFixture {
	t.Helper()
	ctx := context.Background()
	pool := testutil.Postgres(t, 0)
	q := dbgen.New(pool)
	users := iamstore.New(q, pool)
	repo := iamstore.NewAuthentication(q, pool)
	passwords := iam.NewPasswordHasher(iam.DefaultArgon2Params())
	hash, err := passwords.Hash("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := users.Create(ctx, iam.CreateUserInput{Email: "regression@example.com", Username: "regression", Password: hash})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE authentication_settings SET initialized_at=NOW(); UPDATE users SET email_verified_at=NOW()"); err != nil {
		t.Fatal(err)
	}
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	jwt, err := iam.NewJWTManager(bytes.Repeat([]byte{1}, 32), "regression", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	base := iam.NewServiceWithManagement(users, users, users, users, passwords, jwt, iam.NewRefreshStore(refreshAdapter{client}, "regression", time.Hour))
	codes := authcache.New(client, "regression", bytes.Repeat([]byte{2}, 32))
	crypto, err := authcrypto.New(bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{2}, 32), "Test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	auth := iam.NewAuthenticationService(base, repo, codes, &recordingMailer{codes: map[string]string{}}, fixedCrypto{crypto, now}, codes.Challenges())
	session, err := base.Login(ctx, user.Email, "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	secret, _, err := crypto.NewSecret(user.Email)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := crypto.Encrypt(secret, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	security, err := repo.Security(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	expiry := now.Add(time.Hour)
	security.Mode, security.Secret, security.LastTOTPStep = "totp", encrypted, -1
	security.RecoveryHashes = []string{crypto.RecoveryHash("recovery-one"), crypto.RecoveryHash("recovery-two")}
	security.RecoveryCodesExpiresAt = &expiry
	if err = repo.SaveSecurity(ctx, user.ID, security, security.Version); err != nil {
		t.Fatal(err)
	}
	return securityFixture{pool, users, repo, base, auth, server, user, session, secret, now}
}

func TestPasswordResetInvalidatesOutstandingChallenges(t *testing.T) {
	for _, factor := range []string{"totp", "recovery"} {
		t.Run(factor, func(t *testing.T) {
			f := newSecurityFixture(t)
			ctx := context.Background()
			login, err := f.auth.Login(ctx, iam.LoginInput{Email: f.user.Email, Password: "correct-password"})
			if err != nil {
				t.Fatal(err)
			}
			if err = f.base.ResetUserPassword(ctx, f.user.ID, "new-correct-password"); err != nil {
				t.Fatal(err)
			}
			proof := iam.VerifyInput{ChallengeID: login.ChallengeID, Code: totpAt(f.secret, f.now)}
			if factor == "recovery" {
				proof.Code = ""
				proof.RecoveryCode = "recovery-one"
			}
			if pair, err := f.auth.VerifyLogin(ctx, proof); !errors.Is(err, iam.ErrInvalidCode) || pair.AccessToken != "" {
				t.Fatalf("old challenge survived reset: %+v, %v", pair, err)
			}
			if _, err = f.base.Refresh(ctx, f.session.RefreshToken); err == nil {
				t.Fatal("old session survived reset")
			}
			if _, err = f.auth.Login(ctx, iam.LoginInput{Email: f.user.Email, Password: "correct-password"}); !errors.Is(err, iam.ErrInvalidCredentials) {
				t.Fatal("old password accepted", err)
			}
			login, err = f.auth.Login(ctx, iam.LoginInput{Email: f.user.Email, Password: "new-correct-password"})
			if err != nil {
				t.Fatal(err)
			}
			proof.ChallengeID = login.ChallengeID
			if pair, err := f.auth.VerifyLogin(ctx, proof); err != nil || pair.AccessToken == "" {
				t.Fatal("new challenge rejected", err)
			}
		})
	}
}

func TestDisableRetryCompletesSessionRevocation(t *testing.T) {
	f := newSecurityFixture(t)
	ctx := context.Background()
	proof := iam.SecurityProof{Password: "correct-password", RecoveryCode: "recovery-one"}
	f.redis.SetError("ERR injected revocation failure")
	if err := f.auth.Disable(ctx, f.user.ID, proof); err == nil {
		t.Fatal("revocation failure hidden")
	}
	security, err := f.repo.Security(ctx, f.user.ID)
	if err != nil || security.TOTPEnabled {
		t.Fatal("disable was not committed", err)
	}
	f.redis.SetError("")
	if err = f.auth.Disable(ctx, f.user.ID, iam.SecurityProof{Password: "wrong-password"}); !errors.Is(err, iam.ErrInvalidCredentials) {
		t.Fatal(err)
	}
	// The failed attempt left the refresh token usable; the retry must revoke it.
	pair, err := f.base.Refresh(ctx, f.session.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.auth.Disable(ctx, f.user.ID, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = f.base.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Fatal("retry reported success but left session usable")
	}
}

func TestEmailFactorRejectsAddressChange(t *testing.T) {
	f := newSecurityFixture(t)
	ctx := context.Background()
	if err := f.auth.Disable(ctx, f.user.ID, iam.SecurityProof{Password: "correct-password", RecoveryCode: "recovery-one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.PutSecurity(ctx, f.user.ID, "email"); err != nil {
		t.Fatal(err)
	}
	input := iam.UpdateUserInput{Email: "changed@example.com", Username: f.user.Username, DisplayName: "changed"}
	if _, err := f.base.UpdateUser(ctx, f.user.ID, input); !errors.Is(err, iam.ErrAuthenticationForbidden) {
		t.Fatalf("unsafe email change accepted: %v", err)
	}
	user, err := f.users.FindByID(ctx, f.user.ID)
	if err != nil || user.Email != f.user.Email || user.EmailVerifiedAt == nil {
		t.Fatal("rejected update changed user", user, err)
	}
	input.Email = f.user.Email
	if _, err = f.base.UpdateUser(ctx, f.user.ID, input); err != nil {
		t.Fatal("same-email update rejected", err)
	}
	if _, err = f.auth.PutSecurity(ctx, f.user.ID, "default"); err != nil {
		t.Fatal(err)
	}
	input.Email = "changed@example.com"
	user, err = f.base.UpdateUser(ctx, f.user.ID, input)
	if err != nil || user.EmailVerifiedAt != nil {
		t.Fatal("default-mode email update failed", user, err)
	}
	if _, err = f.auth.PutSecurity(ctx, f.user.ID, "email"); !errors.Is(err, iam.ErrAuthenticationForbidden) {
		t.Fatal("unverified email factor accepted", err)
	}
}
