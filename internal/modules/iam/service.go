package iam

import (
	"context"
	"errors"
	"strings"

	"github.com/jyysy/backend-infrastructure-go/pkg/pagination"
)

const dummyPasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type Service struct {
	authentication *AuthenticationService
	users          UserRepository
	rbac           RBACRepository
	preferences    PreferencesRepository
	management     ManagementRepository
	passwords      PasswordService
	jwt            *JWTManager
	refresh        *RefreshStore
}

func NewService(users UserRepository, rbac RBACRepository, passwords PasswordService, jwt *JWTManager, refresh *RefreshStore) *Service {
	return NewServiceWithPreferences(users, rbac, nil, passwords, jwt, refresh)
}

func NewServiceWithPreferences(users UserRepository, rbac RBACRepository, preferences PreferencesRepository, passwords PasswordService, jwt *JWTManager, refresh *RefreshStore) *Service {
	return NewServiceWithManagement(users, rbac, preferences, nil, passwords, jwt, refresh)
}

func NewServiceWithManagement(users UserRepository, rbac RBACRepository, preferences PreferencesRepository, management ManagementRepository, passwords PasswordService, jwt *JWTManager, refresh *RefreshStore) *Service {
	return &Service{users: users, rbac: rbac, preferences: preferences, management: management, passwords: passwords, jwt: jwt, refresh: refresh}
}

func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	if s.authentication != nil {
		result, err := s.authentication.Login(ctx, LoginInput{Email: email, Password: password})
		if result.Status == "totp_required" {
			return TokenPair{}, ErrInvalidCredentials
		}
		return result.TokenPair, err
	}
	user, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	passwordHash := dummyPasswordHash
	if err == nil {
		passwordHash = user.PasswordHash
	}
	ok, verifyErr := s.passwords.Verify(password, passwordHash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}
	if verifyErr != nil {
		return TokenPair{}, verifyErr
	}
	if !ok {
		return TokenPair{}, ErrInvalidCredentials
	}
	if !user.Active {
		return TokenPair{}, ErrInactiveUser
	}
	return s.issuePair(ctx, user.ID)
}

func (s *Service) Refresh(ctx context.Context, raw string) (TokenPair, error) {
	if err := s.requireInitialized(ctx); err != nil {
		return TokenPair{}, err
	}
	userID, err := s.refresh.Lookup(ctx, raw)
	if err != nil {
		return TokenPair{}, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return TokenPair{}, ErrInvalidRefreshToken
		}
		return TokenPair{}, err
	}
	if !user.Active {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	access, err := s.jwt.Issue(userID)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.refresh.Rotate(ctx, raw, userID)
	if err != nil {
		return TokenPair{}, err
	}
	return s.tokenPair(access, refresh), nil
}

func (s *Service) Logout(ctx context.Context, raw string) error { return s.refresh.Revoke(ctx, raw) }
func (s *Service) RevokeAll(ctx context.Context, userID string) error {
	return s.refresh.RevokeAll(ctx, userID)
}

func (s *Service) CurrentUser(ctx context.Context, userID string) (AuthenticatedUser, error) {
	if err := s.requireInitialized(ctx); err != nil {
		return AuthenticatedUser{}, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthenticatedUser{}, ErrNotFound
		}
		return AuthenticatedUser{}, err
	}
	if !user.Active {
		return AuthenticatedUser{}, ErrInactiveUser
	}
	authenticated := AuthenticatedUser{User: user, Permissions: []string{}}
	if s.rbac == nil {
		return authenticated, nil
	}
	permissions, err := s.rbac.PermissionsForUser(ctx, userID)
	if err != nil {
		return AuthenticatedUser{}, err
	}
	if permissions != nil {
		authenticated.Permissions = permissions
	}
	return authenticated, nil
}

func (s *Service) Preferences(ctx context.Context, userID string) (Preferences, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return Preferences{}, err
	}
	if s.preferences == nil {
		return Preferences{}, ErrNotFound
	}
	preferences, err := s.preferences.GetPreferences(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return DefaultPreferences(), nil
	}
	return preferences, err
}

func (s *Service) PutPreferences(ctx context.Context, userID string, preferences Preferences) error {
	if err := preferences.Validate(); err != nil {
		return err
	}
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return err
	}
	if s.preferences == nil {
		return ErrNotFound
	}
	return s.preferences.PutPreferences(ctx, userID, preferences)
}

func (s *Service) Users(ctx context.Context, query UserQuery) (pagination.Page[User], error) {
	page, size := pagination.Normalize(query.Page, query.Size)
	items, total, err := s.users.List(ctx, UserFilter{
		Query:  strings.TrimSpace(query.Filter.Query),
		Active: query.Filter.Active,
	}, size, (page-1)*size)
	if err != nil {
		return pagination.Page[User]{}, err
	}
	return pagination.New(items, page, size, total), nil
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.TrimSpace(input.Username)
	if input.Email == "" || input.Username == "" || len(input.Password) < 12 {
		return User{}, ErrInvalidUserInput
	}
	hash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return User{}, err
	}
	input.Password = hash
	return s.users.Create(ctx, input)
}

