package iam

import (
	"context"

	"backend-infrastructure-go/internal/shared/pagination"
)

type Application interface {
	Login(context.Context, string, string) (TokenPair, error)
	Refresh(context.Context, string) (TokenPair, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (AuthenticatedUser, error)
	Preferences(context.Context, string) (Preferences, error)
	PutPreferences(context.Context, string, Preferences) error
	Users(context.Context, UserQuery) (pagination.Page[User], error)
	CreateUser(context.Context, CreateUserInput) (User, error)
	UpdateUser(context.Context, string, UpdateUserInput) (User, error)
	ResetUserPassword(context.Context, string, string) error
	SetUserActive(context.Context, string, bool) error
	ReplaceUserRoles(context.Context, string, []string) error
	Roles(context.Context) ([]Role, error)
	Role(context.Context, string) (RoleDetail, error)
	CreateRole(context.Context, RoleInput) (Role, error)
	UpdateRole(context.Context, string, RoleInput) (Role, error)
	DeleteRole(context.Context, string) error
	ReplaceRolePermissions(context.Context, string, []string) error
	Permissions(context.Context) ([]Permission, error)
	AssignRole(context.Context, string, string) error
	GrantPermission(context.Context, string, string) error
	Authorize(context.Context, string, string) error
}

type subjectKey struct{}

func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

func Subject(ctx context.Context) string {
	value, _ := ctx.Value(subjectKey{}).(string)
	return value
}
