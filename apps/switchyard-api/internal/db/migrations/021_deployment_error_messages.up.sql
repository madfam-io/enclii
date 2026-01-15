-- Migration 021: Add error_message columns to deployments and releases tables
-- Purpose: Store error information for failed builds and deployments to aid debugging

-- Add error_message to deployments table
-- This stores the reconciliation error when a deployment fails
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS error_message TEXT;

-- Add error_message to releases table
-- This stores the build error when a release fails to build
ALTER TABLE releases ADD COLUMN IF NOT EXISTS error_message TEXT;

-- Create index for quickly finding failed deployments with errors
CREATE INDEX IF NOT EXISTS idx_deployments_status_error
ON deployments(status) WHERE status = 'failed';

-- Create index for quickly finding failed releases with errors
CREATE INDEX IF NOT EXISTS idx_releases_status_error
ON releases(status) WHERE status = 'failed';

-- Add comments for documentation
COMMENT ON COLUMN deployments.error_message IS 'Error message from reconciliation failure, NULL if deployment succeeded';
COMMENT ON COLUMN releases.error_message IS 'Error message from build failure, NULL if build succeeded';
