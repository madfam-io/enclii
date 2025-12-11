-- Add auto-deploy configuration to services
-- This enables automatic deployment after successful builds triggered by webhooks

ALTER TABLE services
    ADD COLUMN IF NOT EXISTS auto_deploy BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS auto_deploy_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    ADD COLUMN IF NOT EXISTS auto_deploy_env VARCHAR(255) NOT NULL DEFAULT 'development';

-- Add index for services with auto-deploy enabled (for webhook lookups)
CREATE INDEX IF NOT EXISTS idx_services_auto_deploy ON services(auto_deploy) WHERE auto_deploy = TRUE;

COMMENT ON COLUMN services.auto_deploy IS 'Enable automatic deployment after successful webhook-triggered builds';
COMMENT ON COLUMN services.auto_deploy_branch IS 'Branch to auto-deploy from (e.g., main, master)';
COMMENT ON COLUMN services.auto_deploy_env IS 'Target environment for auto-deployments (e.g., development, staging)';
