package iam

import (
	"context"
	"errors"
	"testing"

	"backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeSQLC struct {
	user        dbgen.User
	createErr   error
	permissions []string
}

func (f *fakeSQLC) GetUserByEmail(context.Context, string) (dbgen.User, error) {
	if f.user.Email == "" {
		return dbgen.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}
func (f *fakeSQLC) GetUserByID(context.Context, pgtype.UUID) (dbgen.User, error) {
	if f.user.Email == "" {
		return dbgen.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}
func (f *fakeSQLC) CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error) {
	return f.user, f.createErr
}
func (f *fakeSQLC) SetUserActive(context.Context, dbgen.SetUserActiveParams) error { return nil }
func (f *fakeSQLC) ListUserPermissions(context.Context, pgtype.UUID) ([]string, error) {
	return f.permissions, nil
}
func (f *fakeSQLC) ListRoles(context.Context) ([]dbgen.Role, error)                  { return nil, nil }
func (f *fakeSQLC) ListPermissions(context.Context) ([]dbgen.Permission, error)      { return nil, nil }
func (f *fakeSQLC) AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error { return nil }
func (f *fakeSQLC) GrantRolePermission(context.Context, dbgen.GrantRolePermissionParams) error {
	return nil
}

func TestSQLCRepositoryMapsNoRowsAndDuplicate(t *testing.T) {
	repo := NewSQLCRepository(&fakeSQLC{})
	if _, err := repo.FindByEmail(context.Background(), "missing@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not found=%v", err)
	}
	repo = NewSQLCRepository(&fakeSQLC{createErr: &pgconn.PgError{Code: "23505"}})
	if _, err := repo.Create(context.Background(), CreateUserInput{Email: "a", Username: "a", Password: "hash"}); !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("duplicate=%v", err)
	}
}

func TestSQLCRepositoryRejectsInvalidUUID(t *testing.T) {
	repo := NewSQLCRepository(&fakeSQLC{})
	if _, err := repo.FindByID(context.Background(), "not-a-uuid"); err == nil {
		t.Fatal("invalid UUID accepted")
	}
	if err := repo.AssignRole(context.Background(), "bad", "also-bad"); err == nil {
		t.Fatal("invalid UUIDs accepted")
	}
}

func TestSQLCRepositoryNormalPaths(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a"); err != nil {
		t.Fatal(err)
	}
	sqlc := &fakeSQLC{user: dbgen.User{ID: id, Email: "a@example.com", Username: "alice", IsActive: true}, permissions: []string{"users:read"}}
	repo := NewSQLCRepository(sqlc)
	user, err := repo.FindByID(context.Background(), "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a")
	if err != nil || user.ID != "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if _, err := repo.Create(context.Background(), CreateUserInput{Email: "a@example.com", Username: "alice", Password: "hash"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetActive(context.Background(), user.ID, false); err != nil {
		t.Fatal(err)
	}
	if permissions, err := repo.PermissionsForUser(context.Background(), user.ID); err != nil || len(permissions) != 1 {
		t.Fatalf("permissions=%v err=%v", permissions, err)
	}
	if err := repo.AssignRole(context.Background(), user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.GrantPermission(context.Background(), user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Roles(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Permissions(context.Background()); err != nil {
		t.Fatal(err)
	}
}
