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
	ErrInvalidUserInput    = errors.New("invalid user input")
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

type AuthenticatedUser struct {
	User
	Permissions []string `json:"permissions"`
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateUserInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type UserFilter struct {
	Query  string
	Active *bool
}

type UserQuery struct {
	Filter UserFilter
	Page   int
	Size   int
}

type UserRepository interface {
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
	List(context.Context, UserFilter, int, int) ([]User, int64, error)
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
	RefreshToken string `json:"-"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}
