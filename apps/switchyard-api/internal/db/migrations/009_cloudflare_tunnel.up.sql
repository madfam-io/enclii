-- Migration 009: Add Cloudflare tunnel configuration tables
-- This adds support for managing Cloudflare tunnels and Zero Trust protection

-- Platform-level Cloudflare account configuration (admin only)
CREATE TABLE IF NOT EXISTS cloudflare_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    account_id VARCHAR(100) NOT NULL UNIQUE,
    api_token_encrypted TEXT NOT NULL,
    zone_id VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cloudflare_accounts_account_id ON cloudflare_accounts(account_id);

-- Environment-scoped Cloudflare tunnels
CREATE TABLE IF NOT EXISTS cloudflare_tunnels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cloudflare_account_id UUID NOT NULL REFERENCES cloudflare_accounts(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    tunnel_id VARCHAR(100) NOT NULL,
    tunnel_name VARCHAR(100) NOT NULL,
    tunnel_token_encrypted TEXT NOT NULL,
    cname VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_health_check TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(environment_id)
);

CREATE INDEX idx_cloudflare_tunnels_environment_id ON cloudflare_tunnels(environment_id);
CREATE INDEX idx_cloudflare_tunnels_cloudflare_account_id ON cloudflare_tunnels(cloudflare_account_id);

-- Extend custom_domains table with Cloudflare-specific fields
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS cloudflare_tunnel_id UUID REFERENCES cloudflare_tunnels(id);
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS is_platform_domain BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS zero_trust_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS access_policy_id VARCHAR(100);
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS tls_provider VARCHAR(50) DEFAULT 'cert-manager';
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'pending';
ALTER TABLE custom_domains ADD COLUMN IF NOT EXISTS dns_cname VARCHAR(255);

CREATE INDEX idx_custom_domains_cloudflare_tunnel_id ON custom_domains(cloudflare_tunnel_id);
CREATE INDEX idx_custom_domains_status ON custom_domains(status);

-- Add trigger for updated_at on new tables
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_cloudflare_accounts_updated_at ON cloudflare_accounts;
CREATE TRIGGER update_cloudflare_accounts_updated_at
    BEFORE UPDATE ON cloudflare_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_cloudflare_tunnels_updated_at ON cloudflare_tunnels;
CREATE TRIGGER update_cloudflare_tunnels_updated_at
    BEFORE UPDATE ON cloudflare_tunnels
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
