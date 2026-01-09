-- Rollback: Revert auto_deploy defaults
-- Note: We only revert the column defaults, not the data changes
-- This prevents breaking services that are actively using auto-deploy

-- Revert default auto_deploy to FALSE
ALTER TABLE services ALTER COLUMN auto_deploy SET DEFAULT FALSE;

-- Revert default auto_deploy_env to 'development'
ALTER TABLE services ALTER COLUMN auto_deploy_env SET DEFAULT 'development';

-- Note: We intentionally do NOT revert the data changes (UPDATE statement from up migration)
-- Services that were updated to auto_deploy=true should continue working
-- Reverting data would break production deployments
