-- Migration: Change auto_deploy defaults to enable by default
-- This ensures all services automatically deploy after successful builds

-- Change default auto_deploy to TRUE (was FALSE)
ALTER TABLE services ALTER COLUMN auto_deploy SET DEFAULT TRUE;

-- Change default auto_deploy_env to 'production' (was 'development')
-- Production is a more sensible default for auto-deploy
ALTER TABLE services ALTER COLUMN auto_deploy_env SET DEFAULT 'production';

-- Update existing services that have the old defaults to use new defaults
-- This ensures existing services that were never explicitly configured will now auto-deploy
UPDATE services
SET auto_deploy = TRUE,
    auto_deploy_env = 'production'
WHERE auto_deploy = FALSE
  AND auto_deploy_env = 'development';

-- Log the migration result
DO $$
DECLARE
    updated_count INTEGER;
BEGIN
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    RAISE NOTICE 'Updated % services to enable auto-deploy with production environment', updated_count;
END $$;
