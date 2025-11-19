-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255), -- NULL for OIDC-only users
    name VARCHAR(255) NOT NULL,
    oidc_sub VARCHAR(255) UNIQUE, -- OIDC subject identifier
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

-- Teams table
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Team members table (many-to-many relationship)
CREATE TABLE IF NOT EXISTS team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL, -- 'owner', 'member'
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

-- Project access table (environment-specific permissions)
CREATE TABLE IF NOT EXISTS project_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE CASCADE, -- NULL = all environments
    role VARCHAR(50) NOT NULL, -- 'admin', 'developer', 'viewer'
    granted_by UUID NOT NULL REFERENCES users(id),
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE, -- NULL = never expires
    UNIQUE(user_id, project_id, environment_id) -- User can have one role per project-environment combo
);

-- Audit logs table (immutable)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    actor_id UUID NOT NULL REFERENCES users(id),
    actor_email VARCHAR(255) NOT NULL,
    actor_role VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL, -- 'deploy', 'scale', 'delete', 'access_logs', 'create_service'
    resource_type VARCHAR(50) NOT NULL, -- 'service', 'environment', 'secret', 'deployment'
    resource_id VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    environment_id UUID REFERENCES environments(id) ON DELETE SET NULL,
    ip_address INET,
    user_agent TEXT,
    outcome VARCHAR(20) NOT NULL, -- 'success', 'failure', 'denied'
    context JSONB, -- {pr_url, commit_sha, approver, change_ticket}
    metadata JSONB
);

-- Sessions table (for token revocation and session management)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE, -- SHA256 of the refresh token
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Approval records table (deployment provenance)
CREATE TABLE IF NOT EXISTS approval_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    pr_url VARCHAR(500),
    pr_number INTEGER,
    approver_email VARCHAR(255),
    approver_name VARCHAR(255),
    approved_at TIMESTAMP WITH TIME ZONE,
    ci_status VARCHAR(50), -- 'passed', 'failed', 'pending'
    change_ticket_url VARCHAR(500),
    compliance_receipt TEXT, -- Signed JSON receipt for auditors
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add provenance fields to deployments table
ALTER TABLE deployments
    ADD COLUMN IF NOT EXISTS deployed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS pr_url VARCHAR(500),
    ADD COLUMN IF NOT EXISTS commit_message TEXT,
    ADD COLUMN IF NOT EXISTS sbom TEXT, -- Software Bill of Materials
    ADD COLUMN IF NOT EXISTS image_signature TEXT; -- Cosign signature

-- Add provenance fields to releases table
ALTER TABLE releases
    ADD COLUMN IF NOT EXISTS sbom TEXT,
    ADD COLUMN IF NOT EXISTS sbom_format VARCHAR(50), -- 'cyclonedx-json', 'spdx'
    ADD COLUMN IF NOT EXISTS image_signature TEXT,
    ADD COLUMN IF NOT EXISTS signature_verified_at TIMESTAMP WITH TIME ZONE;

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_oidc_sub ON users(oidc_sub);
CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_project_access_user_id ON project_access(user_id);
CREATE INDEX IF NOT EXISTS idx_project_access_project_id ON project_access(project_id);
CREATE INDEX IF NOT EXISTS idx_project_access_environment_id ON project_access(environment_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_project_id ON audit_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_approval_records_deployment_id ON approval_records(deployment_id);

-- Create triggers to automatically update updated_at for new tables
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Immutability policy for audit_logs (PostgreSQL row-level security)
-- Prevent updates and deletes on audit logs to ensure immutability
CREATE POLICY audit_log_immutable_update ON audit_logs FOR UPDATE USING (false);
CREATE POLICY audit_log_immutable_delete ON audit_logs FOR DELETE USING (false);

-- Enable row-level security on audit_logs
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

-- Allow inserts (but not updates/deletes)
CREATE POLICY audit_log_allow_insert ON audit_logs FOR INSERT WITH CHECK (true);

-- View for active sessions (non-revoked, non-expired)
CREATE OR REPLACE VIEW active_sessions AS
SELECT
    s.id,
    s.user_id,
    u.email,
    u.name,
    s.ip_address,
    s.user_agent,
    s.expires_at,
    s.created_at
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.revoked_at IS NULL
  AND s.expires_at > NOW();

-- View for user permissions (flattened for easy querying)
CREATE OR REPLACE VIEW user_permissions AS
SELECT
    u.id AS user_id,
    u.email,
    u.name,
    pa.project_id,
    p.slug AS project_slug,
    pa.environment_id,
    e.name AS environment_name,
    pa.role,
    pa.granted_at,
    pa.expires_at
FROM users u
JOIN project_access pa ON u.id = pa.user_id
JOIN projects p ON pa.project_id = p.id
LEFT JOIN environments e ON pa.environment_id = e.id
WHERE u.active = true
  AND (pa.expires_at IS NULL OR pa.expires_at > NOW());

-- Function to check if user has access to a project/environment
CREATE OR REPLACE FUNCTION user_has_access(
    p_user_id UUID,
    p_project_id UUID,
    p_environment_id UUID DEFAULT NULL,
    p_required_role VARCHAR DEFAULT 'viewer'
) RETURNS BOOLEAN AS $$
DECLARE
    user_role VARCHAR;
    role_hierarchy INT;
    required_hierarchy INT;
BEGIN
    -- Get user's role for this project/environment
    SELECT pa.role INTO user_role
    FROM project_access pa
    WHERE pa.user_id = p_user_id
      AND pa.project_id = p_project_id
      AND (pa.environment_id = p_environment_id OR pa.environment_id IS NULL)
      AND (pa.expires_at IS NULL OR pa.expires_at > NOW())
    LIMIT 1;

    -- If no access found, return false
    IF user_role IS NULL THEN
        RETURN false;
    END IF;

    -- Role hierarchy: admin > developer > viewer
    role_hierarchy := CASE user_role
        WHEN 'admin' THEN 3
        WHEN 'developer' THEN 2
        WHEN 'viewer' THEN 1
        ELSE 0
    END;

    required_hierarchy := CASE p_required_role
        WHEN 'admin' THEN 3
        WHEN 'developer' THEN 2
        WHEN 'viewer' THEN 1
        ELSE 0
    END;

    -- Check if user's role meets or exceeds required role
    RETURN role_hierarchy >= required_hierarchy;
END;
$$ LANGUAGE plpgsql;
