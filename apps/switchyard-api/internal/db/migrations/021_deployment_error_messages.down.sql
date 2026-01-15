-- Rollback migration 021: Remove error_message columns

DROP INDEX IF EXISTS idx_releases_status_error;
DROP INDEX IF EXISTS idx_deployments_status_error;

ALTER TABLE releases DROP COLUMN IF EXISTS error_message;
ALTER TABLE deployments DROP COLUMN IF EXISTS error_message;
