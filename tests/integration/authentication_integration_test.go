package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lotusrain-net/backend-infrastructure-go/internal/app"
	"github.com/lotusrain-net/backend-infrastructure-go/internal/testutil"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/iamstore"
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
	sql, e := os.ReadFile("../../pkg/migrations/000009_authentication.up.sql")
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
	down, e := os.ReadFile("../../pkg/migrations/000009_authentication.down.sql")
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

func TestPasswordUpdateAndCredentialInvalidationAreAtomic(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t, 0)
	q := dbgen.New(pool)
	users := iamstore.New(q, pool)
	repo := iamstore.NewAuthentication(q, pool)
	user, err := users.Create(ctx, iam.CreateUserInput{Email: "atomic@example.com", Username: "atomic", Password: "old-hash"})
	if err != nil {
		t.Fatal(err)
	}
	security, err := repo.Security(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour)
	security.PendingSecret = []byte("pending-ciphertext")
	security.PendingExpiresAt = &expiry
	if err = repo.SaveSecurity(ctx, user.ID, security, security.Version); err != nil {
		t.Fatal(err)
	}
	before, err := repo.Security(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Fail the password write after invalidation, forcing the entire transaction back.
	_, err = pool.Exec(ctx, `CREATE FUNCTION reject_password_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected password write failure'; END $$;
 CREATE TRIGGER reject_password_update BEFORE UPDATE OF password_hash ON users FOR EACH ROW EXECUTE FUNCTION reject_password_update()`)
	if err != nil {
		t.Fatal(err)
	}
	if err = users.UpdateUserPassword(ctx, user.ID, "new-hash"); err == nil {
		t.Fatal("injected failure hidden")
	}
	after, err := repo.Security(ctx, user.ID)
	if err != nil || after.Version != before.Version || string(after.PendingSecret) != string(before.PendingSecret) {
		t.Fatal("invalidation was not rolled back", err)
	}
	unchanged, err := users.FindByID(ctx, user.ID)
	if err != nil || unchanged.PasswordHash != "old-hash" {
		t.Fatal("password was not rolled back", err)
	}
	if _, err = pool.Exec(ctx, "DROP TRIGGER reject_password_update ON users"); err != nil {
		t.Fatal(err)
	}
	if err = users.UpdateUserPassword(ctx, user.ID, "new-hash"); err != nil {
		t.Fatal(err)
	}
	after, err = repo.Security(ctx, user.ID)
	if err != nil || after.Version <= before.Version || len(after.PendingSecret) != 0 || after.PendingExpiresAt != nil {
		t.Fatal("pending enrollment survived reset", err)
	}
	changed, err := users.FindByID(ctx, user.ID)
	if err != nil || changed.PasswordHash != "new-hash" {
		t.Fatal("password not updated", err)
	}
}

func TestEmailPolicyAndProfileChangesSerialize(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t, 0)
	q := dbgen.New(pool)
	users := iamstore.New(q, pool)
	repo := iamstore.NewAuthentication(q, pool)
	for i := range 20 {
		user, err := users.Create(ctx, iam.CreateUserInput{Email: fmt.Sprintf("email-%d@example.com", i), Username: fmt.Sprintf("email-%d", i), Password: "hash"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "UPDATE users SET email_verified_at=NOW() WHERE id=$1", user.ID); err != nil {
			t.Fatal(err)
		}
		security, err := repo.Security(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		security.Mode = "email"
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() { <-start; results <- repo.SaveSecurity(ctx, user.ID, security, security.Version) }()
		go func() {
			<-start
			_, err := users.UpdateUser(ctx, user.ID, iam.UpdateUserInput{Email: fmt.Sprintf("changed-%d@example.com", i), Username: user.Username})
			results <- err
		}()
		close(start)
		successes := 0
		for range 2 {
			err := <-results
			if err == nil {
				successes++
			} else if !errors.Is(err, iam.ErrAuthenticationForbidden) {
				t.Fatal(err)
			}
		}
		if successes != 1 {
			t.Fatalf("policy and address change both succeeded: %d", successes)
		}
		current, err := users.FindByID(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		saved, err := repo.Security(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Mode == "email" && current.EmailVerifiedAt == nil {
			t.Fatal("concurrent operations stranded email login")
		}
	}
}

func TestExistingEmailVerificationTransaction(t *testing.T) {
	ctx := context.Background()
	pool := testutil.Postgres(t, 0)
	q := dbgen.New(pool)
	users := iamstore.New(q, pool)
	repo := iamstore.NewAuthentication(q, pool)
	user, err := users.Create(ctx, iam.CreateUserInput{Email: "old@example.com", Username: "verify", Password: "hashed"})
	if err != nil {
		t.Fatal(err)
	}
	r := iam.CodeReceipt{ID: "verify-receipt", ExpiresAt: time.Now().Add(time.Minute)}
	if _, err = repo.VerifyEmail(ctx, user.ID, "stale@example.com", r); !errors.Is(err, iam.ErrSecurityConflict) {
		t.Fatal("stale email accepted", err)
	}
	user, err = repo.VerifyEmail(ctx, user.ID, user.Email, r)
	if err != nil || user.EmailVerifiedAt == nil {
		t.Fatal(user, err)
	}
	if _, err = repo.VerifyEmail(ctx, user.ID, user.Email, r); !errors.Is(err, iam.ErrInvalidCode) {
		t.Fatal("replay accepted", err)
	}
	user, err = users.UpdateUser(ctx, user.ID, iam.UpdateUserInput{Email: "new@example.com", Username: "verify"})
	if err != nil || user.EmailVerifiedAt != nil {
		t.Fatal(user, err)
	}
	r.ID = "second-receipt"
	if _, err = repo.VerifyEmail(ctx, user.ID, "old@example.com", r); !errors.Is(err, iam.ErrSecurityConflict) {
		t.Fatal("old email accepted", err)
	}
	r.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err = repo.VerifyEmail(ctx, user.ID, user.Email, r); !errors.Is(err, iam.ErrInvalidCode) {
		t.Fatal("expired receipt accepted", err)
	}
	r.ExpiresAt = time.Now().Add(time.Minute)
	var wg sync.WaitGroup
	var successes atomic.Int32
	for range 8 {
		wg.Go(func() {
			if _, e := repo.VerifyEmail(ctx, user.ID, user.Email, r); e == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("concurrent receipt successes", successes.Load())
	}
}
