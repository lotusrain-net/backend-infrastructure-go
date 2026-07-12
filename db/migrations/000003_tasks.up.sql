CREATE TABLE task_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    task_type TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    default_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    max_retries INTEGER NOT NULL DEFAULT 3,
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT task_definitions_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT task_definitions_type_not_blank CHECK (btrim(task_type) <> ''),
    CONSTRAINT task_definitions_max_retries_valid CHECK (max_retries >= 0),
    CONSTRAINT task_definitions_timeout_valid CHECK (timeout_seconds > 0)
);

CREATE TABLE task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id UUID REFERENCES task_definitions(id) ON DELETE SET NULL,
    task_type TEXT NOT NULL,
    queue_id TEXT,
    idempotency_key TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued',
    attempt INTEGER NOT NULL DEFAULT 0,
    processed_count BIGINT NOT NULL DEFAULT 0,
    error_summary TEXT,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT task_executions_type_not_blank CHECK (btrim(task_type) <> ''),
    CONSTRAINT task_executions_status_valid CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT task_executions_attempt_valid CHECK (attempt >= 0),
    CONSTRAINT task_executions_processed_count_valid CHECK (processed_count >= 0),
    CONSTRAINT task_executions_idempotency_unique UNIQUE (task_type, idempotency_key)
);

CREATE TABLE task_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id UUID NOT NULL REFERENCES task_definitions(id) ON DELETE CASCADE,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_enqueued_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT task_schedules_cron_not_blank CHECK (btrim(cron_expression) <> ''),
    CONSTRAINT task_schedules_timezone_not_blank CHECK (btrim(timezone) <> '')
);

CREATE INDEX task_executions_status_created_at_idx ON task_executions(status, created_at);
CREATE INDEX task_executions_queue_id_idx ON task_executions(queue_id);
CREATE INDEX task_schedules_enabled_idx ON task_schedules(is_enabled) WHERE is_enabled = TRUE;
