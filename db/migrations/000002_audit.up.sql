CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id TEXT NOT NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    result TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    ip_address INET,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_logs_request_id_not_blank CHECK (btrim(request_id) <> ''),
    CONSTRAINT audit_logs_action_not_blank CHECK (btrim(action) <> ''),
    CONSTRAINT audit_logs_result_valid CHECK (result IN ('success', 'failure')),
    CONSTRAINT audit_logs_resource_type_not_blank CHECK (btrim(resource_type) <> '')
);

CREATE INDEX audit_logs_actor_id_created_at_idx ON audit_logs(actor_id, created_at DESC);
CREATE INDEX audit_logs_resource_created_at_idx ON audit_logs(resource_type, resource_id, created_at DESC);
CREATE INDEX audit_logs_request_id_idx ON audit_logs(request_id);
