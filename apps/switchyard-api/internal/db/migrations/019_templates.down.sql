-- Drop trigger first
DROP TRIGGER IF EXISTS trigger_increment_deploy_count ON template_deployments;
DROP FUNCTION IF EXISTS increment_template_deploy_count();

-- Drop indexes
DROP INDEX IF EXISTS idx_template_deployments_user;
DROP INDEX IF EXISTS idx_template_deployments_project;
DROP INDEX IF EXISTS idx_template_deployments_template;
DROP INDEX IF EXISTS idx_templates_deploy_count;
DROP INDEX IF EXISTS idx_templates_is_official;
DROP INDEX IF EXISTS idx_templates_is_featured;
DROP INDEX IF EXISTS idx_templates_language;
DROP INDEX IF EXISTS idx_templates_framework;
DROP INDEX IF EXISTS idx_templates_category;
DROP INDEX IF EXISTS idx_templates_slug;

-- Drop tables
DROP TABLE IF EXISTS template_deployments;
DROP TABLE IF EXISTS templates;
