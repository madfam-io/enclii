-- Rollback Roundhouse build jobs

DROP VIEW IF EXISTS build_metrics;
DROP TRIGGER IF EXISTS build_jobs_updated_at ON build_jobs;
DROP FUNCTION IF EXISTS update_build_jobs_updated_at();
DROP TABLE IF EXISTS build_jobs;
