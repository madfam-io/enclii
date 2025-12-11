-- Rollback: 008_monorepo_support
-- Remove monorepo support

-- Drop index
DROP INDEX IF EXISTS idx_services_git_repo_app_path;

-- Remove app_path column
ALTER TABLE services DROP COLUMN IF EXISTS app_path;
