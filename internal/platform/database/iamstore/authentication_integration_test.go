package iamstore_test

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jyysy/backend-infrastructure-go/internal/app"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/iamstore"
	"github.com/jyysy/backend-infrastructure-go/internal/testutil"
)

func TestAuthenticationTransactionsAndBootstrap(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t, 0)
	q := dbgen.New(pool)
	repo := iamstore.NewAuthentication(q, pool)
	seed := app.AdminBootstrap{Email: "bootstrap@example.com", Username: "bootstrap", Password: "strong-bootstrap-password"}
	var wg sync.WaitGroup
	var failures atomic.Int32
	for range 4 {
		wg.Go(func() {
			if app.SeedBootstrapAdmin(ctx, pool, iam.NewPasswordHasher(iam.DefaultArgon2Params()), seed) != nil {
				failures.Add(1)
			}
		})
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatal("seed failures", failures.Load())
	}
	state, e := repo.Authentication(ctx)
	if e != nil || state.BootstrapAdminUserID == "" || state.InitializedAt != nil {
		t.Fatal(state, e)
	}
	other := seed
	other.Email = "second@example.com"
	other.Username = "second"
	if app.SeedBootstrapAdmin(ctx, pool, iam.NewPasswordHasher(iam.DefaultArgon2Params()), other) == nil {
		t.Fatal("second bootstrap accepted")
	}
	for range 8 {
		wg.Go(func() {
			if repo.Initialize(ctx, state.BootstrapAdminUserID) != nil {
				failures.Add(1)
			}
		})
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatal("initialize failures")
	}
	reopened := iamstore.NewAuthentication(dbgen.New(pool), pool)
	state, e = reopened.Authentication(ctx)
	if e != nil || state.InitializedAt == nil {
		t.Fatal("initialization not durable", e)
	}
	settings := iam.DefaultAuthenticationSettings()
	settings.RegistrationEnabled = true
	if e = repo.PutAuthentication(ctx, settings); e != nil {
		t.Fatal(e)
	}
	receipt := &iam.CodeReceipt{ID: "registration-receipt", ExpiresAt: time.Now().Add(time.Minute)}
	in := iam.CreateUserInput{Email: "new@example.com", Username: "new", Password: "hashed"}
	// Missing default role forces rollback after creating the user and consuming the receipt.
	if _, e = pool.Exec(ctx, "UPDATE roles SET name='temporarily-missing-user' WHERE name='user'"); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.Register(ctx, in, receipt); e == nil {
		t.Fatal("missing role accepted")
	}
	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email=$1", in.Email).Scan(&count)
	if count != 0 {
		t.Fatal("partial user")
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM authentication_consumptions WHERE credential_id='email:registration-receipt'").Scan(&count)
	if count != 0 {
		t.Fatal("receipt not rolled back")
	}
	if _, e = pool.Exec(ctx, "UPDATE roles SET name='user' WHERE name='temporarily-missing-user'"); e != nil {
		t.Fatal(e)
	}
	user, e := repo.Register(ctx, in, receipt)
	if e != nil || user.EmailVerifiedAt == nil {
		t.Fatal(user, e)
	}
	in.Email = "replay@example.com"
	in.Username = "replay"
	if _, e = repo.Register(ctx, in, receipt); e == nil {
		t.Fatal("receipt replay")
	}
	security, e := repo.Security(ctx, user.ID)
	if e != nil || security.Mode != "default" {
		t.Fatal(security, e)
	}
	security.Mode = "totp"
	security.Secret = []byte("encrypted")
	security.RecoveryHashes = []string{"hash"}
	expiry := time.Now().Add(time.Hour)
	security.RecoveryCodesExpiresAt = &expiry
	security.LastTOTPStep = -1
	if e = repo.SaveSecurity(ctx, user.ID, security, security.Version); e != nil {
		t.Fatal(e)
	}
	security, _ = repo.Security(ctx, user.ID)
	var consumed atomic.Int32
	for range 8 {
		wg.Go(func() {
			if repo.ConsumeCredential(ctx, user.ID, "challenge", security.Version, -1, "hash") == nil {
				consumed.Add(1)
			}
		})
	}
	wg.Wait()
	if consumed.Load() != 1 {
		t.Fatal("recovery consumed", consumed.Load())
	}
	security, _ = repo.Security(ctx, user.ID)
	if len(security.RecoveryHashes) != 0 {
		t.Fatal("recovery persisted")
	}
}

