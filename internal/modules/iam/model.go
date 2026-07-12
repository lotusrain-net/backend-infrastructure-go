package iam

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInactiveUser        = errors.New("user is inactive")
	ErrDuplicateIdentity   = errors.New("email or username already exists")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrPermissionDenied    = errors.New("permission denied")
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`
	Active       bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type Role struct{ ID, Name, Description string }
type Permission struct{ ID, Name, Description string }

type CreateUserInput struct{ Email, Username, Password, DisplayName string }

type UserRepository interface {
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
	Create(context.Context, CreateUserInput) (User, error)
	SetActive(context.Context, string, bool) error
}

type RBACRepository interface {
	PermissionsForUser(context.Context, string) ([]string, error)
	Roles(context.Context) ([]Role, error)
	Permissions(context.Context) ([]Permission, error)
	AssignRole(context.Context, string, string) error
	GrantPermission(context.Context, string, string) error
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}
