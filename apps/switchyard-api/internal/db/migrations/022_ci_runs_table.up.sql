-- Migration 022: Add ci_runs table for GitHub Actions workflow status tracking
-- Purpose: Store workflow run status to enable unified build progress UI

CREATE TABLE IF NOT EXISTS ci_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    commit_sha VARCHAR(64) NOT NULL,

    -- GitHub workflow identification
    workflow_name VARCHAR(255) NOT NULL,
    workflow_id BIGINT NOT NULL,
    run_id BIGINT NOT NULL UNIQUE,
    run_number INTEGER NOT NULL,

    -- Status tracking
    status VARCHAR(32) NOT NULL DEFAULT 'queued',  -- queued, in_progress, completed
    conclusion VARCHAR(32),  -- success, failure, cancelled, skipped, timed_out, action_required

    -- Metadata
    html_url TEXT,
    branch VARCHAR(255),
    event_type VARCHAR(64),  -- push, pull_request, workflow_dispatch, etc.
    actor VARCHAR(255),  -- GitHub username who triggered

    -- Timestamps
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for finding CI runs by service and commit
CREATE INDEX IF NOT EXISTS idx_ci_runs_service_commit
ON ci_runs(service_id, commit_sha);

-- Index for finding latest CI runs by commit
CREATE INDEX IF NOT EXISTS idx_ci_runs_commit_sha
ON ci_runs(commit_sha);

-- Index for finding CI runs by status (for monitoring)
CREATE INDEX IF NOT EXISTS idx_ci_runs_status
ON ci_runs(status) WHERE status != 'completed';

-- Index for finding CI runs by workflow run ID (for webhook updates)
CREATE INDEX IF NOT EXISTS idx_ci_runs_run_id
ON ci_runs(run_id);

COMMENT ON TABLE ci_runs IS 'Stores GitHub Actions workflow run status for unified build progress tracking';
COMMENT ON COLUMN ci_runs.status IS 'Workflow status: queued, in_progress, completed';
COMMENT ON COLUMN ci_runs.conclusion IS 'Final result when completed: success, failure, cancelled, skipped, timed_out, action_required';
