-- Migration: Add health tracking columns to services table
-- Purpose: Store K8s namespace and health/replica data for Cartographer sync

-- Store actual K8s namespace (may differ from project slug)
ALTER TABLE services ADD COLUMN IF NOT EXISTS k8s_namespace VARCHAR(255);

-- Health status: unknown, healthy, unhealthy
ALTER TABLE services ADD COLUMN IF NOT EXISTS health VARCHAR(50) DEFAULT 'unknown' NOT NULL;

-- Service status: unknown, pending, running, failed
ALTER TABLE services ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'unknown' NOT NULL;

-- Replica tracking
ALTER TABLE services ADD COLUMN IF NOT EXISTS desired_replicas INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE services ADD COLUMN IF NOT EXISTS ready_replicas INTEGER DEFAULT 0 NOT NULL;

-- When was health last checked
ALTER TABLE services ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ;

-- Index for health queries
CREATE INDEX IF NOT EXISTS idx_services_health ON services(health);
CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
