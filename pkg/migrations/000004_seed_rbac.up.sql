INSERT INTO roles (name, description)
VALUES
    ('admin', 'System administrator'),
    ('user', 'Standard authenticated user')
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO permissions (name, description)
VALUES
    ('*', 'All permissions'),
    ('users:read', 'Read users'),
    ('users:write', 'Manage users'),
    ('roles:read', 'Read roles and permissions'),
    ('roles:write', 'Manage roles and permissions'),
    ('audit:read', 'Read audit logs'),
    ('tasks:read', 'Read tasks and executions'),
    ('tasks:write', 'Manage tasks and executions'),
    ('schedules:read', 'Read task schedules'),
    ('schedules:write', 'Manage task schedules')
ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
CROSS JOIN permissions
WHERE roles.name = 'admin' AND permissions.name = '*'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles
JOIN permissions ON permissions.name IN ('users:read', 'tasks:read', 'schedules:read')
WHERE roles.name = 'user'
ON CONFLICT (role_id, permission_id) DO NOTHING;
