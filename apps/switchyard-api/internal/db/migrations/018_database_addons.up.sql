-- Migration 018: Database Add-ons
-- Enables one-click database provisioning for PostgreSQL, Redis, MySQL
-- Matches Railway's core value proposition for developer experience

-- ============================================================================
-- DATABASE ADDONS TABLE
-- Stores metadata for provisioned database instances
-- ============================================================================
CREATE TABLE IF NOT EXISTS database_addons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id UUID REFERENCES environments(id) ON DELETE SET NULL,

    -- Database type and identification
    type VARCHAR(50) NOT NULL, -- 'postgres', 'redis', 'mysql'
    name VARCHAR(255) NOT NULL,

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'provisioning', 'ready', 'failed', 'deleting', 'deleted'
    status_message TEXT, -- Detailed status or error message

    -- Configuration (stored as JSONB for flexibility)
    -- Example: {"version": "16", "storage_gb": 10, "cpu": "100m", "memory": "256Mi", "ha_enabled": false}
    config JSONB NOT NULL DEFAULT '{}',

    -- Kubernetes resources
    k8s_namespace VARCHAR(255), -- Namespace where the database is deployed
    k8s_resource_name VARCHAR(255), -- Name of the K8s resource (e.g., CloudNativePG Cluster)
    connection_secret VARCHAR(255), -- K8s secret name containing credentials

    -- Connection info (stored encrypted, populated after provisioning)
    host VARCHAR(255),
    port INTEGER,
    database_name VARCHAR(255),
    username VARCHAR(255),
    -- Password is stored in K8s secret, referenced by connection_secret

    -- Resource tracking
    storage_used_bytes BIGINT DEFAULT 0,
    connections_active INTEGER DEFAULT 0,
    last_backup_at TIMESTAMP WITH TIME ZONE,

    -- Audit fields
    created_by UUID REFERENCES users(id),
    created_by_email VARCHAR(255),

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    provisioned_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT valid_addon_type CHECK (type IN ('postgres', 'redis', 'mysql')),
    CONSTRAINT valid_addon_status CHECK (status IN ('pending', 'provisioning', 'ready', 'failed', 'deleting', 'deleted')),
    CONSTRAINT unique_addon_name_per_project UNIQUE (project_id, name)
);

-- ============================================================================
-- DATABASE ADDON SERVICE BINDINGS
-- Links database addons to services for automatic env var injection
-- ============================================================================
CREATE TABLE IF NOT EXISTS database_addon_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    addon_id UUID NOT NULL REFERENCES database_addons(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,

    -- Environment variable configuration
    env_var_name VARCHAR(255) NOT NULL DEFAULT 'DATABASE_URL', -- e.g., 'DATABASE_URL', 'REDIS_URL'

    -- Binding status
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- 'active', 'suspended', 'deleted'

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_binding_per_service UNIQUE (addon_id, service_id),
    CONSTRAINT valid_binding_status CHECK (status IN ('active', 'suspended', 'deleted'))
);

-- ============================================================================
-- DATABASE ADDON BACKUPS
-- Tracks backup history for database addons
-- ============================================================================
CREATE TABLE IF NOT EXISTS database_addon_backups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    addon_id UUID NOT NULL REFERENCES database_addons(id) ON DELETE CASCADE,

    -- Backup info
    backup_type VARCHAR(50) NOT NULL DEFAULT 'scheduled', -- 'scheduled', 'manual', 'pre_delete'
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'in_progress', 'completed', 'failed'
    status_message TEXT,

    -- Storage info
    storage_path VARCHAR(1024), -- S3/R2 path
    size_bytes BIGINT,

    -- Timestamps
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE, -- When the backup will be auto-deleted
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_backup_type CHECK (backup_type IN ('scheduled', 'manual', 'pre_delete')),
    CONSTRAINT valid_backup_status CHECK (status IN ('pending', 'in_progress', 'completed', 'failed'))
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_database_addons_project_id ON database_addons(project_id);
CREATE INDEX IF NOT EXISTS idx_database_addons_environment_id ON database_addons(environment_id) WHERE environment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_database_addons_status ON database_addons(status);
CREATE INDEX IF NOT EXISTS idx_database_addons_type ON database_addons(type);
CREATE INDEX IF NOT EXISTS idx_database_addons_created_by ON database_addons(created_by) WHERE created_by IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_database_addon_bindings_addon_id ON database_addon_bindings(addon_id);
CREATE INDEX IF NOT EXISTS idx_database_addon_bindings_service_id ON database_addon_bindings(service_id);
CREATE INDEX IF NOT EXISTS idx_database_addon_bindings_status ON database_addon_bindings(status) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_database_addon_backups_addon_id ON database_addon_backups(addon_id);
CREATE INDEX IF NOT EXISTS idx_database_addon_backups_status ON database_addon_backups(status);
CREATE INDEX IF NOT EXISTS idx_database_addon_backups_expires_at ON database_addon_backups(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================================
-- TRIGGER FOR UPDATED_AT
-- ============================================================================
CREATE OR REPLACE FUNCTION update_database_addons_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_database_addons_updated_at ON database_addons;
CREATE TRIGGER trigger_update_database_addons_updated_at
    BEFORE UPDATE ON database_addons
    FOR EACH ROW
    EXECUTE FUNCTION update_database_addons_updated_at();

DROP TRIGGER IF EXISTS trigger_update_database_addon_bindings_updated_at ON database_addon_bindings;
CREATE TRIGGER trigger_update_database_addon_bindings_updated_at
    BEFORE UPDATE ON database_addon_bindings
    FOR EACH ROW
    EXECUTE FUNCTION update_database_addons_updated_at();

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================
COMMENT ON TABLE database_addons IS 'Database add-ons for one-click PostgreSQL, Redis, MySQL provisioning';
COMMENT ON COLUMN database_addons.type IS 'Database type: postgres, redis, mysql';
COMMENT ON COLUMN database_addons.config IS 'JSON configuration: {version, storage_gb, cpu, memory, ha_enabled}';
COMMENT ON COLUMN database_addons.connection_secret IS 'K8s Secret name containing database credentials';
COMMENT ON COLUMN database_addons.k8s_resource_name IS 'Name of the K8s resource (e.g., CloudNativePG Cluster name)';

COMMENT ON TABLE database_addon_bindings IS 'Links database addons to services for automatic env var injection';
COMMENT ON COLUMN database_addon_bindings.env_var_name IS 'Environment variable name to inject (e.g., DATABASE_URL, REDIS_URL)';

COMMENT ON TABLE database_addon_backups IS 'Backup history for database addons';
COMMENT ON COLUMN database_addon_backups.storage_path IS 'S3/R2 path where backup is stored';
