-- Migration 023: Serverless Functions
-- Enables scale-to-zero serverless function deployment (Enclii Functions)
-- Provides Vercel-like serverless experience with KEDA-based autoscaling

-- ============================================================================
-- FUNCTIONS TABLE
-- Stores metadata for serverless functions
-- ============================================================================
CREATE TABLE IF NOT EXISTS functions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    -- Function identification
    name VARCHAR(255) NOT NULL,

    -- Configuration (stored as JSONB for flexibility)
    -- Example: {"runtime": "go", "handler": "main.Handler", "memory": "128Mi", "timeout": 30, ...}
    config JSONB NOT NULL DEFAULT '{}',

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'building', 'deploying', 'ready', 'failed', 'deleting'
    status_message TEXT, -- Detailed status or error message

    -- Kubernetes resources
    k8s_namespace VARCHAR(255), -- Namespace where the function is deployed
    k8s_resource_name VARCHAR(255), -- Name of the K8s Function CRD resource

    -- Image and endpoint
    image_uri TEXT, -- Container image URI after build
    endpoint TEXT, -- Public endpoint URL (e.g., https://hello.fn.enclii.dev)

    -- Runtime metrics
    available_replicas INTEGER DEFAULT 0,
    invocation_count BIGINT DEFAULT 0,
    avg_duration_ms DOUBLE PRECISION DEFAULT 0,
    last_invoked_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    created_by UUID REFERENCES users(id),
    created_by_email VARCHAR(255),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deployed_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT valid_function_status CHECK (status IN ('pending', 'building', 'deploying', 'ready', 'failed', 'deleting')),
    CONSTRAINT unique_function_name_per_project UNIQUE (project_id, name)
);

-- ============================================================================
-- FUNCTION INVOCATIONS TABLE
-- Tracks individual function invocations for analytics and debugging
-- Partitioned by created_at for efficient querying and retention management
-- ============================================================================
CREATE TABLE IF NOT EXISTS function_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_id UUID NOT NULL REFERENCES functions(id) ON DELETE CASCADE,

    -- Invocation metadata
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    duration_ms BIGINT, -- Execution duration in milliseconds
    status_code INTEGER, -- HTTP status code of the response
    cold_start BOOLEAN DEFAULT FALSE, -- Whether this was a cold start invocation
    error_type VARCHAR(255), -- Error type if the invocation failed
    request_id VARCHAR(255), -- Unique request identifier for tracing

    -- Timestamp for partitioning
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Create default partition for function invocations
CREATE TABLE IF NOT EXISTS function_invocations_default PARTITION OF function_invocations DEFAULT;

-- Create monthly partitions for current and next 3 months
DO $$
DECLARE
    start_date DATE := date_trunc('month', CURRENT_DATE);
    partition_name TEXT;
    partition_start DATE;
    partition_end DATE;
BEGIN
    FOR i IN 0..3 LOOP
        partition_start := start_date + (i || ' months')::interval;
        partition_end := start_date + ((i + 1) || ' months')::interval;
        partition_name := 'function_invocations_' || to_char(partition_start, 'YYYY_MM');

        -- Check if partition already exists
        IF NOT EXISTS (
            SELECT 1 FROM pg_class WHERE relname = partition_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE %I PARTITION OF function_invocations FOR VALUES FROM (%L) TO (%L)',
                partition_name,
                partition_start,
                partition_end
            );
        END IF;
    END LOOP;
END;
$$;

-- ============================================================================
-- FUNCTION METRICS TABLE (AGGREGATED)
-- Pre-aggregated metrics for efficient dashboard queries
-- ============================================================================
CREATE TABLE IF NOT EXISTS function_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_id UUID NOT NULL REFERENCES functions(id) ON DELETE CASCADE,

    -- Aggregation period
    period VARCHAR(20) NOT NULL, -- 'hourly', 'daily', 'weekly'
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Aggregated metrics
    total_invocations BIGINT DEFAULT 0,
    success_count BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    cold_start_count BIGINT DEFAULT 0,
    avg_duration_ms DOUBLE PRECISION DEFAULT 0,
    p50_duration_ms DOUBLE PRECISION DEFAULT 0,
    p95_duration_ms DOUBLE PRECISION DEFAULT 0,
    p99_duration_ms DOUBLE PRECISION DEFAULT 0,

    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_function_metric_period UNIQUE (function_id, period, period_start)
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_functions_project_id ON functions(project_id);
CREATE INDEX IF NOT EXISTS idx_functions_status ON functions(status);
CREATE INDEX IF NOT EXISTS idx_functions_created_by ON functions(created_by) WHERE created_by IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_functions_name ON functions(name);
CREATE INDEX IF NOT EXISTS idx_functions_deleted_at ON functions(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_function_invocations_function_id ON function_invocations(function_id);
CREATE INDEX IF NOT EXISTS idx_function_invocations_started_at ON function_invocations(started_at);
CREATE INDEX IF NOT EXISTS idx_function_invocations_cold_start ON function_invocations(cold_start) WHERE cold_start = TRUE;
CREATE INDEX IF NOT EXISTS idx_function_invocations_request_id ON function_invocations(request_id) WHERE request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_function_metrics_function_id ON function_metrics(function_id);
CREATE INDEX IF NOT EXISTS idx_function_metrics_period ON function_metrics(period, period_start);

-- ============================================================================
-- TRIGGER FOR UPDATED_AT
-- ============================================================================
CREATE OR REPLACE FUNCTION update_functions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_functions_updated_at ON functions;
CREATE TRIGGER trigger_update_functions_updated_at
    BEFORE UPDATE ON functions
    FOR EACH ROW
    EXECUTE FUNCTION update_functions_updated_at();

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================
COMMENT ON TABLE functions IS 'Serverless functions with scale-to-zero autoscaling via KEDA';
COMMENT ON COLUMN functions.config IS 'JSON configuration: {runtime, handler, memory, cpu, timeout, minReplicas, maxReplicas, cooldownPeriod, concurrency}';
COMMENT ON COLUMN functions.status IS 'Lifecycle status: pending, building, deploying, ready, failed, deleting';
COMMENT ON COLUMN functions.endpoint IS 'Public endpoint URL (e.g., https://hello.fn.enclii.dev)';
COMMENT ON COLUMN functions.available_replicas IS 'Current number of running pods (0 = scaled to zero)';

COMMENT ON TABLE function_invocations IS 'Individual function invocation records for analytics and debugging';
COMMENT ON COLUMN function_invocations.cold_start IS 'Whether this invocation triggered a scale-up from zero';
COMMENT ON COLUMN function_invocations.request_id IS 'Unique request ID for distributed tracing';

COMMENT ON TABLE function_metrics IS 'Pre-aggregated metrics for efficient dashboard queries';
COMMENT ON COLUMN function_metrics.period IS 'Aggregation period: hourly, daily, weekly';
