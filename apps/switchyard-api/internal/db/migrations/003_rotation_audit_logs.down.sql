-- Drop rotation_audit_logs table
DROP INDEX IF EXISTS idx_rotation_audit_logs_service_started;
DROP INDEX IF EXISTS idx_rotation_audit_logs_started_at;
DROP INDEX IF EXISTS idx_rotation_audit_logs_status;
DROP INDEX IF EXISTS idx_rotation_audit_logs_event_id;
DROP INDEX IF EXISTS idx_rotation_audit_logs_service_id;
DROP TABLE IF EXISTS rotation_audit_logs;
