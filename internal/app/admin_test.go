package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
)

type adminQueriesStub struct {
	user      dbgen.User
	getErr    error
	createErr error
	getUser   func() (dbgen.User, error)
	created   int
	assigned  int
}

func (s *adminQueriesStub) GetUserByEmail(context.Context, string) (dbgen.User, error) {
	if s.getUser != nil {
		return s.getUser()
	}
	return s.user, s.getErr
}
func (s *adminQueriesStub) CreateUser(_ context.Context, p dbgen.CreateUserParams) (dbgen.User, error) {
	s.created++
	if s.createErr != nil {
		return dbgen.User{}, s.createErr
	}
	s.user = dbgen.User{ID: pgtype.UUID{Valid: true}, Email: p.Email, Username: p.Username, PasswordHash: p.PasswordHash, IsActive: true}
	return s.user, nil
}
func (s *adminQueriesStub) GetRoleByName(context.Context, string) (dbgen.Role, error) {
	return dbgen.Role{ID: pgtype.UUID{Valid: true}, Name: "admin"}, nil
}
func (s *adminQueriesStub) AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error {
	s.assigned++
	return nil
}

func TestBootstrapAdminCreatesOnceAndAlwaysEnsuresRole(t *testing.T) {
	queries := &adminQueriesStub{getErr: pgx.ErrNoRows}
	input := AdminBootstrap{Email: "admin@example.com", Username: "admin", Password: "long-admin-password"}
	hasher := iam.NewPasswordHasher(iam.DefaultArgon2Params())
	if err := BootstrapAdmin(context.Background(), queries, hasher, input); err != nil {
		t.Fatal(err)
	}
	queries.getErr = nil
	if err := BootstrapAdmin(context.Background(), queries, hasher, input); err != nil {
		t.Fatal(err)
	}
	if queries.created != 1 || queries.assigned != 2 {
		t.Fatalf("created=%d assigned=%d", queries.created, queries.assigned)
	}
	if queries.user.PasswordHash == input.Password {
		t.Fatal("plain password persisted")
	}
}
func TestBootstrapAdminPropagatesDatabaseFailure(t *testing.T) {
	want := errors.New("database down")
	err := BootstrapAdmin(context.Background(), &adminQueriesStub{getErr: want}, iam.NewPasswordHasher(iam.DefaultArgon2Params()), AdminBootstrap{Email: "a@b.com", Username: "admin", Password: "long-admin-password"})
	if !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestBootstrapAdminRecoversConcurrentInsert(t *testing.T) {
	queries := &adminQueriesStub{
		user:      dbgen.User{ID: pgtype.UUID{Valid: true}, Email: "admin@example.com", Username: "admin", IsActive: true},
		getErr:    pgx.ErrNoRows,
		createErr: &pgconn.PgError{Code: "23505"},
	}
	queriesAfterConflict := false
	queries.getUser = func() (dbgen.User, error) {
		if queriesAfterConflict {
			return queries.user, nil
		}
		queriesAfterConflict = true
		return dbgen.User{}, pgx.ErrNoRows
	}

	err := BootstrapAdmin(context.Background(), queries, iam.NewPasswordHasher(iam.DefaultArgon2Params()), AdminBootstrap{
		Email: "admin@example.com", Username: "admin", Password: "long-admin-password",
	})
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v, want concurrent insert recovery", err)
	}
	if queries.assigned != 1 {
		t.Fatalf("assigned = %d, want 1", queries.assigned)
	}
}
