package migrations_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jyysy/backend-infrastructure-go/internal/modules/iam"
	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/dbgen"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/iamstore"
	"github.com/jyysy/backend-infrastructure-go/internal/platform/database/taskstore"
	queueplatform "github.com/jyysy/backend-infrastructure-go/internal/platform/queue"
)

type integrationPublisher struct {
	message taskmodule.Message
}

func (publisher *integrationPublisher) Publish(_ context.Context, message taskmodule.Message, _ taskmodule.PublishOptions) error {
	publisher.message = message
	return nil
}

func TestMigrationsUpDownUpAndSeedIdempotency(t *testing.T) {
	databaseURL := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL is not set")
	}

	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(directory)}).String()
	migrator, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = migrator.Close() })

	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("initial down: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if err := migrator.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second up error = %v, want ErrNoChange", err)
	}

	ctx := t.Context()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	seedSQL, err := os.ReadFile("000004_seed_rbac.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := conn.Exec(ctx, string(seedSQL)); err != nil {
			t.Fatalf("reapply idempotent seed: %v", err)
		}
	}

	var roles, permissions int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM roles WHERE name IN ('admin', 'user')").Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM permissions").Scan(&permissions); err != nil {
		t.Fatal(err)
	}
	if roles != 2 || permissions != 12 {
		t.Fatalf("seed counts roles=%d permissions=%d", roles, permissions)
	}
	exerciseGeneratedQueries(t, ctx, conn)
	exerciseIAMManagement(t, ctx, conn)
	exerciseConstraints(t, ctx, conn)

	_, err = conn.Exec(ctx, "INSERT INTO users (email, username, password_hash) VALUES ('invalid@example.com', '', 'hash')")
	if err == nil {
		t.Fatal("users_username_not_blank constraint accepted an empty username")
	}
	if _, err := conn.Exec(ctx, "INSERT INTO roles (name) VALUES ('custom-seed-consumer')"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT roles.id, permissions.id
		FROM roles, permissions
		WHERE roles.name = 'custom-seed-consumer' AND permissions.name = 'tasks:read'
	`); err != nil {
		t.Fatal(err)
	}

	if err := migrator.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}

func exerciseConstraints(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (email, username, password_hash)
		VALUES ('constraint-email@example.com', 'constraint-user', 'hash'),
		       ('constraint-other@example.com', 'constraint-other', 'hash')
	`); err != nil {
		t.Fatal(err)
	}
	var definitionID pgtype.UUID
	if err := conn.QueryRow(ctx, `
		INSERT INTO task_definitions (name, task_type) VALUES ('constraint-definition', 'system.constraint')
		RETURNING id
	`).Scan(&definitionID); err != nil {
		t.Fatal(err)
	}
	var executionID pgtype.UUID
	if err := conn.QueryRow(ctx, `
		INSERT INTO task_executions (task_type, idempotency_key)
		VALUES ('system.constraint', 'constraint-idempotency')
		RETURNING id
	`).Scan(&executionID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		code string
		sql  string
		args []any
	}{
		{name: "unique email", code: "23505", sql: "INSERT INTO users (email, username, password_hash) VALUES ('constraint-email@example.com', 'unique-email-user', 'hash')"},
		{name: "unique username", code: "23505", sql: "INSERT INTO users (email, username, password_hash) VALUES ('unique-username@example.com', 'constraint-user', 'hash')"},
		{name: "RBAC user foreign key", code: "23503", sql: "INSERT INTO user_roles (user_id, role_id) SELECT gen_random_uuid(), id FROM roles WHERE name = 'user'"},
		{name: "audit result", code: "23514", sql: "INSERT INTO audit_logs (request_id, action, result, resource_type) VALUES ('constraint-request', 'test', 'unknown', 'test')"},
		{name: "task definition retries", code: "23514", sql: "INSERT INTO task_definitions (name, task_type, max_retries) VALUES ('invalid-retries', 'system.test', -1)"},
		{name: "task definition timeout", code: "23514", sql: "INSERT INTO task_definitions (name, task_type, timeout_seconds) VALUES ('invalid-timeout', 'system.test', 0)"},
		{name: "task execution status", code: "23514", sql: "INSERT INTO task_executions (task_type, status) VALUES ('system.test', 'unknown')"},
		{name: "task execution attempt", code: "23514", sql: "INSERT INTO task_executions (task_type, attempt) VALUES ('system.test', -1)"},
		{name: "task execution processed rows", code: "23514", sql: "INSERT INTO task_executions (task_type, processed_rows) VALUES ('system.test', -1)"},
		{name: "task schedule cron", code: "23514", sql: "INSERT INTO task_schedules (definition_id, cron_expression) VALUES ($1, '')", args: []any{definitionID}},
		{name: "task schedule timezone", code: "23514", sql: "INSERT INTO task_schedules (definition_id, cron_expression, timezone) VALUES ($1, '* * * * *', '')", args: []any{definitionID}},
		{name: "task idempotency", code: "23505", sql: "INSERT INTO task_executions (task_type, idempotency_key) VALUES ('system.constraint', 'constraint-idempotency')"},
		{name: "outbox execution foreign key", code: "23503", sql: "INSERT INTO task_outbox_messages (queue_id, execution_id, task_type) VALUES ('missing-execution', gen_random_uuid(), 'system.test')"},
		{name: "outbox queue blank", code: "23514", sql: "INSERT INTO task_outbox_messages (queue_id, execution_id, task_type) VALUES ('', $1, 'system.test')", args: []any{executionID}},
		{name: "outbox type blank", code: "23514", sql: "INSERT INTO task_outbox_messages (queue_id, execution_id, task_type) VALUES ('blank-type', $1, '')", args: []any{executionID}},
		{name: "outbox retries", code: "23514", sql: "INSERT INTO task_outbox_messages (queue_id, execution_id, task_type, max_retries) VALUES ('invalid-retries', $1, 'system.test', -1)", args: []any{executionID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conn.Exec(ctx, tt.sql, tt.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tt.code {
				t.Fatalf("Exec() error = %v, want PostgreSQL code %s", err, tt.code)
			}
		})
	}

	var userID, roleID pgtype.UUID
	if err := conn.QueryRow(ctx, "SELECT id FROM users WHERE username = 'constraint-other'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'user'").Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)", userID, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	var userRoleCount int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM user_roles WHERE user_id = $1", userID).Scan(&userRoleCount); err != nil {
		t.Fatal(err)
	}
	if userRoleCount != 0 {
		t.Fatalf("user role cascade count = %d, want 0", userRoleCount)
	}

	var cascadeRoleID pgtype.UUID
	if err := conn.QueryRow(ctx, "INSERT INTO roles (name) VALUES ('cascade-role') RETURNING id").Scan(&cascadeRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT $1, id FROM permissions WHERE name = 'audit:read'
	`, cascadeRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "DELETE FROM roles WHERE id = $1", cascadeRoleID); err != nil {
		t.Fatal(err)
	}
	var rolePermissionCount int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM role_permissions WHERE role_id = $1", cascadeRoleID).Scan(&rolePermissionCount); err != nil {
		t.Fatal(err)
	}
	if rolePermissionCount != 0 {
		t.Fatalf("role permission cascade count = %d, want 0", rolePermissionCount)
	}
}

func exerciseGeneratedQueries(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	queries := dbgen.New(conn)
	executionStore := taskstore.NewPostgresExecutionStore(queries)
	execution, err := executionStore.CreateExecutionWithPendingPublish(ctx,
		taskmodule.NewExecution{TaskType: taskmodule.SystemTestTaskType, QueueID: "integration-outbox", Payload: json.RawMessage(`{"processed_rows":1}`)},
		taskmodule.Message{TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{"processed_rows":1}`)},
		taskmodule.PublishOptions{QueueID: "integration-outbox", MaxRetries: 3, Timeout: time.Minute},
	)
	if err != nil {
		t.Fatalf("CreateExecutionWithPendingPublish() error = %v", err)
	}
	publisher := &integrationPublisher{}
	if err := queueplatform.NewOutboxDispatcher(executionStore, publisher).DispatchPending(ctx, 10); err != nil {
		t.Fatalf("DispatchPending() error = %v", err)
	}
	if publisher.message.ExecutionID != execution.ID {
		t.Fatalf("published message = %+v, execution = %+v", publisher.message, execution)
	}
	pending, err := executionStore.ListPendingPublishes(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingPublishes() after drain error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending publishes after drain = %+v", pending)
	}

	terminalExecution, err := executionStore.CreateExecutionWithPendingPublish(ctx,
		taskmodule.NewExecution{TaskType: taskmodule.SystemTestTaskType, QueueID: "terminal-outbox", Payload: json.RawMessage(`{}`)},
		taskmodule.Message{TaskType: taskmodule.SystemTestTaskType, Payload: json.RawMessage(`{}`)},
		taskmodule.PublishOptions{QueueID: "terminal-outbox", Timeout: time.Minute},
	)
	if err != nil {
		t.Fatalf("create terminal outbox execution: %v", err)
	}
	if _, err := conn.Exec(ctx, "UPDATE task_executions SET status = 'succeeded' WHERE id = $1", terminalExecution.ID); err != nil {
		t.Fatalf("mark terminal outbox execution succeeded: %v", err)
	}
	publisher.message = taskmodule.Message{}
	if err := queueplatform.NewOutboxDispatcher(executionStore, publisher).DispatchPending(ctx, 10); err != nil {
		t.Fatalf("dispatch terminal outbox: %v", err)
	}
	if publisher.message.ExecutionID != "" {
		t.Fatalf("terminal execution was republished: %+v", publisher.message)
	}
	pending, err = executionStore.ListPendingPublishes(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending terminal publishes after cleanup = %+v, %v", pending, err)
	}

	store := iamstore.New(queries)
	authorization := iam.NewService(nil, store, iam.PasswordHasher{}, nil, nil)
	created, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        "dbgen@example.com",
		Username:     "dbgen-user",
		PasswordHash: "hash-1",
		DisplayName:  "DB Gen",
	})
	if err != nil {
		t.Fatal(err)
	}
	byID, err := queries.GetUserByID(ctx, created.ID)
	if err != nil || byID.Email != created.Email {
		t.Fatalf("GetUserByID() = %#v, %v", byID, err)
	}
	byEmail, err := queries.GetUserByEmail(ctx, created.Email)
	if err != nil || byEmail.ID != created.ID {
		t.Fatalf("GetUserByEmail() = %#v, %v", byEmail, err)
	}
	if _, err := queries.UpdateUserProfile(ctx, dbgen.UpdateUserProfileParams{
		ID:          created.ID,
		Email:       "dbgen-updated@example.com",
		Username:    "dbgen-updated",
		DisplayName: "Updated",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{ID: created.ID, PasswordHash: "hash-2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: created.ID, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	missingID := "00000000-0000-4000-8000-000000000099"
	if err := store.SetActive(ctx, missingID, false); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("SetActive(missing) error = %v", err)
	}
	updated, err := queries.GetUserByID(ctx, created.ID)
	if err != nil || updated.Email != "dbgen-updated@example.com" || updated.Username != "dbgen-updated" || updated.DisplayName != "Updated" || updated.PasswordHash != "hash-2" || updated.IsActive {
		t.Fatalf("updated user = %#v, %v", updated, err)
	}

	var customRoleID pgtype.UUID
	if err := conn.QueryRow(ctx, "INSERT INTO roles (name, description) VALUES ('dbgen-role', 'dbgen integration') RETURNING id").Scan(&customRoleID); err != nil {
		t.Fatal(err)
	}
	role, err := queries.GetRoleByName(ctx, "dbgen-role")
	if err != nil || role.ID != customRoleID {
		t.Fatalf("GetRoleByName() = %#v, %v", role, err)
	}
	permission, err := queries.GetPermissionByName(ctx, "tasks:write")
	if err != nil {
		t.Fatal(err)
	}
	grant := dbgen.GrantRolePermissionParams{RoleID: role.ID, PermissionID: permission.ID}
	if err := queries.GrantRolePermission(ctx, grant); err != nil {
		t.Fatal(err)
	}
	if err := store.AssignRole(ctx, missingID, missingID); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("AssignRole(missing) error = %v", err)
	}
	if err := store.GrantPermission(ctx, missingID, missingID); !errors.Is(err, iam.ErrNotFound) {
		t.Fatalf("GrantPermission(missing) error = %v", err)
	}
	if err := queries.GrantRolePermission(ctx, grant); err != nil {
		t.Fatalf("duplicate GrantRolePermission() error = %v", err)
	}
	assignment := dbgen.AssignUserRoleParams{UserID: created.ID, RoleID: role.ID}
	if err := queries.AssignUserRole(ctx, assignment); err != nil {
		t.Fatal(err)
	}
	if err := queries.AssignUserRole(ctx, assignment); err != nil {
		t.Fatalf("duplicate AssignUserRole() error = %v", err)
	}
	if _, err := queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: created.ID, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	userPermissions, err := queries.ListUserPermissions(ctx, created.ID)
	if err != nil || len(userPermissions) != 1 || userPermissions[0] != "tasks:write" {
		t.Fatalf("ListUserPermissions() = %v, %v", userPermissions, err)
	}
	if err := authorization.Authorize(ctx, created.ID.String(), "tasks:write"); err != nil {
		t.Fatalf("Authorize() active user error = %v", err)
	}
	if _, err := queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: created.ID, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	inactivePermissions, err := queries.ListUserPermissions(ctx, created.ID)
	if err != nil || len(inactivePermissions) != 0 {
		t.Fatalf("ListUserPermissions() for inactive user = %v, %v; want no permissions", inactivePermissions, err)
	}
	if err := authorization.Authorize(ctx, created.ID.String(), "tasks:write"); !errors.Is(err, iam.ErrPermissionDenied) {
		t.Fatalf("Authorize() inactive user error = %v, want permission denied", err)
	}
	if _, err := queries.SetUserActive(ctx, dbgen.SetUserActiveParams{ID: created.ID, IsActive: true}); err != nil {
		t.Fatal(err)
	}
	if roles, err := queries.ListRoles(ctx); err != nil || len(roles) < 3 {
		t.Fatalf("ListRoles() count = %d, %v", len(roles), err)
	}
	if permissions, err := queries.ListPermissions(ctx); err != nil || len(permissions) != 12 {
		t.Fatalf("ListPermissions() count = %d, %v", len(permissions), err)
	}
	if err := queries.DeleteUser(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.GetUserByID(ctx, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetUserByID() after delete error = %v", err)
	}
	missingPermissions, err := queries.ListUserPermissions(ctx, created.ID)
	if err != nil || len(missingPermissions) != 0 {
		t.Fatalf("ListUserPermissions() for missing user = %v, %v; want no permissions", missingPermissions, err)
	}
	if err := authorization.Authorize(ctx, created.ID.String(), "tasks:write"); !errors.Is(err, iam.ErrPermissionDenied) {
		t.Fatalf("Authorize() missing user error = %v, want permission denied", err)
	}
}

func exerciseIAMManagement(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	queries := dbgen.New(conn)
	store := iamstore.New(queries, conn)
	adminRole, err := queries.GetRoleByName(ctx, "admin")
	if err != nil || !adminRole.IsSystem {
		t.Fatalf("admin system role = %#v, %v", adminRole, err)
	}
	adminUser, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "management-admin@example.com", Username: "management-admin", PasswordHash: "hash", DisplayName: "Management Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.AssignUserRole(ctx, dbgen.AssignUserRoleParams{UserID: adminUser.ID, RoleID: adminRole.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserActive(ctx, adminUser.ID.String(), false); !errors.Is(err, iam.ErrLastActiveAdministrator) {
		t.Fatalf("SetUserActive(last admin) error = %v", err)
	}
	if err := store.ReplaceUserRoles(ctx, adminUser.ID.String(), []string{}); !errors.Is(err, iam.ErrLastActiveAdministrator) {
		t.Fatalf("ReplaceUserRoles(last admin) error = %v", err)
	}
	if _, err := store.UpdateRole(ctx, adminRole.ID.String(), iam.RoleInput{Name: "renamed-admin", Description: "no"}); !errors.Is(err, iam.ErrSystemRoleProtected) {
		t.Fatalf("UpdateRole(system rename) error = %v", err)
	}
	if err := store.DeleteRole(ctx, adminRole.ID.String()); !errors.Is(err, iam.ErrSystemRoleProtected) {
		t.Fatalf("DeleteRole(system) error = %v", err)
	}
	if err := store.ReplaceRolePermissions(ctx, adminRole.ID.String(), []string{}); !errors.Is(err, iam.ErrSystemRoleProtected) {
		t.Fatalf("ReplaceRolePermissions(system) error = %v", err)
	}

	customRole, err := store.CreateRole(ctx, iam.RoleInput{Name: "management-operators", Description: "Management operators"})
	if err != nil {
		t.Fatal(err)
	}
	permission, err := queries.GetPermissionByName(ctx, "tasks:read")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceRolePermissions(ctx, customRole.ID, []string{permission.ID.String()}); err != nil {
		t.Fatal(err)
	}
	detail, err := store.Role(ctx, customRole.ID)
	if err != nil || detail.IsSystem || len(detail.Permissions) != 1 || detail.Permissions[0].Name != "tasks:read" {
		t.Fatalf("custom role detail = %+v, %v", detail, err)
	}
	member, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email: "management-member@example.com", Username: "management-member", PasswordHash: "hash", DisplayName: "Management Member",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceUserRoles(ctx, member.ID.String(), []string{customRole.ID}); err != nil {
		t.Fatal(err)
	}
	roles, err := queries.ListRolesForUser(ctx, member.ID)
	if err != nil || len(roles) != 1 || roles[0].ID.String() != customRole.ID {
		t.Fatalf("replacement roles = %+v, %v", roles, err)
	}
}
