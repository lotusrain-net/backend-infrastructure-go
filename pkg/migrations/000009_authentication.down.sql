DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE name IN ('system-settings:read','system-settings:write'));
DELETE FROM permissions WHERE name IN ('system-settings:read','system-settings:write');
DROP TABLE authentication_consumptions;
DROP TABLE user_security_settings;
DROP TABLE authentication_settings;
ALTER TABLE users DROP COLUMN email_verified_at;
