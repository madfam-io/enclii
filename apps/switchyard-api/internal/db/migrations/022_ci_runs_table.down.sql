-- Rollback migration 022: Remove ci_runs table

DROP INDEX IF EXISTS idx_ci_runs_run_id;
DROP INDEX IF EXISTS idx_ci_runs_status;
DROP INDEX IF EXISTS idx_ci_runs_commit_sha;
DROP INDEX IF EXISTS idx_ci_runs_service_commit;

DROP TABLE IF EXISTS ci_runs;