func TestAuthenticationUpgradePreservesExistingAdministrator(t *testing.T) {
	pool := testutil.Postgres(t, 8)
	ctx := context.Background()
	q := dbgen.New(pool)
	if _, e := pool.Exec(ctx, "INSERT INTO users(email,username,password_hash) VALUES ('legacy@example.com','legacy','legacy-hash'); INSERT INTO user_roles(user_id,role_id) SELECT u.id,r.id FROM users u,roles r WHERE u.email='legacy@example.com' AND r.name='admin'"); e != nil {
		t.Fatal(e)
	}
	// Read the original credential without the new generated column list.
	var id, hash string
	if e := pool.QueryRow(ctx, "SELECT id,password_hash FROM users WHERE email='legacy@example.com'").Scan(&id, &hash); e != nil {
		t.Fatal(e)
	}
	sql, e := os.ReadFile("../../../../db/migrations/000009_authentication.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(sql)); e != nil {
		t.Fatal(e)
	}
	if e := app.SeedBootstrapAdmin(ctx, pool, iam.NewPasswordHasher(iam.DefaultArgon2Params()), app.AdminBootstrap{Email: "legacy@example.com", Username: "legacy", Password: "legacy-password"}); e != nil {
		t.Fatal(e)
	}
	repo := iamstore.NewAuthentication(q, pool)
	state, e := repo.Authentication(ctx)
	if e != nil || state.BootstrapAdminUserID != id || state.InitializedAt != nil {
		t.Fatal(state, e)
	}
	user, e := q.GetUserByEmail(ctx, "legacy@example.com")
	if e != nil || user.PasswordHash != hash {
		t.Fatal("legacy credentials changed", e)
	}
	down, e := os.ReadFile("../../../../db/migrations/000009_authentication.down.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(down)); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(sql)); e != nil {
		t.Fatal(e)
	}
	if e := app.SeedBootstrapAdmin(ctx, pool, iam.NewPasswordHasher(iam.DefaultArgon2Params()), app.AdminBootstrap{Email: "legacy@example.com", Username: "legacy", Password: "legacy-password"}); e != nil {
		t.Fatal(e)
	}
	state, e = repo.Authentication(ctx)
	if e != nil || state.BootstrapAdminUserID != id {
		t.Fatal("upgrade-down-up lost bootstrap", e)
	}
}

func TestExpiredRecoveryCodesCannotBeConsumed(t *testing.T) {
	pool := testutil.Postgres(t, 0)
	ctx := context.Background()
	q := dbgen.New(pool)
	users := iamstore.New(q, pool)
	repo := iamstore.NewAuthentication(q, pool)
	user, e := users.Create(ctx, iam.CreateUserInput{Email: "expired@example.com", Username: "expired", Password: "hash"})
	if e != nil {
		t.Fatal(e)
	}
	security, e := repo.Security(ctx, user.ID)
	if e != nil {
		t.Fatal(e)
	}
	expired := time.Now().Add(-time.Minute)
	security.Mode = "totp"
	security.Secret = []byte("ciphertext")
	security.RecoveryHashes = []string{"hash"}
	security.RecoveryCodesExpiresAt = &expired
	if e = repo.SaveSecurity(ctx, user.ID, security, security.Version); e != nil {
		t.Fatal(e)
	}
	security, e = repo.Security(ctx, user.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.ConsumeCredential(ctx, user.ID, "challenge", security.Version, -1, "hash"); e == nil {
		t.Fatal("expired recovery code accepted")
	}
}
