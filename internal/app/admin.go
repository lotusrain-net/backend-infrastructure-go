package app

import (
	"context"
	"errors"
	"fmt"

	"backend-infrastructure-go/internal/modules/iam"
	"backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jackc/pgx/v5"
)

type AdminBootstrap struct{ Email, Username, Password string }
type adminQueries interface {
	GetUserByEmail(context.Context, string) (dbgen.User, error)
	CreateUser(context.Context, dbgen.CreateUserParams) (dbgen.User, error)
	GetRoleByName(context.Context, string) (dbgen.Role, error)
	AssignUserRole(context.Context, dbgen.AssignUserRoleParams) error
}

func BootstrapAdmin(ctx context.Context, queries adminQueries, hasher iam.PasswordHasher, input AdminBootstrap) error {
	user, err := queries.GetUserByEmail(ctx, input.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, hashErr := hasher.Hash(input.Password)
		if hashErr != nil {
			return fmt.Errorf("hash bootstrap admin password: %w", hashErr)
		}
		user, err = queries.CreateUser(ctx, dbgen.CreateUserParams{Email: input.Email, Username: input.Username, PasswordHash: hash, DisplayName: "Administrator"})
	}
	if err != nil {
		return fmt.Errorf("load bootstrap admin: %w", err)
	}
	role, err := queries.GetRoleByName(ctx, "admin")
	if err != nil {
		return fmt.Errorf("load admin role: %w", err)
	}
	if err := queries.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: user.ID, RoleID: role.ID}); err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}
	return nil
}
