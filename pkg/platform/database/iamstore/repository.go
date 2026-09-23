package iamstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	database "github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
)

type Queries interface {
	GetUserByEmail(context.Context, string) (dbgen.User, error)
	GetUserByID(context.Context, pgtype.UUID) (dbgen.User, error)
	GetUserPreferences(context.Context, pgtype.UUID) (dbgen.UserPreference, error)
	ListUsers(context.Context, dbgen.ListUsersParams) ([]dbgen.User, error)
	CountUsers(context.Context, dbgen.CountUsersParams) (int64, error)
	CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error)
	SetUserActive(context.Context, dbgen.SetUserActiveParams) (int64, error)
	UpdateUserProfile(context.Context, dbgen.UpdateUserProfileParams) (dbgen.User, error)
	UpdateUserPassword(context.Context, dbgen.UpdateUserPasswordParams) (int64, error)
	EnsureSecuritySettings(context.Context, pgtype.UUID) error
	LockSecuritySettings(context.Context, pgtype.UUID) (dbgen.UserSecuritySetting, error)
	InvalidatePasswordCredentials(context.Context, pgtype.UUID) error
	ListUserPermissions(context.Context, pgtype.UUID) ([]string, error)
	ListRoles(context.Context) ([]dbgen.Role, error)
	ListRolesForUser(context.Context, pgtype.UUID) ([]dbgen.Role, error)
	ListRolePermissions(context.Context, pgtype.UUID) ([]dbgen.Permission, error)
	ListPermissions(context.Context) ([]dbgen.Permission, error)
	AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error
	GrantRolePermission(context.Context, dbgen.GrantRolePermissionParams) error
	GetRoleByID(context.Context, pgtype.UUID) (dbgen.Role, error)
	GetSystemAdminRole(context.Context) (dbgen.Role, error)
	LockSystemAdminRole(context.Context) (dbgen.Role, error)
	GetPermissionByID(context.Context, pgtype.UUID) (dbgen.Permission, error)
	CreateRole(context.Context, dbgen.CreateRoleParams) (dbgen.Role, error)
	UpdateRole(context.Context, dbgen.UpdateRoleParams) (dbgen.Role, error)
	DeleteRole(context.Context, pgtype.UUID) (int64, error)
	DeleteUserRoles(context.Context, pgtype.UUID) error
	DeleteRolePermissions(context.Context, pgtype.UUID) error
	LockActiveSystemAdministratorIDs(context.Context) ([]pgtype.UUID, error)
	UpsertUserPreferences(context.Context, dbgen.UpsertUserPreferencesParams) error
}

type Store struct {
	queries            Queries
	beginner           database.Beginner
	transactionQueries func(pgx.Tx) Queries
}

func New(queries Queries, beginners ...database.Beginner) *Store {
	store := &Store{queries: queries}
	if len(beginners) == 0 {
		return store
	}
	generated, ok := queries.(*dbgen.Queries)
	if !ok || beginners[0] == nil {
		return store
	}
	store.beginner = beginners[0]
	store.transactionQueries = func(tx pgx.Tx) Queries { return generated.WithTx(tx) }
	return store
}

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

func (s *Store) GetPreferences(ctx context.Context, id string) (iam.Preferences, error) {
	userID, err := parseUUID(id)
	if err != nil {
		return iam.Preferences{}, err
	}
	row, err := s.queries.GetUserPreferences(ctx, userID)
	if err != nil {
		return iam.Preferences{}, mapDBError(err)
	}
	return preferencesFromDB(row), nil
}

func (s *Store) PutPreferences(ctx context.Context, id string, preferences iam.Preferences) error {
	userID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return mapDBError(s.queries.UpsertUserPreferences(ctx, dbgen.UpsertUserPreferencesParams{
		UserID:      userID,
		Theme:       string(preferences.Theme),
		ColorMode:   string(preferences.ColorMode),
		AccentColor: optionalAccentColor(preferences.AccentColor),
		FontScale:   string(preferences.FontScale),
		RadiusScale: string(preferences.RadiusScale),
	}))
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
		roles, err := s.queries.ListRolesForUser(ctx, row.ID)
		if err != nil {
			return nil, 0, mapDBError(err)
		}
		user := userFromDB(row)
		user.Roles = rolesFromDB(roles)
		users = append(users, user)
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

func (s *Store) UpdateUser(ctx context.Context, id string, input iam.UpdateUserInput) (iam.User, error) {
	userID, err := parseUUID(id)
	if err != nil {
		return iam.User{}, err
	}
	var row dbgen.User
	err = s.withLockedSecurity(ctx, userID, func(ctx context.Context, q Queries, security dbgen.UserSecuritySetting) error {
		current, err := q.GetUserByID(ctx, userID)
		if err != nil {
			return mapDBError(err)
		}
		// Preserve the verified mailbox while email second-factor login is active.
		// The user must switch to default before changing and reverifying it.
		if current.Email != input.Email && security.Mode == "email" {
			return iam.ErrAuthenticationForbidden
		}
		row, err = q.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
			ID: userID, Email: input.Email, Username: input.Username, DisplayName: input.DisplayName,
		})
		return mapDBError(err)
	})
	if err != nil {
		return iam.User{}, mapDBError(err)
	}
	return userFromDB(row), nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	userID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return s.withLockedSecurity(ctx, userID, func(ctx context.Context, q Queries, _ dbgen.UserSecuritySetting) error {
		// Password and security version commit together. This also invalidates
		// enrollment authorized by the old password, without removing TOTP.
		if err := q.InvalidatePasswordCredentials(ctx, userID); err != nil {
			return mapDBError(err)
		}
		rows, err := q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{ID: userID, PasswordHash: passwordHash})
		if err != nil {
			return mapDBError(err)
		}
		if rows == 0 {
			return iam.ErrNotFound
		}
		return nil
	})
}

