-- Rollback: Environment Variables & Secrets Management

-- Drop trigger first
DROP TRIGGER IF EXISTS trigger_env_var_updated_at ON environment_variables;

-- Drop function
DROP FUNCTION IF EXISTS update_env_var_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_env_var_audit_timestamp;
DROP INDEX IF EXISTS idx_env_var_audit_service;
DROP INDEX IF EXISTS idx_env_var_audit_env_var;
DROP INDEX IF EXISTS idx_env_vars_key;
DROP INDEX IF EXISTS idx_env_vars_service_env;
DROP INDEX IF EXISTS idx_env_vars_environment;
DROP INDEX IF EXISTS idx_env_vars_service;

-- Drop tables
DROP TABLE IF EXISTS env_var_audit_logs;
DROP TABLE IF EXISTS environment_variables;
