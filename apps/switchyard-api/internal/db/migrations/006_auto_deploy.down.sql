-- Rollback auto-deploy configuration from services

DROP INDEX IF EXISTS idx_services_auto_deploy;

ALTER TABLE services
    DROP COLUMN IF EXISTS auto_deploy,
    DROP COLUMN IF EXISTS auto_deploy_branch,
    DROP COLUMN IF EXISTS auto_deploy_env;
