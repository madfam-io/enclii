-- Enclii Consolidated Schema Migration
-- This single migration creates the complete database schema for Enclii
-- Consolidates migrations 001-005 with OIDC-compatible modifications
--
-- Key changes from original migrations:
-- - audit_logs.actor_id is nullable (supports OIDC users without local user row)
-- - All tables use IF NOT EXISTS for idempotent deployment

-- ============================================================================
-- SECTION 1: Core Platform Tables (from 001_initial_schema)
-- ============================================================================

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Environments table
CREATE TABLE IF NOT EXISTS environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    kube_namespace VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, name)
);

-- Services table
CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    git_repo VARCHAR(500) NOT NULL,
    build_config JSONB NOT NULL DEFAULT '{}',
    volumes JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, name)
);

-- Releases table
CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    version VARCHAR(255) NOT NULL,
    image_uri VARCHAR(500) NOT NULL,
    git_sha VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'building',
    sbom TEXT,
    sbom_format VARCHAR(50),
    image_signature TEXT,
    signature_verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(service_id, version)
);

-- Deployments table
CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    replicas INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    health VARCHAR(50) NOT NULL DEFAULT 'unknown',
    deployed_by UUID,
    pr_url VARCHAR(500),
    commit_message TEXT,
    sbom TEXT,
    image_signature TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(release_id, environment_id)
);

-- ============================================================================
-- SECTION 2: Users and Access Control (from 002_compliance_schema + 005_oidc_support)
-- ============================================================================

-- Users table (supports both local and OIDC users)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'developer',
    oidc_subject TEXT,
    oidc_issuer TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

COMMENT ON COLUMN users.role IS 'User role for RBAC: admin, developer, or viewer';
COMMENT ON COLUMN users.oidc_subject IS 'OIDC subject identifier (sub claim) from external provider';
COMMENT ON COLUMN users.oidc_issuer IS 'OIDC issuer URL (iss claim) from external provider';

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
    role VARCHAR(50) NOT NULL,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

-- Project access table (environment-specific permissions)
CREATE TABLE IF NOT EXISTS project_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    granted_by UUID NOT NULL REFERENCES users(id),
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, project_id, environment_id)
);

-- ============================================================================
-- SECTION 3: Audit and Compliance (from 002_compliance_schema)
-- Modified: actor_id is nullable for OIDC compatibility
-- ============================================================================

-- Audit logs table (immutable) - actor_id nullable for OIDC users
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    actor_id UUID REFERENCES users(id),
    actor_email VARCHAR(255) NOT NULL,
    actor_role VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    environment_id UUID REFERENCES environments(id) ON DELETE SET NULL,
    ip_address INET,
    user_agent TEXT,
    outcome VARCHAR(20) NOT NULL,
    context JSONB,
    metadata JSONB
);

-- Sessions table (for token revocation and session management)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
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
    ci_status VARCHAR(50),
    change_ticket_url VARCHAR(500),
    compliance_receipt TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- SECTION 4: Secret Rotation Audit (from 003_rotation_audit_logs)
-- ============================================================================

CREATE TABLE IF NOT EXISTS rotation_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    service_name VARCHAR(255) NOT NULL,
    environment VARCHAR(50) NOT NULL,
    secret_name VARCHAR(255) NOT NULL,
    secret_path TEXT NOT NULL,
    old_version INTEGER NOT NULL,
    new_version INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,
    rollout_strategy VARCHAR(50),
    pods_restarted INTEGER DEFAULT 0,
    error TEXT,
    changed_by VARCHAR(255),
    triggered_by VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- SECTION 5: Custom Domains and Routes (from 004_custom_domains_routes)
-- ============================================================================

CREATE TABLE IF NOT EXISTS custom_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    tls_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    tls_issuer VARCHAR(100) DEFAULT 'letsencrypt-prod',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMP,
    UNIQUE(service_id, environment_id, domain)
);

CREATE TABLE IF NOT EXISTS routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    path VARCHAR(500) NOT NULL,
    path_type VARCHAR(50) NOT NULL DEFAULT 'Prefix',
    port INTEGER NOT NULL DEFAULT 80,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(service_id, environment_id, path)
);

-- ============================================================================
-- SECTION 6: Indexes
-- ============================================================================

