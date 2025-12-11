-- 007_github_integration.up.sql
-- GitHub App installation tracking and repository connections

-- GitHub App installations (per-user)
-- This stores the installation ID when a user installs our GitHub App
CREATE TABLE IF NOT EXISTS github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL UNIQUE,  -- GitHub installation ID
    account_login VARCHAR(255) NOT NULL,      -- GitHub username or org name
    account_type VARCHAR(50) NOT NULL,        -- 'User' or 'Organization'
    access_tokens_url TEXT,                   -- URL to get installation access tokens
    repositories_url TEXT,                    -- URL to list accessible repos
    permissions JSONB DEFAULT '{}',           -- Granted permissions
    events JSONB DEFAULT '[]',                -- Subscribed events
    suspended_at TIMESTAMP WITH TIME ZONE,    -- If installation is suspended
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for fast lookup by user
CREATE INDEX IF NOT EXISTS idx_github_installations_user_id ON github_installations(user_id);

-- Index for lookup by installation ID (GitHub webhook payloads)
CREATE INDEX IF NOT EXISTS idx_github_installations_installation_id ON github_installations(installation_id);

-- GitHub repository connections (which repos are linked to which services)
CREATE TABLE IF NOT EXISTS github_repositories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id UUID NOT NULL REFERENCES github_installations(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,  -- Linked service (optional)
    repo_id BIGINT NOT NULL,                  -- GitHub repository ID
    repo_full_name VARCHAR(255) NOT NULL,     -- e.g., 'madfam-io/enclii'
    repo_name VARCHAR(255) NOT NULL,          -- e.g., 'enclii'
    default_branch VARCHAR(255) DEFAULT 'main',
    private BOOLEAN NOT NULL DEFAULT false,
    webhook_id BIGINT,                        -- GitHub webhook ID if auto-registered
    webhook_secret VARCHAR(255),              -- Webhook secret for this repo
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(installation_id, repo_id)
);

-- Index for finding repos by service
CREATE INDEX IF NOT EXISTS idx_github_repositories_service_id ON github_repositories(service_id);

-- Index for finding repos by full name (for webhook matching)
CREATE INDEX IF NOT EXISTS idx_github_repositories_repo_full_name ON github_repositories(repo_full_name);

-- Add github_repo_id to services for quick lookup
ALTER TABLE services ADD COLUMN IF NOT EXISTS github_repo_id UUID REFERENCES github_repositories(id) ON DELETE SET NULL;

-- Update trigger for github_installations
CREATE OR REPLACE FUNCTION update_github_installations_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_github_installations_updated_at
    BEFORE UPDATE ON github_installations
    FOR EACH ROW
    EXECUTE FUNCTION update_github_installations_updated_at();

-- Update trigger for github_repositories
CREATE OR REPLACE FUNCTION update_github_repositories_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_github_repositories_updated_at
    BEFORE UPDATE ON github_repositories
    FOR EACH ROW
    EXECUTE FUNCTION update_github_repositories_updated_at();
