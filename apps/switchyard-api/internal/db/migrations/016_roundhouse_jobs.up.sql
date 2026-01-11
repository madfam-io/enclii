-- Roundhouse build jobs table for historical tracking and audit
-- The actual queue lives in Redis, this is for persistence and querying

CREATE TABLE build_jobs (
    id UUID PRIMARY KEY,
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    git_sha VARCHAR(255) NOT NULL,
    git_branch VARCHAR(255),
    build_config JSONB NOT NULL DEFAULT '{}',

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'queued',
    worker_id VARCHAR(255),
    error_message TEXT,

    -- Results
    image_uri VARCHAR(500),
    image_digest VARCHAR(255),
    image_size_bytes BIGINT,
    sbom TEXT,
    sbom_format VARCHAR(50),
    image_signature TEXT,

    -- Timing
    queued_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_seconds DECIMAL(10, 2),

    -- Metadata
    callback_url VARCHAR(500),
    callback_status VARCHAR(50),
    priority INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    parent_job_id UUID REFERENCES build_jobs(id),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_build_jobs_release ON build_jobs(release_id);
CREATE INDEX idx_build_jobs_service ON build_jobs(service_id);
CREATE INDEX idx_build_jobs_project ON build_jobs(project_id);
CREATE INDEX idx_build_jobs_status ON build_jobs(status);
CREATE INDEX idx_build_jobs_queued_at ON build_jobs(queued_at DESC);
CREATE INDEX idx_build_jobs_worker ON build_jobs(worker_id) WHERE worker_id IS NOT NULL;

-- Build job status constraint
ALTER TABLE build_jobs ADD CONSTRAINT build_jobs_status_check
    CHECK (status IN ('queued', 'building', 'completed', 'failed', 'cancelled'));

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_build_jobs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER build_jobs_updated_at
    BEFORE UPDATE ON build_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_build_jobs_updated_at();

-- Build metrics view for dashboard
CREATE VIEW build_metrics AS
SELECT
    project_id,
    DATE_TRUNC('day', queued_at) AS build_date,
    COUNT(*) AS total_builds,
    COUNT(*) FILTER (WHERE status = 'completed') AS successful_builds,
    COUNT(*) FILTER (WHERE status = 'failed') AS failed_builds,
    AVG(duration_seconds) FILTER (WHERE status = 'completed') AS avg_duration_seconds,
    SUM(image_size_bytes) FILTER (WHERE status = 'completed') AS total_image_bytes
FROM build_jobs
GROUP BY project_id, DATE_TRUNC('day', queued_at);

COMMENT ON TABLE build_jobs IS 'Historical record of all build jobs processed by Roundhouse';
COMMENT ON VIEW build_metrics IS 'Aggregated build metrics per project per day';
