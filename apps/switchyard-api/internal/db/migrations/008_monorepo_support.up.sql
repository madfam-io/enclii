-- Migration: 008_monorepo_support
-- Add app_path column for monorepo subdirectory support
-- This allows multiple services from the same git_repo with different app paths

-- Add app_path column to services table
ALTER TABLE services ADD COLUMN IF NOT EXISTS app_path VARCHAR(500) DEFAULT '';

-- Update unique constraint to include app_path
-- This allows: repo=github.com/org/monorepo, app_path=apps/api
--          and: repo=github.com/org/monorepo, app_path=apps/web
-- While still preventing: same project, same name
ALTER TABLE services DROP CONSTRAINT IF EXISTS services_project_id_name_key;
ALTER TABLE services ADD CONSTRAINT services_project_id_name_key UNIQUE (project_id, name);

-- Create index for efficient webhook lookups (git_repo + app_path)
CREATE INDEX IF NOT EXISTS idx_services_git_repo_app_path ON services(git_repo, app_path);

-- Comment for documentation
COMMENT ON COLUMN services.app_path IS 'Monorepo subdirectory path (e.g., apps/api, packages/web). Empty string means root of repository.';
