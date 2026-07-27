-- name: ListUserPermissions :many
SELECT DISTINCT permissions.name
FROM permissions
JOIN role_permissions ON role_permissions.permission_id = permissions.id
JOIN user_roles ON user_roles.role_id = role_permissions.role_id
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.user_id = $1 AND users.is_active = TRUE
ORDER BY permissions.name;

-- name: AssignUserRole :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: GrantRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- name: GetRoleByName :one
SELECT * FROM roles WHERE name = $1;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: GetSystemAdminRole :one
SELECT * FROM roles WHERE name = 'admin' AND is_system = TRUE;

-- name: LockSystemAdminRole :one
SELECT * FROM roles WHERE name = 'admin' AND is_system = TRUE FOR UPDATE;

-- name: GetPermissionByName :one
SELECT * FROM permissions WHERE name = $1;

-- name: GetPermissionByID :one
SELECT * FROM permissions WHERE id = $1;

-- name: ListRoles :many
SELECT * FROM roles ORDER BY name;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY name;

-- name: ListRolesForUser :many
SELECT roles.*
FROM roles
JOIN user_roles ON user_roles.role_id = roles.id
WHERE user_roles.user_id = $1
ORDER BY roles.name;

-- name: ListRolePermissions :many
SELECT permissions.*
FROM permissions
JOIN role_permissions ON role_permissions.permission_id = permissions.id
WHERE role_permissions.role_id = $1
ORDER BY permissions.name;

-- name: CreateRole :one
INSERT INTO roles (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateRole :one
UPDATE roles
SET name = $2, description = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteRole :execrows
DELETE FROM roles WHERE id = $1;

-- name: DeleteUserRoles :exec
DELETE FROM user_roles WHERE user_id = $1;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: LockActiveSystemAdministratorIDs :many
SELECT users.id
FROM users
JOIN user_roles ON user_roles.user_id = users.id
JOIN roles ON roles.id = user_roles.role_id
WHERE users.is_active = TRUE
  AND roles.name = 'admin'
  AND roles.is_system = TRUE
FOR UPDATE OF users;
