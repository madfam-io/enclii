-- Rollback Migration 005: Remove OIDC support and role column

-- Drop indexes
DROP INDEX IF EXISTS idx_users_oidc_identity;
DROP INDEX IF EXISTS idx_users_oidc_subject;

-- Drop OIDC columns
ALTER TABLE users DROP COLUMN IF EXISTS oidc_issuer;
ALTER TABLE users DROP COLUMN IF EXISTS oidc_subject;

-- Restore old oidc_sub column (for backward compatibility)
ALTER TABLE users ADD COLUMN oidc_sub VARCHAR(255) UNIQUE;

-- Drop role column
ALTER TABLE users DROP COLUMN IF EXISTS role;
