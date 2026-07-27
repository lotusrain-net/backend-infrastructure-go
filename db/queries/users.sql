-- name: CreateUser :one
INSERT INTO users (email, username, password_hash, display_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users
WHERE (sqlc.narg('query')::text IS NULL
    OR email ILIKE '%' || sqlc.narg('query') || '%'
    OR username ILIKE '%' || sqlc.narg('query') || '%'
    OR display_name ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('active')::boolean IS NULL OR is_active = sqlc.narg('active'))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE (sqlc.narg('query')::text IS NULL
    OR email ILIKE '%' || sqlc.narg('query') || '%'
    OR username ILIKE '%' || sqlc.narg('query') || '%'
    OR display_name ILIKE '%' || sqlc.narg('query') || '%')
  AND (sqlc.narg('active')::boolean IS NULL OR is_active = sqlc.narg('active'));

-- name: UpdateUserProfile :one
UPDATE users
SET email = $2, username = $3, display_name = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :execrows
UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1;

-- name: SetUserActive :execrows
UPDATE users SET is_active = $2, updated_at = NOW() WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
