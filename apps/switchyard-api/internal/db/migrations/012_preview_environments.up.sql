-- Migration: 012_preview_environments
-- Add preview environments for automatic PR-based deployments
-- This is the killer feature for Vercel/Railway parity

-- Preview environments for PR-based ephemeral deployments
CREATE TABLE preview_environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    
    -- PR Information
    pr_number INTEGER NOT NULL,
    pr_title TEXT,
    pr_url TEXT,
    pr_author VARCHAR(255),
    pr_branch VARCHAR(255) NOT NULL,
    pr_base_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    commit_sha VARCHAR(40) NOT NULL,
    
    -- Preview URL (e.g., pr-123.preview.enclii.app)
    preview_subdomain VARCHAR(255) NOT NULL,
    preview_url TEXT NOT NULL,
    
    -- Status: pending, building, deploying, active, sleeping, failed, closed
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    status_message TEXT,
    
    -- Auto-sleep configuration (in minutes, 0 = never sleep)
    auto_sleep_after INTEGER NOT NULL DEFAULT 30,
    last_accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    sleeping_since TIMESTAMP WITH TIME ZONE,
    
    -- Resource tracking
    deployment_id UUID REFERENCES deployments(id) ON DELETE SET NULL,
    build_logs_url TEXT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,
    
    -- Ensure unique preview per PR per service
    CONSTRAINT unique_preview_per_pr UNIQUE(service_id, pr_number)
);

-- Add preview_environment_id to deployments for linking
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS preview_environment_id UUID REFERENCES preview_environments(id) ON DELETE SET NULL;

-- Preview environment comments for collaboration (like Vercel comments)
CREATE TABLE preview_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    preview_id UUID NOT NULL REFERENCES preview_environments(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    user_name VARCHAR(255),
    
    -- Comment content
    content TEXT NOT NULL,
    
    -- Optional: attach to specific URL path or coordinate
    path TEXT,
    x_position INTEGER,
    y_position INTEGER,
    
    -- Status: active, resolved, deleted
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by UUID,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Preview access logs for analytics and auto-sleep detection
CREATE TABLE preview_access_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    preview_id UUID NOT NULL REFERENCES preview_environments(id) ON DELETE CASCADE,
    accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Request metadata
    path TEXT,
    user_agent TEXT,
    ip_address INET,
    
    -- Optional: authenticated user
    user_id UUID,
    
    -- Response metadata
    status_code INTEGER,
    response_time_ms INTEGER
);

-- Indexes for efficient queries
CREATE INDEX idx_preview_environments_project ON preview_environments(project_id);
CREATE INDEX idx_preview_environments_service ON preview_environments(service_id);
CREATE INDEX idx_preview_environments_pr ON preview_environments(pr_number);
CREATE INDEX idx_preview_environments_status ON preview_environments(status);
CREATE INDEX idx_preview_environments_subdomain ON preview_environments(preview_subdomain);
CREATE INDEX idx_preview_environments_created_at ON preview_environments(created_at DESC);
CREATE INDEX idx_preview_environments_active ON preview_environments(status) WHERE status IN ('active', 'sleeping');

CREATE INDEX idx_preview_comments_preview ON preview_comments(preview_id);
CREATE INDEX idx_preview_comments_user ON preview_comments(user_id);
CREATE INDEX idx_preview_comments_status ON preview_comments(status) WHERE status = 'active';

CREATE INDEX idx_preview_access_logs_preview ON preview_access_logs(preview_id);
CREATE INDEX idx_preview_access_logs_accessed_at ON preview_access_logs(accessed_at DESC);

CREATE INDEX idx_deployments_preview ON deployments(preview_environment_id) WHERE preview_environment_id IS NOT NULL;

-- Triggers for updated_at
CREATE TRIGGER update_preview_environments_updated_at
    BEFORE UPDATE ON preview_environments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_preview_comments_updated_at
    BEFORE UPDATE ON preview_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE preview_environments IS 'Ephemeral environments created automatically for pull requests, similar to Vercel/Railway preview deployments';
COMMENT ON COLUMN preview_environments.preview_subdomain IS 'Unique subdomain like pr-123-myservice for routing';
COMMENT ON COLUMN preview_environments.auto_sleep_after IS 'Minutes of inactivity before auto-sleeping (0 = never)';
COMMENT ON COLUMN preview_environments.sleeping_since IS 'When the preview went to sleep (null if active)';
COMMENT ON TABLE preview_comments IS 'Collaborative comments on preview deployments like Vercel comment feature';
COMMENT ON COLUMN preview_comments.x_position IS 'Optional X coordinate for visual feedback on specific elements';
COMMENT ON TABLE preview_access_logs IS 'Access logs for preview environments to track usage and enable auto-sleep';
