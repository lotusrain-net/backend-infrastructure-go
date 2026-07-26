package app

import (
	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/shared/pagination"
	"backend-infrastructure-go/internal/shared/requestcontext"
	"context"
)

type auditPreserver interface {
	Preserve(context.Context, audit.NewEvent, error) error
}
type auditedIAM struct {
	next  iam.Application
	audit auditPreserver
}

func newAuditedIAM(next iam.Application, recorder auditPreserver) *auditedIAM {
	return &auditedIAM{next: next, audit: recorder}
}
func (a *auditedIAM) Login(ctx context.Context, email, password string) (iam.TokenPair, error) {
	value, err := a.next.Login(ctx, email, password)
	return value, a.record(ctx, "auth.login", "session", "", nil, err)
}
func (a *auditedIAM) Refresh(ctx context.Context, token string) (iam.TokenPair, error) {
	value, err := a.next.Refresh(ctx, token)
	return value, a.record(ctx, "auth.refresh", "session", "", nil, err)
}
func (a *auditedIAM) Logout(ctx context.Context, token string) error {
	err := a.next.Logout(ctx, token)
	return a.record(ctx, "auth.logout", "session", "", nil, err)
}
func (a *auditedIAM) CurrentUser(ctx context.Context, id string) (iam.AuthenticatedUser, error) {
	return a.next.CurrentUser(ctx, id)
}
func (a *auditedIAM) Users(ctx context.Context, query iam.UserQuery) (pagination.Page[iam.User], error) {
	return a.next.Users(ctx, query)
}
func (a *auditedIAM) CreateUser(ctx context.Context, input iam.CreateUserInput) (iam.User, error) {
	value, err := a.next.CreateUser(ctx, input)
	return value, a.record(ctx, "administration.user.create", "user", value.ID, nil, err)
}
func (a *auditedIAM) SetUserActive(ctx context.Context, id string, active bool) error {
	err := a.next.SetUserActive(ctx, id, active)
	return a.record(ctx, "administration.user.active", "user", id, nil, err)
}
func (a *auditedIAM) Roles(ctx context.Context) ([]iam.Role, error) { return a.next.Roles(ctx) }
func (a *auditedIAM) Permissions(ctx context.Context) ([]iam.Permission, error) {
	return a.next.Permissions(ctx)
}
func (a *auditedIAM) AssignRole(ctx context.Context, userID, roleID string) error {
	err := a.next.AssignRole(ctx, userID, roleID)
	return a.record(ctx, "administration.role.assign", "user", userID, nil, err)
}
func (a *auditedIAM) GrantPermission(ctx context.Context, roleID, permissionID string) error {
	err := a.next.GrantPermission(ctx, roleID, permissionID)
	return a.record(ctx, "administration.permission.grant", "role", roleID, nil, err)
}
func (a *auditedIAM) Authorize(ctx context.Context, userID, required string) error {
	err := a.next.Authorize(ctx, userID, required)
	return a.record(ctx, "authorization.check", "permission", required, &userID, err)
}
func (a *auditedIAM) record(ctx context.Context, action, resourceType, resourceID string, actorID *string, primary error) error {
	result := audit.ResultSuccess
	if primary != nil {
		result = audit.ResultFailure
	}
	if actorID == nil {
		if subject := iam.Subject(ctx); subject != "" {
			actorID = &subject
		}
	}
	metadata := audit.RequestMetadataFromContext(ctx)
	return a.audit.Preserve(ctx, audit.NewEvent{
		RequestID: requestcontext.RequestID(ctx), ActorID: actorID, Action: action, Result: result,
		ResourceType: resourceType, ResourceID: resourceID, IPAddress: metadata.IPAddress, UserAgent: metadata.UserAgent,
	}, primary)
}
