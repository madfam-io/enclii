-- Migration: 010_deployment_groups
-- Add deployment groups for coordinated multi-service deployments
-- This enables atomic deployment of multiple services with dependency ordering

-- Deployment groups for atomic multi-service deployments
CREATE TABLE deployment_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- Status values: 'pending', 'in_progress', 'deploying', 'succeeded', 'failed', 'rolled_back'
    strategy VARCHAR(50) NOT NULL DEFAULT 'dependency_ordered',
    -- Strategy values: 'parallel', 'dependency_ordered', 'sequential'
    triggered_by VARCHAR(255), -- 'webhook', 'manual', 'promotion', 'rollback'
    git_sha VARCHAR(40),
    pr_url TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Link deployments to their group
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS group_id UUID REFERENCES deployment_groups(id) ON DELETE SET NULL;
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS deploy_order INTEGER DEFAULT 0;

-- Service dependencies within a project
-- Enables topological sorting for deployment order
CREATE TABLE service_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    depends_on_service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    dependency_type VARCHAR(50) NOT NULL DEFAULT 'runtime',
    -- Dependency types: 'runtime' (must run before), 'build' (build dependency), 'data' (data migration)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_dependency UNIQUE(service_id, depends_on_service_id),
    CONSTRAINT no_self_dependency CHECK(service_id != depends_on_service_id)
);

-- Indexes for efficient queries
CREATE INDEX idx_deployment_groups_project ON deployment_groups(project_id);
CREATE INDEX idx_deployment_groups_environment ON deployment_groups(environment_id);
CREATE INDEX idx_deployment_groups_status ON deployment_groups(status);
CREATE INDEX idx_deployment_groups_created_at ON deployment_groups(created_at DESC);
CREATE INDEX idx_deployments_group ON deployments(group_id) WHERE group_id IS NOT NULL;
CREATE INDEX idx_deployments_deploy_order ON deployments(group_id, deploy_order) WHERE group_id IS NOT NULL;
CREATE INDEX idx_service_dependencies_service ON service_dependencies(service_id);
CREATE INDEX idx_service_dependencies_depends_on ON service_dependencies(depends_on_service_id);

-- Triggers for updated_at
CREATE TRIGGER update_deployment_groups_updated_at
    BEFORE UPDATE ON deployment_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE deployment_groups IS 'Coordinates atomic multi-service deployments with dependency ordering';
COMMENT ON COLUMN deployment_groups.strategy IS 'Deployment strategy: parallel (all at once), dependency_ordered (topological sort), sequential (one by one)';
COMMENT ON COLUMN deployment_groups.triggered_by IS 'What initiated this deployment: webhook, manual, promotion, rollback';
COMMENT ON TABLE service_dependencies IS 'Defines inter-service dependencies for deployment ordering via topological sort';
COMMENT ON COLUMN service_dependencies.dependency_type IS 'Type of dependency: runtime (must run before), build (build-time), data (migration)';
COMMENT ON COLUMN deployments.deploy_order IS 'Order within deployment group (0 = first layer, 1 = second layer, etc.)';
