-- name: ListUserPermissions :many
SELECT DISTINCT permissions.name
FROM permissions
JOIN role_permissions ON role_permissions.permission_id = permissions.id
JOIN user_roles ON user_roles.role_id = role_permissions.role_id
WHERE user_roles.user_id = $1
ORDER BY permissions.name;

-- name: AssignUserRole :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: GrantRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- name: ListRoles :many
SELECT * FROM roles ORDER BY name;

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY name;
