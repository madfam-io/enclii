-- Migration 013 Down: Revert Team Management Enhancement

-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_update_team_invitations_updated_at ON team_invitations;
DROP FUNCTION IF EXISTS update_team_invitations_updated_at();
DROP FUNCTION IF EXISTS cleanup_expired_invitations();

-- Drop indexes
DROP INDEX IF EXISTS idx_team_invitations_team_id;
DROP INDEX IF EXISTS idx_team_invitations_email;
DROP INDEX IF EXISTS idx_team_invitations_token;
DROP INDEX IF EXISTS idx_team_invitations_status;
DROP INDEX IF EXISTS idx_team_invitations_expires_at;
DROP INDEX IF EXISTS idx_projects_team_id;
DROP INDEX IF EXISTS idx_teams_owner_id;

-- Remove columns from team_members
ALTER TABLE team_members DROP COLUMN IF EXISTS invited_by;
ALTER TABLE team_members DROP COLUMN IF EXISTS accepted_at;

-- Remove team_id from projects
ALTER TABLE projects DROP COLUMN IF EXISTS team_id;

-- Remove columns from teams
ALTER TABLE teams DROP COLUMN IF EXISTS description;
ALTER TABLE teams DROP COLUMN IF EXISTS avatar_url;
ALTER TABLE teams DROP COLUMN IF EXISTS billing_email;
ALTER TABLE teams DROP COLUMN IF EXISTS settings;
ALTER TABLE teams DROP COLUMN IF EXISTS owner_id;

-- Drop team_invitations table
DROP TABLE IF EXISTS team_invitations;
