-- Drop views
DROP VIEW IF EXISTS user_permissions;
DROP VIEW IF EXISTS active_sessions;

-- Drop function
DROP FUNCTION IF EXISTS user_has_access(UUID, UUID, UUID, VARCHAR);

-- Disable row-level security and drop policies
ALTER TABLE IF EXISTS audit_logs DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS audit_log_allow_insert ON audit_logs;
DROP POLICY IF EXISTS audit_log_immutable_delete ON audit_logs;
DROP POLICY IF EXISTS audit_log_immutable_update ON audit_logs;

-- Drop triggers
DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop indexes
DROP INDEX IF EXISTS idx_approval_records_deployment_id;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_token_hash;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP INDEX IF EXISTS idx_audit_logs_project_id;
DROP INDEX IF EXISTS idx_audit_logs_resource_type;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_actor_id;
DROP INDEX IF EXISTS idx_audit_logs_timestamp;
DROP INDEX IF EXISTS idx_project_access_environment_id;
DROP INDEX IF EXISTS idx_project_access_project_id;
DROP INDEX IF EXISTS idx_project_access_user_id;
DROP INDEX IF EXISTS idx_team_members_user_id;
DROP INDEX IF EXISTS idx_team_members_team_id;
DROP INDEX IF EXISTS idx_users_oidc_sub;
DROP INDEX IF EXISTS idx_users_email;

-- Remove provenance fields from existing tables
ALTER TABLE releases
    DROP COLUMN IF EXISTS signature_verified_at,
    DROP COLUMN IF EXISTS image_signature,
    DROP COLUMN IF EXISTS sbom_format,
    DROP COLUMN IF EXISTS sbom;

ALTER TABLE deployments
    DROP COLUMN IF EXISTS image_signature,
    DROP COLUMN IF EXISTS sbom,
    DROP COLUMN IF EXISTS commit_message,
    DROP COLUMN IF EXISTS pr_url,
    DROP COLUMN IF EXISTS deployed_by;

-- Drop new tables (in reverse order of creation to respect foreign keys)
DROP TABLE IF EXISTS approval_records;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS project_access;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;
