package iam

import (
	"context"
	"errors"
	"strings"
)

const dummyPasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type Service struct {
	users     UserRepository
	rbac      RBACRepository
	passwords PasswordService
	jwt       *JWTManager
	refresh   *RefreshStore
}

func NewService(users UserRepository, rbac RBACRepository, passwords PasswordService, jwt *JWTManager, refresh *RefreshStore) *Service {
	return &Service{users: users, rbac: rbac, passwords: passwords, jwt: jwt, refresh: refresh}
}

func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
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

func (s *Service) CurrentUser(ctx context.Context, userID string) (User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	if !user.Active {
		return User{}, ErrInactiveUser
	}
	return user, nil
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

func (s *Service) SetUserActive(ctx context.Context, userID string, active bool) error {
	if !active {
		if err := s.refresh.RevokeAll(ctx, userID); err != nil {
			return err
		}
	}
	return s.users.SetActive(ctx, userID, active)
}
func (s *Service) Roles(ctx context.Context) ([]Role, error) { return s.rbac.Roles(ctx) }
func (s *Service) Permissions(ctx context.Context) ([]Permission, error) {
	return s.rbac.Permissions(ctx)
}
func (s *Service) AssignRole(ctx context.Context, userID, roleID string) error {
	return s.rbac.AssignRole(ctx, userID, roleID)
}
func (s *Service) GrantPermission(ctx context.Context, roleID, permissionID string) error {
	return s.rbac.GrantPermission(ctx, roleID, permissionID)
}
func (s *Service) Authorize(ctx context.Context, userID, required string) error {
	p, err := s.rbac.PermissionsForUser(ctx, userID)
	if err != nil {
		return err
	}
	if !HasPermission(p, required) {
		return ErrPermissionDenied
	}
	return nil
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
