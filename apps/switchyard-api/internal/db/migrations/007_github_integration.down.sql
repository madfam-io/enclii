-- 007_github_integration.down.sql
-- Rollback GitHub App integration tables

-- Remove the foreign key column from services first
ALTER TABLE services DROP COLUMN IF EXISTS github_repo_id;

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_github_repositories_updated_at ON github_repositories;
DROP TRIGGER IF EXISTS trigger_github_installations_updated_at ON github_installations;

-- Drop trigger functions
DROP FUNCTION IF EXISTS update_github_repositories_updated_at();
DROP FUNCTION IF EXISTS update_github_installations_updated_at();

-- Drop tables in correct order (repositories references installations)
DROP TABLE IF EXISTS github_repositories;
DROP TABLE IF EXISTS github_installations;
