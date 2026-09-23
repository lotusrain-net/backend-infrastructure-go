ALTER TABLE roles ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE roles
SET is_system = TRUE
WHERE name IN ('admin', 'user');
