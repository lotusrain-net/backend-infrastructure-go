CREATE TABLE task_outbox_messages (
    queue_id TEXT PRIMARY KEY,
    execution_id UUID NOT NULL UNIQUE REFERENCES task_executions(id) ON DELETE CASCADE,
    task_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    max_retries INTEGER NOT NULL DEFAULT 0,
    timeout_seconds INTEGER NOT NULL DEFAULT 0,
    unique_for_seconds INTEGER NOT NULL DEFAULT 0,
    process_after_seconds INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT task_outbox_queue_not_blank CHECK (btrim(queue_id) <> ''),
    CONSTRAINT task_outbox_type_not_blank CHECK (btrim(task_type) <> ''),
    CONSTRAINT task_outbox_retries_valid CHECK (max_retries >= 0),
    CONSTRAINT task_outbox_timeout_valid CHECK (timeout_seconds >= 0),
    CONSTRAINT task_outbox_unique_for_valid CHECK (unique_for_seconds >= 0),
    CONSTRAINT task_outbox_process_after_valid CHECK (process_after_seconds >= 0)
);

CREATE INDEX task_outbox_pending_idx
    ON task_outbox_messages (created_at, queue_id)
    WHERE published_at IS NULL;
