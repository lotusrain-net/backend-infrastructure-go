DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE name IN ('*', 'users:read', 'users:write', 'roles:read', 'roles:write', 'audit:read', 'tasks:read', 'tasks:write', 'schedules:read', 'schedules:write')
);

DELETE FROM permissions
WHERE name IN ('*', 'users:read', 'users:write', 'roles:read', 'roles:write', 'audit:read', 'tasks:read', 'tasks:write', 'schedules:read', 'schedules:write');

DELETE FROM roles WHERE name IN ('admin', 'user');