-- Core platform indexes
CREATE INDEX IF NOT EXISTS idx_environments_project_id ON environments(project_id);
CREATE INDEX IF NOT EXISTS idx_services_project_id ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_releases_service_id ON releases(service_id);
CREATE INDEX IF NOT EXISTS idx_deployments_release_id ON deployments(release_id);
CREATE INDEX IF NOT EXISTS idx_deployments_environment_id ON deployments(environment_id);

-- User and access indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_oidc_subject ON users(oidc_subject);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_identity ON users(oidc_issuer, oidc_subject)
    WHERE oidc_issuer IS NOT NULL AND oidc_subject IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_project_access_user_id ON project_access(user_id);
CREATE INDEX IF NOT EXISTS idx_project_access_project_id ON project_access(project_id);
CREATE INDEX IF NOT EXISTS idx_project_access_environment_id ON project_access(environment_id);

-- Audit log indexes
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_project_id ON audit_logs(project_id);

-- Session indexes
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- Approval record indexes
CREATE INDEX IF NOT EXISTS idx_approval_records_deployment_id ON approval_records(deployment_id);

-- Rotation audit log indexes
CREATE INDEX IF NOT EXISTS idx_rotation_audit_logs_service_id ON rotation_audit_logs(service_id);
CREATE INDEX IF NOT EXISTS idx_rotation_audit_logs_event_id ON rotation_audit_logs(event_id);
CREATE INDEX IF NOT EXISTS idx_rotation_audit_logs_status ON rotation_audit_logs(status);
CREATE INDEX IF NOT EXISTS idx_rotation_audit_logs_started_at ON rotation_audit_logs(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_rotation_audit_logs_service_started ON rotation_audit_logs(service_id, started_at DESC);

-- Custom domains and routes indexes
CREATE INDEX IF NOT EXISTS idx_custom_domains_service_id ON custom_domains(service_id);
CREATE INDEX IF NOT EXISTS idx_custom_domains_environment_id ON custom_domains(environment_id);
CREATE INDEX IF NOT EXISTS idx_custom_domains_domain ON custom_domains(domain);
CREATE INDEX IF NOT EXISTS idx_routes_service_id ON routes(service_id);
CREATE INDEX IF NOT EXISTS idx_routes_environment_id ON routes(environment_id);

-- ============================================================================
-- SECTION 7: Functions and Triggers
-- ============================================================================

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_projects_updated_at') THEN
        CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_environments_updated_at') THEN
        CREATE TRIGGER update_environments_updated_at BEFORE UPDATE ON environments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_services_updated_at') THEN
        CREATE TRIGGER update_services_updated_at BEFORE UPDATE ON services FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_releases_updated_at') THEN
        CREATE TRIGGER update_releases_updated_at BEFORE UPDATE ON releases FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_deployments_updated_at') THEN
        CREATE TRIGGER update_deployments_updated_at BEFORE UPDATE ON deployments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_users_updated_at') THEN
        CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_teams_updated_at') THEN
        CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

-- ============================================================================
-- SECTION 8: Row-Level Security for Audit Logs
-- ============================================================================

-- Immutability policy for audit_logs (PostgreSQL row-level security)
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

-- Create policies if they don't exist
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'audit_log_immutable_update') THEN
        CREATE POLICY audit_log_immutable_update ON audit_logs FOR UPDATE USING (false);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'audit_log_immutable_delete') THEN
        CREATE POLICY audit_log_immutable_delete ON audit_logs FOR DELETE USING (false);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'audit_log_allow_insert') THEN
        CREATE POLICY audit_log_allow_insert ON audit_logs FOR INSERT WITH CHECK (true);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE policyname = 'audit_log_allow_select') THEN
        CREATE POLICY audit_log_allow_select ON audit_logs FOR SELECT USING (true);
    END IF;
END $$;

-- ============================================================================
-- SECTION 9: Views
-- ============================================================================

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

-- ============================================================================
-- SECTION 10: Helper Functions
-- ============================================================================

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
    SELECT pa.role INTO user_role
    FROM project_access pa
    WHERE pa.user_id = p_user_id
      AND pa.project_id = p_project_id
      AND (pa.environment_id = p_environment_id OR pa.environment_id IS NULL)
      AND (pa.expires_at IS NULL OR pa.expires_at > NOW())
    LIMIT 1;

    IF user_role IS NULL THEN
        RETURN false;
    END IF;

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

    RETURN role_hierarchy >= required_hierarchy;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- Migration complete
-- ============================================================================
