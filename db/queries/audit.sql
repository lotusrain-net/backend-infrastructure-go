-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    request_id, actor_id, action, result, resource_type, resource_id,
    ip_address, user_agent, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;
