-- Create rotation_audit_logs table for secret rotation tracking
CREATE TABLE IF NOT EXISTS rotation_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    service_name VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL, -- 'production', 'staging', 'dev'
    secret_name VARCHAR(255) NOT NULL,
    secret_path TEXT NOT NULL,
    old_version INTEGER NOT NULL,
    new_version INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL, -- 'pending', 'in_progress', 'completed', 'failed', 'rolled_back'
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT, -- Duration in milliseconds
    rollout_strategy VARCHAR(50), -- 'rolling', 'recreate', 'blue-green'
    pods_restarted INTEGER DEFAULT 0,
    error TEXT,
    changed_by VARCHAR(255),
    triggered_by VARCHAR(50) NOT NULL, -- 'watcher', 'webhook', 'manual'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for service queries
CREATE INDEX idx_rotation_audit_logs_service_id ON rotation_audit_logs(service_id);

-- Create index for event queries
CREATE INDEX idx_rotation_audit_logs_event_id ON rotation_audit_logs(event_id);

-- Create index for status queries
CREATE INDEX idx_rotation_audit_logs_status ON rotation_audit_logs(status);

-- Create index for time-based queries
CREATE INDEX idx_rotation_audit_logs_started_at ON rotation_audit_logs(started_at DESC);

-- Create composite index for service history queries
CREATE INDEX idx_rotation_audit_logs_service_started ON rotation_audit_logs(service_id, started_at DESC);
