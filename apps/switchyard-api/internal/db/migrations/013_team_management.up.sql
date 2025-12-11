-- Migration 013: Team Management Enhancement
-- Adds team invitations, enhanced settings, and team-project relationships
-- for full Railway/Vercel-style team management

-- ============================================================================
-- TEAM INVITATIONS
-- Allows inviting users by email before they have an account
-- ============================================================================
CREATE TABLE IF NOT EXISTS team_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- 'admin', 'member', 'viewer'
    invited_by UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL UNIQUE, -- secure token for accepting invitation
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'accepted', 'declined', 'expired'
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    accepted_at TIMESTAMP WITH TIME ZONE,
    declined_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, email, status) -- Can only have one pending invitation per email per team
);

-- ============================================================================
-- ENHANCED TEAM SETTINGS
-- Add additional team configuration options
-- ============================================================================
ALTER TABLE teams ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE teams ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(500);
ALTER TABLE teams ADD COLUMN IF NOT EXISTS billing_email VARCHAR(255);
ALTER TABLE teams ADD COLUMN IF NOT EXISTS settings JSONB DEFAULT '{}';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id);

-- ============================================================================
-- TEAM-PROJECT RELATIONSHIPS
-- Link projects to teams for organization
-- ============================================================================
ALTER TABLE projects ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

-- ============================================================================
-- ENHANCED TEAM MEMBER ROLES
-- Add more granular role options and metadata
-- ============================================================================
-- Add invited_by column to track who invited the member
ALTER TABLE team_members ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES users(id);
-- Add accepted_at to track when invitation was accepted
ALTER TABLE team_members ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMP WITH TIME ZONE;

-- ============================================================================
-- INDEXES FOR PERFORMANCE
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_team_invitations_team_id ON team_invitations(team_id);
CREATE INDEX IF NOT EXISTS idx_team_invitations_email ON team_invitations(email);
CREATE INDEX IF NOT EXISTS idx_team_invitations_token ON team_invitations(token);
CREATE INDEX IF NOT EXISTS idx_team_invitations_status ON team_invitations(status);
CREATE INDEX IF NOT EXISTS idx_team_invitations_expires_at ON team_invitations(expires_at);
CREATE INDEX IF NOT EXISTS idx_projects_team_id ON projects(team_id);
CREATE INDEX IF NOT EXISTS idx_teams_owner_id ON teams(owner_id);

-- ============================================================================
-- FUNCTIONS AND TRIGGERS
-- ============================================================================

-- Function to clean up expired invitations (can be called periodically)
CREATE OR REPLACE FUNCTION cleanup_expired_invitations()
RETURNS void AS $$
BEGIN
    UPDATE team_invitations
    SET status = 'expired', updated_at = NOW()
    WHERE status = 'pending' AND expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Trigger to update updated_at on team_invitations
CREATE OR REPLACE FUNCTION update_team_invitations_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_update_team_invitations_updated_at ON team_invitations;
CREATE TRIGGER trigger_update_team_invitations_updated_at
    BEFORE UPDATE ON team_invitations
    FOR EACH ROW
    EXECUTE FUNCTION update_team_invitations_updated_at();

-- ============================================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================================
COMMENT ON TABLE team_invitations IS 'Pending and historical team invitations';
COMMENT ON COLUMN team_invitations.token IS 'Secure random token for accepting invitation via link';
COMMENT ON COLUMN team_invitations.status IS 'Invitation status: pending, accepted, declined, expired';
COMMENT ON COLUMN team_invitations.role IS 'Role to assign when invitation is accepted: admin, member, viewer';
COMMENT ON COLUMN teams.owner_id IS 'Primary owner of the team with full administrative rights';
COMMENT ON COLUMN teams.settings IS 'JSON configuration for team-specific settings';
COMMENT ON COLUMN projects.team_id IS 'Team that owns this project (NULL for personal projects)';
