-- Rollback: Remove health tracking columns from services table

DROP INDEX IF EXISTS idx_services_status;
DROP INDEX IF EXISTS idx_services_health;
ALTER TABLE services DROP COLUMN IF EXISTS last_health_check;
ALTER TABLE services DROP COLUMN IF EXISTS ready_replicas;
ALTER TABLE services DROP COLUMN IF EXISTS desired_replicas;
ALTER TABLE services DROP COLUMN IF EXISTS status;
ALTER TABLE services DROP COLUMN IF EXISTS health;
ALTER TABLE services DROP COLUMN IF EXISTS k8s_namespace;
