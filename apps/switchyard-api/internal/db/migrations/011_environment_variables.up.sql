-- Migration: Environment Variables & Secrets Management
-- Description: Add environment variables table with encryption support for secrets

-- Environment variables table
-- Supports per-service and per-environment variable configuration
CREATE TABLE IF NOT EXISTS environment_variables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE CASCADE, -- NULL = applies to all environments
    key VARCHAR(255) NOT NULL,
    value_encrypted TEXT NOT NULL, -- AES-256-GCM encrypted value
    is_secret BOOLEAN NOT NULL DEFAULT false, -- If true, value is masked in UI
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID, -- User who created this variable (can be NULL for OIDC users)
    created_by_email VARCHAR(255), -- Email of creator for audit trail

    -- Unique constraint: one key per service+environment combination
    -- (NULL environment_id means "all environments")
    CONSTRAINT unique_env_var_key UNIQUE (service_id, environment_id, key)
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_env_vars_service ON environment_variables(service_id);
CREATE INDEX IF NOT EXISTS idx_env_vars_environment ON environment_variables(environment_id);
CREATE INDEX IF NOT EXISTS idx_env_vars_service_env ON environment_variables(service_id, environment_id);
CREATE INDEX IF NOT EXISTS idx_env_vars_key ON environment_variables(key);

-- Environment variable audit log for tracking changes
CREATE TABLE IF NOT EXISTS env_var_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    env_var_id UUID NOT NULL,
    service_id UUID NOT NULL,
    environment_id UUID,
    action VARCHAR(50) NOT NULL, -- 'created', 'updated', 'deleted', 'revealed'
    key VARCHAR(255) NOT NULL,
    old_value_hash VARCHAR(64), -- SHA-256 hash of old value (for change detection, not recovery)
    new_value_hash VARCHAR(64), -- SHA-256 hash of new value
    actor_id UUID,
    actor_email VARCHAR(255) NOT NULL,
    actor_ip VARCHAR(45),
    user_agent TEXT,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_env_var_audit_env_var ON env_var_audit_logs(env_var_id);
CREATE INDEX IF NOT EXISTS idx_env_var_audit_service ON env_var_audit_logs(service_id);
CREATE INDEX IF NOT EXISTS idx_env_var_audit_timestamp ON env_var_audit_logs(timestamp);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_env_var_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update timestamp on modification
DROP TRIGGER IF EXISTS trigger_env_var_updated_at ON environment_variables;
CREATE TRIGGER trigger_env_var_updated_at
    BEFORE UPDATE ON environment_variables
    FOR EACH ROW
    EXECUTE FUNCTION update_env_var_updated_at();

-- Add comments for documentation
COMMENT ON TABLE environment_variables IS 'Service environment variables and secrets with encryption at rest';
COMMENT ON COLUMN environment_variables.value_encrypted IS 'AES-256-GCM encrypted value, base64 encoded';
COMMENT ON COLUMN environment_variables.is_secret IS 'If true, value is masked in API responses and UI';
COMMENT ON COLUMN environment_variables.environment_id IS 'NULL means variable applies to all environments';
COMMENT ON TABLE env_var_audit_logs IS 'Immutable audit trail for environment variable changes';
