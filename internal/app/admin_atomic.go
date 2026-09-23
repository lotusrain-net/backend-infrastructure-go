package app

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/modules/iam"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/platform/database/dbgen"
	"github.com/lotusrain-net/backend-infrastructure-go/pkg/postgres"
)

// SeedBootstrapAdmin serializes creation and binding so a second seed cannot
// create another administrator or replace the credentials of an existing one.
func SeedBootstrapAdmin(ctx context.Context, beginner postgres.Beginner, hasher iam.PasswordHasher, input AdminBootstrap) error {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	return postgres.RunInTx(ctx, beginner, func(ctx context.Context, tx pgx.Tx) error {
		q := dbgen.New(tx)
		settings, e := q.LockAuthenticationSettings(ctx)
		if e != nil {
			return e
		}
		if settings.BootstrapAdminUserID.Valid {
			user, e := q.GetUserByID(ctx, settings.BootstrapAdminUserID)
			if e != nil {
				return e
			}
			if user.Email != input.Email {
				return errors.New("bootstrap administrator is already bound to a different account")
			}
		}
		if e = BootstrapAdmin(ctx, q, hasher, input); e != nil {
			return e
		}
		user, e := q.GetUserByEmail(ctx, input.Email)
		if e != nil {
			return e
		}
		n, e := q.BindBootstrapAdmin(ctx, user.ID)
		if e != nil {
			return e
		}
		if n != 1 {
			return errors.New("bootstrap administrator binding conflict")
		}
		return nil
	})
}