func (s *Store) withLockedSecurity(ctx context.Context, id pgtype.UUID, fn func(context.Context, Queries, dbgen.UserSecuritySetting) error) error {
	return s.withTransaction(ctx, func(ctx context.Context, q Queries) error {
		if err := q.EnsureSecuritySettings(ctx, id); err != nil {
			return mapDBError(err)
		}
		security, err := q.LockSecuritySettings(ctx, id)
		if err != nil {
			return mapDBError(err)
		}
		return fn(ctx, q, security)
	})
}

func (s *Store) SetUserActive(ctx context.Context, id string, active bool) error {
	if active {
		return s.SetActive(ctx, id, true)
	}
	userID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return s.withTransaction(ctx, func(ctx context.Context, queries Queries) error {
		if _, err := queries.GetUserByID(ctx, userID); err != nil {
			return mapDBError(err)
		}
		if _, err := queries.LockSystemAdminRole(ctx); err != nil {
			return mapDBError(err)
		}
		adminIDs, err := queries.LockActiveSystemAdministratorIDs(ctx)
		if err != nil {
			return mapDBError(err)
		}
		if uuidIn(adminIDs, userID) && len(adminIDs) == 1 {
			return iam.ErrLastActiveAdministrator
		}
		return setActive(ctx, queries, userID, false)
	})
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
		roles = append(roles, roleFromDB(row))
	}
	return roles, nil
}

func (s *Store) Role(ctx context.Context, id string) (iam.RoleDetail, error) {
	roleID, err := parseUUID(id)
	if err != nil {
		return iam.RoleDetail{}, err
	}
	role, err := s.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		return iam.RoleDetail{}, mapDBError(err)
	}
	permissions, err := s.queries.ListRolePermissions(ctx, roleID)
	if err != nil {
		return iam.RoleDetail{}, mapDBError(err)
	}
	return roleDetailFromDB(role, permissions), nil
}

func (s *Store) CreateRole(ctx context.Context, input iam.RoleInput) (iam.Role, error) {
	role, err := s.queries.CreateRole(ctx, dbgen.CreateRoleParams{Name: input.Name, Description: input.Description})
	if err != nil {
		return iam.Role{}, mapDBError(err)
	}
	return roleFromDB(role), nil
}

func (s *Store) UpdateRole(ctx context.Context, id string, input iam.RoleInput) (iam.Role, error) {
	roleID, err := parseUUID(id)
	if err != nil {
		return iam.Role{}, err
	}
	existing, err := s.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		return iam.Role{}, mapDBError(err)
	}
	if existing.IsSystem && existing.Name != input.Name {
		return iam.Role{}, iam.ErrSystemRoleProtected
	}
	role, err := s.queries.UpdateRole(ctx, dbgen.UpdateRoleParams{ID: roleID, Name: input.Name, Description: input.Description})
	if err != nil {
		return iam.Role{}, mapDBError(err)
	}
	return roleFromDB(role), nil
}

func (s *Store) DeleteRole(ctx context.Context, id string) error {
	roleID, err := parseUUID(id)
	if err != nil {
		return err
	}
	role, err := s.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		return mapDBError(err)
	}
	if role.IsSystem {
		return iam.ErrSystemRoleProtected
	}
	rows, err := s.queries.DeleteRole(ctx, roleID)
	if err != nil {
		return mapDBError(err)
	}
	if rows == 0 {
		return iam.ErrNotFound
	}
	return nil
}

