CREATE TABLE IF NOT EXISTS notification_outbox (
    id SERIAL PRIMARY KEY,
    kind VARCHAR(64) NOT NULL,
    incident_id INTEGER NOT NULL REFERENCES incident(id),
    payload JSONB NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    change_id UUID NOT NULL,
    deduplication_key VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_by VARCHAR(255),
    locked_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_outbox_dispatch 
    ON notification_outbox (next_attempt_at) 
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_stale_processing 
    ON notification_outbox (locked_at) 
    WHERE status = 'processing';

CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_deduplication 
    ON notification_outbox (deduplication_key);

CREATE TABLE IF NOT EXISTS notification_log (
    id SERIAL PRIMARY KEY,
    outbox_id INTEGER NOT NULL REFERENCES notification_outbox(id),
    incident_id INTEGER NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    error TEXT,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_log_outbox_id ON notification_log (outbox_id);
CREATE INDEX IF NOT EXISTS idx_log_attempted_at ON notification_log (attempted_at);
