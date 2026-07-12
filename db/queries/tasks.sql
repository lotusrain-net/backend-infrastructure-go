-- name: CreateTaskDefinition :one
INSERT INTO task_definitions (
    name, task_type, description, default_payload, max_retries, timeout_seconds
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateTaskExecution :one
INSERT INTO task_executions (definition_id, task_type, queue_id, idempotency_key, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTaskExecution :one
SELECT * FROM task_executions WHERE id = $1;

-- name: UpdateTaskExecutionStatus :exec
UPDATE task_executions
SET status = $2, started_at = $3, finished_at = $4, error_summary = $5,
    processed_rows = $6, attempt = $7, updated_at = NOW()
WHERE id = $1;

-- name: ListEnabledTaskSchedules :many
SELECT * FROM task_schedules WHERE is_enabled = TRUE ORDER BY created_at, id;
