-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    request_id, actor_id, action, result, resource_type, resource_id,
    ip_address, user_agent, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListFilteredAuditLogs :many
SELECT * FROM audit_logs
WHERE (sqlc.narg('request_id')::text IS NULL OR request_id = sqlc.narg('request_id'))
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('result')::text IS NULL OR result = sqlc.narg('result'))
  AND (sqlc.narg('resource_type')::text IS NULL OR resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('resource_id')::text IS NULL OR resource_id = sqlc.narg('resource_id'))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR created_at <= sqlc.narg('to_time'))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountFilteredAuditLogs :one
SELECT count(*) FROM audit_logs
WHERE (sqlc.narg('request_id')::text IS NULL OR request_id = sqlc.narg('request_id'))
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('result')::text IS NULL OR result = sqlc.narg('result'))
  AND (sqlc.narg('resource_type')::text IS NULL OR resource_type = sqlc.narg('resource_type'))
  AND (sqlc.narg('resource_id')::text IS NULL OR resource_id = sqlc.narg('resource_id'))
  AND (sqlc.narg('from_time')::timestamptz IS NULL OR created_at >= sqlc.narg('from_time'))
  AND (sqlc.narg('to_time')::timestamptz IS NULL OR created_at <= sqlc.narg('to_time'));