func (s *Store) ReplaceUserRoles(ctx context.Context, id string, roleIDs []string) error {
	userID, err := parseUUID(id)
	if err != nil {
		return err
	}
	parsedRoleIDs := make([]pgtype.UUID, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		parsed, err := parseUUID(roleID)
		if err != nil {
			return err
		}
		parsedRoleIDs = append(parsedRoleIDs, parsed)
	}
	return s.withTransaction(ctx, func(ctx context.Context, queries Queries) error {
		if _, err := queries.GetUserByID(ctx, userID); err != nil {
			return mapDBError(err)
		}
		adminRole, err := queries.LockSystemAdminRole(ctx)
		if err != nil {
			return mapDBError(err)
		}
		adminIDs, err := queries.LockActiveSystemAdministratorIDs(ctx)
		if err != nil {
			return mapDBError(err)
		}
		for _, roleID := range parsedRoleIDs {
			if _, err := queries.GetRoleByID(ctx, roleID); err != nil {
				return mapDBError(err)
			}
		}
		if uuidIn(adminIDs, userID) && len(adminIDs) == 1 && !uuidIn(parsedRoleIDs, adminRole.ID) {
			return iam.ErrLastActiveAdministrator
		}
		if err := queries.DeleteUserRoles(ctx, userID); err != nil {
			return mapDBError(err)
		}
		for _, roleID := range parsedRoleIDs {
			if err := queries.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: userID, RoleID: roleID}); err != nil {
				return mapDBError(err)
			}
		}
		return nil
	})
}

func (s *Store) ReplaceRolePermissions(ctx context.Context, id string, permissionIDs []string) error {
	roleID, err := parseUUID(id)
	if err != nil {
		return err
	}
	parsedPermissionIDs := make([]pgtype.UUID, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		parsed, err := parseUUID(permissionID)
		if err != nil {
			return err
		}
		parsedPermissionIDs = append(parsedPermissionIDs, parsed)
	}
	return s.withTransaction(ctx, func(ctx context.Context, queries Queries) error {
		role, err := queries.GetRoleByID(ctx, roleID)
		if err != nil {
			return mapDBError(err)
		}
		if role.IsSystem {
			return iam.ErrSystemRoleProtected
		}
		for _, permissionID := range parsedPermissionIDs {
			if _, err := queries.GetPermissionByID(ctx, permissionID); err != nil {
				return mapDBError(err)
			}
		}
		if err := queries.DeleteRolePermissions(ctx, roleID); err != nil {
			return mapDBError(err)
		}
		for _, permissionID := range parsedPermissionIDs {
			if err := queries.GrantRolePermission(ctx, dbgen.GrantRolePermissionParams{RoleID: roleID, PermissionID: permissionID}); err != nil {
				return mapDBError(err)
			}
		}
		return nil
	})
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
	existing, err := s.queries.GetRoleByID(ctx, role)
	if err != nil {
		return mapDBError(err)
	}
	if existing.IsSystem {
		return iam.ErrSystemRoleProtected
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

func (s *Store) withTransaction(ctx context.Context, fn func(context.Context, Queries) error) error {
	if s.beginner == nil || s.transactionQueries == nil {
		return iam.ErrManagementUnavailable
	}
	return database.RunInTx(ctx, s.beginner, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, s.transactionQueries(tx))
	})
}

func setActive(ctx context.Context, queries Queries, userID pgtype.UUID, active bool) error {
	rows, err := queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: userID, IsActive: active})
	if err != nil {
		return mapDBError(err)
	}
	if rows == 0 {
		return iam.ErrNotFound
	}
	return nil
}

func userFromDB(row dbgen.User) iam.User {
	return iam.User{EmailVerifiedAt: timePointer(row.EmailVerifiedAt), ID: uuidString(row.ID), Email: row.Email, Username: row.Username, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Active: row.IsActive, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}

func roleFromDB(row dbgen.Role) iam.Role {
	return iam.Role{ID: uuidString(row.ID), Name: row.Name, Description: row.Description, IsSystem: row.IsSystem}
}

func rolesFromDB(rows []dbgen.Role) []iam.Role {
	roles := make([]iam.Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, roleFromDB(row))
	}
	return roles
}

func permissionFromDB(row dbgen.Permission) iam.Permission {
	return iam.Permission{ID: uuidString(row.ID), Name: row.Name, Description: row.Description}
}

func roleDetailFromDB(role dbgen.Role, permissions []dbgen.Permission) iam.RoleDetail {
	items := make([]iam.Permission, 0, len(permissions))
	for _, permission := range permissions {
		items = append(items, permissionFromDB(permission))
	}
	return iam.RoleDetail{ID: uuidString(role.ID), Name: role.Name, Description: role.Description, IsSystem: role.IsSystem, Permissions: items}
}

func uuidIn(values []pgtype.UUID, wanted pgtype.UUID) bool {
	for _, value := range values {
		if value.Valid && wanted.Valid && value.Bytes == wanted.Bytes {
			return true
		}
	}
	return false
}

func preferencesFromDB(row dbgen.UserPreference) iam.Preferences {
	preferences := iam.Preferences{
		Theme:       iam.Theme(row.Theme),
		ColorMode:   iam.ColorMode(row.ColorMode),
		FontScale:   iam.FontScale(row.FontScale),
		RadiusScale: iam.RadiusScale(row.RadiusScale),
	}
	if row.AccentColor.Valid {
		accent := row.AccentColor.String
		preferences.AccentColor = &accent
	}
	return preferences
}

func optionalAccentColor(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
