-- Migration: 010_deployment_groups (down)
-- Rollback deployment groups and service dependencies

-- Remove deployment group columns from deployments
ALTER TABLE deployments DROP COLUMN IF EXISTS deploy_order;
ALTER TABLE deployments DROP COLUMN IF EXISTS group_id;

-- Drop triggers
DROP TRIGGER IF EXISTS update_deployment_groups_updated_at ON deployment_groups;

-- Drop indexes
DROP INDEX IF EXISTS idx_service_dependencies_depends_on;
DROP INDEX IF EXISTS idx_service_dependencies_service;
DROP INDEX IF EXISTS idx_deployments_deploy_order;
DROP INDEX IF EXISTS idx_deployments_group;
DROP INDEX IF EXISTS idx_deployment_groups_created_at;
DROP INDEX IF EXISTS idx_deployment_groups_status;
DROP INDEX IF EXISTS idx_deployment_groups_environment;
DROP INDEX IF EXISTS idx_deployment_groups_project;

-- Drop tables
DROP TABLE IF EXISTS service_dependencies;
DROP TABLE IF EXISTS deployment_groups;
