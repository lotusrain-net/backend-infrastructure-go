-- name: CreateTaskDefinition :one
INSERT INTO task_definitions (
    name, task_type, description, default_payload, max_retries, timeout_seconds
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateTaskExecution :one
INSERT INTO task_executions (definition_id, task_type, queue_id, idempotency_key, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreateTaskExecutionWithPendingPublish :one
WITH execution AS (
    INSERT INTO task_executions (definition_id, task_type, queue_id, idempotency_key, payload)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
), outbox AS (
    INSERT INTO task_outbox_messages (
        execution_id, queue_id, task_type, payload, max_retries, timeout_seconds, unique_for_seconds, process_after_seconds
    )
    SELECT execution.id, $3, $2, $5, $6, $7, $8, $9
    FROM execution
)
SELECT * FROM execution;

-- name: GetTaskExecution :one
SELECT * FROM task_executions WHERE id = $1;

-- name: ClaimTaskExecution :execrows
UPDATE task_executions
SET status = 'running', started_at = COALESCE(started_at, $3), attempt = $2, updated_at = NOW()
WHERE id = $1 AND status IN ('queued', 'running') AND attempt < $2;

-- name: UpdateTaskExecutionStatus :execrows
UPDATE task_executions
SET status = sqlc.arg(status),
    started_at = COALESCE(sqlc.narg(started_at), started_at),
    finished_at = COALESCE(sqlc.narg(finished_at), finished_at),
    error_summary = sqlc.narg(error_summary),
    processed_rows = sqlc.arg(processed_rows),
    attempt = sqlc.arg(attempt),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = sqlc.arg(expected_status)
  AND attempt = sqlc.arg(expected_attempt);

-- name: ListPendingTaskOutboxMessages :many
SELECT outbox.execution_id, executions.status AS execution_status,
       outbox.queue_id, outbox.task_type, outbox.payload, outbox.max_retries,
       outbox.timeout_seconds, outbox.unique_for_seconds, outbox.process_after_seconds
FROM task_outbox_messages AS outbox
JOIN task_executions AS executions ON executions.id = outbox.execution_id
WHERE outbox.published_at IS NULL
ORDER BY outbox.created_at, outbox.queue_id
LIMIT $1;

-- name: MarkTaskOutboxMessagePublished :exec
UPDATE task_outbox_messages
SET published_at = NOW(), updated_at = NOW()
WHERE queue_id = $1 AND published_at IS NULL;

-- name: ListEnabledTaskSchedules :many
SELECT task_schedules.*, task_definitions.task_type, task_definitions.max_retries,
       task_definitions.timeout_seconds
FROM task_schedules
JOIN task_definitions ON task_definitions.id = task_schedules.definition_id
WHERE task_schedules.is_enabled = TRUE AND task_definitions.is_active = TRUE
ORDER BY task_schedules.created_at, task_schedules.id;
