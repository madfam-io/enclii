-- Migration 009: Rollback Cloudflare tunnel configuration

-- Remove triggers
DROP TRIGGER IF EXISTS update_cloudflare_accounts_updated_at ON cloudflare_accounts;
DROP TRIGGER IF EXISTS update_cloudflare_tunnels_updated_at ON cloudflare_tunnels;

-- Remove columns from custom_domains
ALTER TABLE custom_domains DROP COLUMN IF EXISTS cloudflare_tunnel_id;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS is_platform_domain;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS zero_trust_enabled;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS access_policy_id;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS tls_provider;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS status;
ALTER TABLE custom_domains DROP COLUMN IF EXISTS dns_cname;

-- Drop indexes
DROP INDEX IF EXISTS idx_custom_domains_cloudflare_tunnel_id;
DROP INDEX IF EXISTS idx_custom_domains_status;
DROP INDEX IF EXISTS idx_cloudflare_tunnels_environment_id;
DROP INDEX IF EXISTS idx_cloudflare_tunnels_cloudflare_account_id;
DROP INDEX IF EXISTS idx_cloudflare_accounts_account_id;

-- Drop tables
DROP TABLE IF EXISTS cloudflare_tunnels;
DROP TABLE IF EXISTS cloudflare_accounts;
