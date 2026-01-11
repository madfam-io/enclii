-- Migration 015: API Tokens
-- Enables programmatic API access for CLI and CI/CD integrations

-- ============================================================================
-- API TOKENS TABLE
-- Stores hashed API tokens for user authentication
-- ============================================================================
CREATE TABLE IF NOT EXISTS api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    -- Token prefix for display (first 8 chars of unhashed token, e.g., "enclii_abc12345...")
    prefix VARCHAR(20) NOT NULL,
    -- SHA-256 hash of the actual token (we never store the raw token)
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    -- Scopes define what the token can do (empty = full access)
    scopes TEXT[] DEFAULT '{}',
    -- Optional expiration (NULL = never expires)
    expires_at TIMESTAMP WITH TIME ZONE,
    -- Track usage
    last_used_at TIMESTAMP WITH TIME ZONE,
    last_used_ip VARCHAR(45),
    -- Status
    revoked BOOLEAN NOT NULL DEFAULT false,
    revoked_at TIMESTAMP WITH TIME ZONE,
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(prefix);
CREATE INDEX IF NOT EXISTS idx_api_tokens_revoked ON api_tokens(revoked) WHERE revoked = false;

-- ============================================================================
-- TRIGGER FOR UPDATED_AT
-- ============================================================================
CREATE OR REPLACE FUNCTION update_api_tokens_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_api_tokens_updated_at ON api_tokens;
CREATE TRIGGER trigger_update_api_tokens_updated_at
    BEFORE UPDATE ON api_tokens
    FOR EACH ROW
    EXECUTE FUNCTION update_api_tokens_updated_at();

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================
COMMENT ON TABLE api_tokens IS 'API tokens for programmatic access (CLI, CI/CD)';
COMMENT ON COLUMN api_tokens.prefix IS 'First 8 chars of token for identification (e.g., "enclii_ab")';
COMMENT ON COLUMN api_tokens.token_hash IS 'SHA-256 hash of the full token (raw token never stored)';
COMMENT ON COLUMN api_tokens.scopes IS 'Array of permission scopes (empty = full access)';
COMMENT ON COLUMN api_tokens.expires_at IS 'Optional expiration timestamp (NULL = never expires)';
