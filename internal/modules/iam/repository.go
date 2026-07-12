package iam

import (
	"context"
	"errors"
	"fmt"

	"backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type SQLCQueries interface {
	GetUserByEmail(context.Context, string) (dbgen.User, error)
	GetUserByID(context.Context, pgtype.UUID) (dbgen.User, error)
	CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error)
	SetUserActive(context.Context, dbgen.SetUserActiveParams) error
	ListUserPermissions(context.Context, pgtype.UUID) ([]string, error)
	ListRoles(context.Context) ([]dbgen.Role, error)
	ListPermissions(context.Context) ([]dbgen.Permission, error)
	AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error
	GrantRolePermission(context.Context, dbgen.GrantRolePermissionParams) error
}

type SQLCRepository struct{ queries SQLCQueries }

func NewSQLCRepository(queries SQLCQueries) *SQLCRepository { return &SQLCRepository{queries: queries} }

func (r *SQLCRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}
func (r *SQLCRepository) FindByID(ctx context.Context, id string) (User, error) {
	uuid, err := parseUUID(id)
	if err != nil {
		return User{}, err
	}
	row, err := r.queries.GetUserByID(ctx, uuid)
	if err != nil {
		return User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}
func (r *SQLCRepository) Create(ctx context.Context, in CreateUserInput) (User, error) {
	row, err := r.queries.CreateUser(ctx, dbgen.CreateUserParams{Email: in.Email, Username: in.Username, PasswordHash: in.Password, DisplayName: in.DisplayName})
	if err != nil {
		return User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}
func (r *SQLCRepository) SetActive(ctx context.Context, id string, active bool) error {
	uuid, err := parseUUID(id)
	if err != nil {
		return err
	}
	return mapDBError(r.queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: uuid, IsActive: active}))
}
func (r *SQLCRepository) PermissionsForUser(ctx context.Context, id string) ([]string, error) {
	uuid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	return r.queries.ListUserPermissions(ctx, uuid)
}
func (r *SQLCRepository) Roles(ctx context.Context) ([]Role, error) {
	rows, err := r.queries.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(rows))
	for _, row := range rows {
		out = append(out, Role{ID: uuidString(row.ID), Name: row.Name, Description: row.Description})
	}
	return out, nil
}
func (r *SQLCRepository) Permissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.queries.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Permission, 0, len(rows))
	for _, row := range rows {
		out = append(out, Permission{ID: uuidString(row.ID), Name: row.Name, Description: row.Description})
	}
	return out, nil
}
func (r *SQLCRepository) AssignRole(ctx context.Context, userID, roleID string) error {
	u, err := parseUUID(userID)
	if err != nil {
		return err
	}
	role, err := parseUUID(roleID)
	if err != nil {
		return err
	}
	return mapDBError(r.queries.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: u, RoleID: role}))
}
func (r *SQLCRepository) GrantPermission(ctx context.Context, roleID, permissionID string) error {
	role, err := parseUUID(roleID)
	if err != nil {
		return err
	}
	permission, err := parseUUID(permissionID)
	if err != nil {
		return err
	}
	return mapDBError(r.queries.GrantRolePermission(ctx, dbgen.GrantRolePermissionParams{RoleID: role, PermissionID: permission}))
}

func parseUUID(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil || !uuid.Valid {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID %q", value)
	}
	return uuid, nil
}
func uuidString(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	b := uuid.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateIdentity
	}
	return err
}
func userFromDB(row dbgen.User) User {
	return User{ID: uuidString(row.ID), Email: row.Email, Username: row.Username, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}
