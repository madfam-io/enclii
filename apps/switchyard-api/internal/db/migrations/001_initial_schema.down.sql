-- Drop triggers
DROP TRIGGER IF EXISTS update_deployments_updated_at ON deployments;
DROP TRIGGER IF EXISTS update_releases_updated_at ON releases;
DROP TRIGGER IF EXISTS update_services_updated_at ON services;
DROP TRIGGER IF EXISTS update_environments_updated_at ON environments;
DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_deployments_environment_id;
DROP INDEX IF EXISTS idx_deployments_release_id;
DROP INDEX IF EXISTS idx_releases_service_id;
DROP INDEX IF EXISTS idx_services_project_id;
DROP INDEX IF EXISTS idx_environments_project_id;

-- Drop tables in reverse order due to foreign key constraints
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS projects;