package iam

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var (
	ErrNotFound                = errors.New("not found")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrInactiveUser            = errors.New("user is inactive")
	ErrDuplicateIdentity       = errors.New("email or username already exists")
	ErrInvalidRefreshToken     = errors.New("invalid refresh token")
	ErrPermissionDenied        = errors.New("permission denied")
	ErrInvalidUserInput        = errors.New("invalid user input")
	ErrCannotDeactivateSelf    = errors.New("cannot deactivate the current user")
	ErrLastActiveAdministrator = errors.New("cannot remove the last active administrator")
	ErrSystemRoleProtected     = errors.New("system role is protected")
	ErrManagementUnavailable   = errors.New("management repository is unavailable")
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
	Roles        []Role    `json:"roles,omitempty"`
}

type AuthenticatedUser struct {
	User
	Permissions []string `json:"permissions"`
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system"`
}

type RoleDetail struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	Permissions []Permission `json:"permissions"`
}
type Permission struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Theme string

const (
	ThemeEnterprise Theme = "enterprise"
	ThemeCyberpunk  Theme = "cyberpunk"
)

type ColorMode string

const (
	ColorModeLight  ColorMode = "light"
	ColorModeDark   ColorMode = "dark"
	ColorModeSystem ColorMode = "system"
)

type FontScale string

const (
	FontScaleSmall    FontScale = "small"
	FontScaleStandard FontScale = "standard"
	FontScaleLarge    FontScale = "large"
)

type RadiusScale string

const (
	RadiusScaleSquare  RadiusScale = "square"
	RadiusScaleCompact RadiusScale = "compact"
	RadiusScaleRounded RadiusScale = "rounded"
)

type Preferences struct {
	Theme       Theme       `json:"theme"`
	ColorMode   ColorMode   `json:"color_mode"`
	AccentColor *string     `json:"accent_color"`
	FontScale   FontScale   `json:"font_scale"`
	RadiusScale RadiusScale `json:"radius_scale"`
}

var accentColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func DefaultPreferences() Preferences {
	return Preferences{
		Theme:       ThemeEnterprise,
		ColorMode:   ColorModeSystem,
		FontScale:   FontScaleStandard,
		RadiusScale: RadiusScaleCompact,
	}
}

func (p Preferences) Validate() error {
	if p.Theme != ThemeEnterprise && p.Theme != ThemeCyberpunk {
		return ErrInvalidUserInput
	}
	if p.ColorMode != ColorModeLight && p.ColorMode != ColorModeDark && p.ColorMode != ColorModeSystem {
		return ErrInvalidUserInput
	}
	if p.FontScale != FontScaleSmall && p.FontScale != FontScaleStandard && p.FontScale != FontScaleLarge {
		return ErrInvalidUserInput
	}
	if p.RadiusScale != RadiusScaleSquare && p.RadiusScale != RadiusScaleCompact && p.RadiusScale != RadiusScaleRounded {
		return ErrInvalidUserInput
	}
	if p.AccentColor != nil && !accentColorPattern.MatchString(*p.AccentColor) {
		return ErrInvalidUserInput
	}
	return nil
}

type CreateUserInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type UpdateUserInput struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type RoleInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

type PreferencesRepository interface {
	GetPreferences(context.Context, string) (Preferences, error)
	PutPreferences(context.Context, string, Preferences) error
}

type ManagementRepository interface {
	UpdateUser(context.Context, string, UpdateUserInput) (User, error)
	UpdateUserPassword(context.Context, string, string) error
	SetUserActive(context.Context, string, bool) error
	ReplaceUserRoles(context.Context, string, []string) error
	Role(context.Context, string) (RoleDetail, error)
	CreateRole(context.Context, RoleInput) (Role, error)
	UpdateRole(context.Context, string, RoleInput) (Role, error)
	DeleteRole(context.Context, string) error
	ReplaceRolePermissions(context.Context, string, []string) error
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
