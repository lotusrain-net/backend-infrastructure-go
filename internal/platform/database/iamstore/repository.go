package iamstore

import (
	"context"
	"errors"
	"fmt"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type Queries interface {
	GetUserByEmail(context.Context, string) (dbgen.User, error)
	GetUserByID(context.Context, pgtype.UUID) (dbgen.User, error)
	ListUsers(context.Context, dbgen.ListUsersParams) ([]dbgen.User, error)
	CountUsers(context.Context, dbgen.CountUsersParams) (int64, error)
	CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error)
	SetUserActive(context.Context, dbgen.SetUserActiveParams) (int64, error)
	ListUserPermissions(context.Context, pgtype.UUID) ([]string, error)
	ListRoles(context.Context) ([]dbgen.Role, error)
	ListPermissions(context.Context) ([]dbgen.Permission, error)
	AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error
	GrantRolePermission(context.Context, dbgen.GrantRolePermissionParams) error
}

type Store struct{ queries Queries }

func New(queries Queries) *Store { return &Store{queries: queries} }

func (s *Store) FindByEmail(ctx context.Context, email string) (iam.User, error) {
	row, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return iam.User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}

func (s *Store) FindByID(ctx context.Context, id string) (iam.User, error) {
	uuid, err := parseUUID(id)
	if err != nil {
		return iam.User{}, err
	}
	row, err := s.queries.GetUserByID(ctx, uuid)
	if err != nil {
		return iam.User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}

func (s *Store) List(ctx context.Context, filter iam.UserFilter, limit, offset int) ([]iam.User, int64, error) {
	rows, err := s.queries.ListUsers(ctx, dbgen.ListUsersParams{
		Query:  optionalText(filter.Query),
		Active: optionalBool(filter.Active),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.queries.CountUsers(ctx, dbgen.CountUsersParams{
		Query:  optionalText(filter.Query),
		Active: optionalBool(filter.Active),
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]iam.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userFromDB(row))
	}
	return users, total, nil
}

func (s *Store) Create(ctx context.Context, input iam.CreateUserInput) (iam.User, error) {
	row, err := s.queries.CreateUser(ctx, dbgen.CreateUserParams{Email: input.Email, Username: input.Username, PasswordHash: input.Password, DisplayName: input.DisplayName})
	if err != nil {
		return iam.User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}

func (s *Store) SetActive(ctx context.Context, id string, active bool) error {
	uuid, err := parseUUID(id)
	if err != nil {
		return err
	}
	rows, err := s.queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: uuid, IsActive: active})
	if err != nil {
		return mapDBError(err)
	}
	if rows == 0 {
		return iam.ErrNotFound
	}
	return nil
}

func (s *Store) PermissionsForUser(ctx context.Context, id string) ([]string, error) {
	uuid, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	return s.queries.ListUserPermissions(ctx, uuid)
}

func (s *Store) Roles(ctx context.Context) ([]iam.Role, error) {
	rows, err := s.queries.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]iam.Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, iam.Role{ID: uuidString(row.ID), Name: row.Name, Description: row.Description})
	}
	return roles, nil
}

func (s *Store) Permissions(ctx context.Context) ([]iam.Permission, error) {
	rows, err := s.queries.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	permissions := make([]iam.Permission, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, iam.Permission{ID: uuidString(row.ID), Name: row.Name, Description: row.Description})
	}
	return permissions, nil
}

func (s *Store) AssignRole(ctx context.Context, userID, roleID string) error {
	user, err := parseUUID(userID)
	if err != nil {
		return err
	}
	role, err := parseUUID(roleID)
	if err != nil {
		return err
	}
	return mapDBError(s.queries.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: user, RoleID: role}))
}

func (s *Store) GrantPermission(ctx context.Context, roleID, permissionID string) error {
	role, err := parseUUID(roleID)
	if err != nil {
		return err
	}
	permission, err := parseUUID(permissionID)
	if err != nil {
		return err
	}
	return mapDBError(s.queries.GrantRolePermission(ctx, dbgen.GrantRolePermissionParams{RoleID: role, PermissionID: permission}))
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

func optionalBool(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return iam.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return iam.ErrDuplicateIdentity
		case "23503":
			return iam.ErrNotFound
		}
	}
	return err
}

func userFromDB(row dbgen.User) iam.User {
	return iam.User{ID: uuidString(row.ID), Email: row.Email, Username: row.Username, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}
