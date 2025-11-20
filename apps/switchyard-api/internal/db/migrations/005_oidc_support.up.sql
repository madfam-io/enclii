-- Migration 005: Add OIDC support and role-based access control
-- This allows users to be linked to external OIDC providers (like Plinto)
-- while maintaining backward compatibility with local JWT authentication

-- Add role column for RBAC (admin, developer, viewer)
ALTER TABLE users ADD COLUMN role VARCHAR(50) NOT NULL DEFAULT 'developer';

-- Remove old oidc_sub column if it exists and add new OIDC columns
ALTER TABLE users DROP COLUMN IF EXISTS oidc_sub;
ALTER TABLE users ADD COLUMN oidc_subject TEXT;
ALTER TABLE users ADD COLUMN oidc_issuer TEXT;

-- Create index for fast OIDC lookup
CREATE INDEX idx_users_oidc_subject ON users(oidc_subject);

-- Create composite index for issuer + subject (unique identifier)
CREATE UNIQUE INDEX idx_users_oidc_identity ON users(oidc_issuer, oidc_subject)
WHERE oidc_issuer IS NOT NULL AND oidc_subject IS NOT NULL;

-- Add comments explaining the schema
COMMENT ON COLUMN users.role IS 'User role for RBAC: admin, developer, or viewer';
COMMENT ON COLUMN users.oidc_subject IS 'OIDC subject identifier (sub claim) from external provider';
COMMENT ON COLUMN users.oidc_issuer IS 'OIDC issuer URL (iss claim) from external provider';
