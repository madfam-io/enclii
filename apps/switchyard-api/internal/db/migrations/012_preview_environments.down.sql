-- Migration: 012_preview_environments (DOWN)
-- Rollback preview environments feature

-- Remove preview_environment_id from deployments
ALTER TABLE deployments DROP COLUMN IF EXISTS preview_environment_id;

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS preview_access_logs;
DROP TABLE IF EXISTS preview_comments;
DROP TABLE IF EXISTS preview_environments;
