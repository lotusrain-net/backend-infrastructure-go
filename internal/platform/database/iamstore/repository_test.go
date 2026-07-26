package iamstore

import (
	"context"
	"errors"
	"testing"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeQueries struct {
	user        dbgen.User
	users       []dbgen.User
	total       int64
	createErr   error
	permissions []string
	setRows     int64
	assignErr   error
	grantErr    error
}

func (f *fakeQueries) GetUserByEmail(context.Context, string) (dbgen.User, error) {
	if f.user.Email == "" {
		return dbgen.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}
func (f *fakeQueries) GetUserByID(context.Context, pgtype.UUID) (dbgen.User, error) {
	if f.user.Email == "" {
		return dbgen.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}
func (f *fakeQueries) ListUsers(context.Context, dbgen.ListUsersParams) ([]dbgen.User, error) {
	return f.users, nil
}
func (f *fakeQueries) CountUsers(context.Context, dbgen.CountUsersParams) (int64, error) {
	return f.total, nil
}
func (f *fakeQueries) CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error) {
	return f.user, f.createErr
}
func (f *fakeQueries) SetUserActive(context.Context, dbgen.SetUserActiveParams) (int64, error) {
	return f.setRows, nil
}
func (f *fakeQueries) ListUserPermissions(context.Context, pgtype.UUID) ([]string, error) {
	return f.permissions, nil
}
func (f *fakeQueries) ListRoles(context.Context) ([]dbgen.Role, error)             { return nil, nil }
func (f *fakeQueries) ListPermissions(context.Context) ([]dbgen.Permission, error) { return nil, nil }
func (f *fakeQueries) AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error {
	return f.assignErr
}
func (f *fakeQueries) GrantRolePermission(context.Context, dbgen.GrantRolePermissionParams) error {
	return f.grantErr
}

func TestStoreMapsMissingMutationTargets(t *testing.T) {
	store := New(&fakeQueries{})
	if err := store.SetActive(context.Background(), "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a", false); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("SetActive() error = %v", err)
	}
	foreignKeyError := &pgconn.PgError{Code: "23503"}
	store = New(&fakeQueries{assignErr: foreignKeyError, grantErr: foreignKeyError})
	if err := store.AssignRole(context.Background(), "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a", "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a"); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("AssignRole() error = %v", err)
	}
	if err := store.GrantPermission(context.Background(), "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a", "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a"); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("GrantPermission() error = %v", err)
	}
}

func TestStoreMapsNoRowsAndDuplicate(t *testing.T) {
	store := New(&fakeQueries{})
	if _, err := store.FindByEmail(context.Background(), "missing@example.com"); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("not found=%v", err)
	}
	store = New(&fakeQueries{createErr: &pgconn.PgError{Code: "23505"}})
	if _, err := store.Create(context.Background(), iam.CreateUserInput{Email: "a", Username: "a", Password: "hash"}); !errors.Is(err, iam.ErrDuplicateIdentity) {
		t.Fatalf("duplicate=%v", err)
	}
}

func TestStoreRejectsInvalidUUID(t *testing.T) {
	store := New(&fakeQueries{})
	if _, err := store.FindByID(context.Background(), "not-a-uuid"); err == nil {
		t.Fatal("invalid UUID accepted")
	}
	if err := store.AssignRole(context.Background(), "bad", "also-bad"); err == nil {
		t.Fatal("invalid UUIDs accepted")
	}
}

func TestStoreNormalPaths(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a"); err != nil {
		t.Fatal(err)
	}
	queries := &fakeQueries{user: dbgen.User{ID: id, Email: "a@example.com", Username: "alice", IsActive: true}, permissions: []string{"users:read"}, setRows: 1}
	store := New(queries)
	user, err := store.FindByID(context.Background(), "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a")
	if err != nil || user.ID != "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	if _, err := store.Create(context.Background(), iam.CreateUserInput{Email: "a@example.com", Username: "alice", Password: "hash"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetActive(context.Background(), user.ID, false); err != nil {
		t.Fatal(err)
	}
	if permissions, err := store.PermissionsForUser(context.Background(), user.ID); err != nil || len(permissions) != 1 {
		t.Fatalf("permissions=%v err=%v", permissions, err)
	}
	if err := store.AssignRole(context.Background(), user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.GrantPermission(context.Background(), user.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Roles(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Permissions(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStoreListsUsersWithTotal(t *testing.T) {
	var id pgtype.UUID
	if err := id.Scan("2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a"); err != nil {
		t.Fatal(err)
	}
	store := New(&fakeQueries{
		users: []dbgen.User{{ID: id, Email: "a@example.com", Username: "alice", IsActive: true}},
		total: 7,
	})
	active := true
	users, total, err := store.List(context.Background(), iam.UserFilter{Query: "ali", Active: &active}, 5, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 1 || users[0].ID != "2ad8767a-4a89-4f4f-b4b7-b2fd6ce45d5a" || total != 7 {
		t.Fatalf("users=%+v total=%d", users, total)
	}
}