func (s *Service) UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (User, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.Email == "" || input.Username == "" {
		return User{}, ErrInvalidUserInput
	}
	management, err := s.managementRepository()
	if err != nil {
		return User{}, err
	}
	return management.UpdateUser(ctx, userID, input)
}

func (s *Service) ResetUserPassword(ctx context.Context, userID, password string) error {
	if len(password) < 12 {
		return ErrInvalidUserInput
	}
	management, err := s.managementRepository()
	if err != nil {
		return err
	}
	hash, err := s.passwords.Hash(password)
	if err != nil {
		return err
	}
	if err := s.refresh.RevokeAll(ctx, userID); err != nil {
		return err
	}
	return management.UpdateUserPassword(ctx, userID, hash)
}

func (s *Service) SetUserActive(ctx context.Context, userID string, active bool) error {
	if !active {
		if iamSubject := Subject(ctx); iamSubject != "" && iamSubject == userID {
			return ErrCannotDeactivateSelf
		}
		if err := s.refresh.RevokeAll(ctx, userID); err != nil {
			return err
		}
	}
	if s.management != nil {
		return s.management.SetUserActive(ctx, userID, active)
	}
	return s.users.SetActive(ctx, userID, active)
}
func (s *Service) Roles(ctx context.Context) ([]Role, error) { return s.rbac.Roles(ctx) }
func (s *Service) Permissions(ctx context.Context) ([]Permission, error) {
	return s.rbac.Permissions(ctx)
}
func (s *Service) ReplaceUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	management, err := s.managementRepository()
	if err != nil {
		return err
	}
	roleIDs, err = normalizedIDs(roleIDs)
	if err != nil {
		return err
	}
	return management.ReplaceUserRoles(ctx, userID, roleIDs)
}
func (s *Service) Role(ctx context.Context, roleID string) (RoleDetail, error) {
	management, err := s.managementRepository()
	if err != nil {
		return RoleDetail{}, err
	}
	return management.Role(ctx, roleID)
}
func (s *Service) CreateRole(ctx context.Context, input RoleInput) (Role, error) {
	management, err := s.managementRepository()
	if err != nil {
		return Role{}, err
	}
	input, err = normalizeRoleInput(input)
	if err != nil {
		return Role{}, err
	}
	return management.CreateRole(ctx, input)
}
func (s *Service) UpdateRole(ctx context.Context, roleID string, input RoleInput) (Role, error) {
	management, err := s.managementRepository()
	if err != nil {
		return Role{}, err
	}
	input, err = normalizeRoleInput(input)
	if err != nil {
		return Role{}, err
	}
	return management.UpdateRole(ctx, roleID, input)
}
func (s *Service) DeleteRole(ctx context.Context, roleID string) error {
	management, err := s.managementRepository()
	if err != nil {
		return err
	}
	return management.DeleteRole(ctx, roleID)
}
func (s *Service) ReplaceRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	management, err := s.managementRepository()
	if err != nil {
		return err
	}
	permissionIDs, err = normalizedIDs(permissionIDs)
	if err != nil {
		return err
	}
	return management.ReplaceRolePermissions(ctx, roleID, permissionIDs)
}
func (s *Service) AssignRole(ctx context.Context, userID, roleID string) error {
	return s.rbac.AssignRole(ctx, userID, roleID)
}
func (s *Service) GrantPermission(ctx context.Context, roleID, permissionID string) error {
	return s.rbac.GrantPermission(ctx, roleID, permissionID)
}
func (s *Service) Authorize(ctx context.Context, userID, required string) error {
	if s.authentication != nil {
		if err := s.requireActiveUser(ctx, userID); err != nil {
			return err
		}
	}
	p, err := s.rbac.PermissionsForUser(ctx, userID)
	if err != nil {
		return err
	}
	if !HasPermission(p, required) {
		return ErrPermissionDenied
	}
	return nil
}

func (s *Service) requireActiveUser(ctx context.Context, userID string) error {
	if err := s.requireInitialized(ctx); err != nil {
		return err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if !user.Active {
		return ErrInactiveUser
	}
	return nil
}

func (s *Service) requireInitialized(ctx context.Context) error {
	if s.authentication == nil {
		return nil
	}
	state, err := s.authentication.repo.Authentication(ctx)
	if err != nil {
		return err
	}
	if state.InitializedAt == nil {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) managementRepository() (ManagementRepository, error) {
	if s.management == nil {
		return nil, ErrManagementUnavailable
	}
	return s.management, nil
}

func normalizeRoleInput(input RoleInput) (RoleInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return RoleInput{}, ErrInvalidUserInput
	}
	return input, nil
}

func normalizedIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, ErrInvalidUserInput
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func (s *Service) issuePair(ctx context.Context, userID string) (TokenPair, error) {
	access, err := s.jwt.Issue(userID)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.refresh.Issue(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	return s.tokenPair(access, refresh), nil
}

func (s *Service) tokenPair(access, refresh string) TokenPair {
	return TokenPair{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.jwt.ttl.Seconds())}
}
