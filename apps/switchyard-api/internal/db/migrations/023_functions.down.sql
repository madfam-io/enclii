-- Migration 023: Serverless Functions (Rollback)
-- Drops all tables and objects created for serverless functions

-- Drop triggers first
DROP TRIGGER IF EXISTS trigger_update_functions_updated_at ON functions;

-- Drop function for trigger
DROP FUNCTION IF EXISTS update_functions_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_functions_project_id;
DROP INDEX IF EXISTS idx_functions_status;
DROP INDEX IF EXISTS idx_functions_created_by;
DROP INDEX IF EXISTS idx_functions_name;
DROP INDEX IF EXISTS idx_functions_deleted_at;

DROP INDEX IF EXISTS idx_function_invocations_function_id;
DROP INDEX IF EXISTS idx_function_invocations_started_at;
DROP INDEX IF EXISTS idx_function_invocations_cold_start;
DROP INDEX IF EXISTS idx_function_invocations_request_id;

DROP INDEX IF EXISTS idx_function_metrics_function_id;
DROP INDEX IF EXISTS idx_function_metrics_period;

-- Drop tables (in correct order due to foreign key constraints)
DROP TABLE IF EXISTS function_metrics CASCADE;
DROP TABLE IF EXISTS function_invocations CASCADE;
DROP TABLE IF EXISTS functions CASCADE;
