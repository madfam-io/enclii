package api

import (
	"database/sql"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
)

// Team Management API Handlers
// Provides endpoints for team CRUD, member management, and invitations

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

type CreateTeamRequest struct {
	Name         string  `json:"name" binding:"required,min=2,max=100"`
	Slug         string  `json:"slug" binding:"required,min=2,max=50"`
	Description  *string `json:"description,omitempty"`
	BillingEmail *string `json:"billing_email,omitempty"`
}

type UpdateTeamRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	BillingEmail *string `json:"billing_email,omitempty"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
}

type InviteMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=admin member viewer"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=owner admin member viewer"`
}

type TeamResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	BillingEmail *string   `json:"billing_email,omitempty"`
	MemberCount  int       `json:"member_count"`
	UserRole     string    `json:"user_role,omitempty"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

type TeamMemberResponse struct {
	ID       uuid.UUID `json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Name     *string   `json:"name,omitempty"`
	Role     string    `json:"role"`
	JoinedAt string    `json:"joined_at"`
}

type TeamInvitationResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	ExpiresAt string    `json:"expires_at"`
	InviterID uuid.UUID `json:"inviter_id"`
	CreatedAt string    `json:"created_at"`
}

type InvitationDetailsResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	TeamName    string    `json:"team_name"`
	TeamSlug    string    `json:"team_slug"`
	InviterName *string   `json:"inviter_name,omitempty"`
	ExpiresAt   string    `json:"expires_at"`
	CreatedAt   string    `json:"created_at"`
}

// ============================================================================
// TEAM CRUD HANDLERS
// ============================================================================

// CreateTeam creates a new team with the current user as owner
func (h *Handler) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Validate slug format (lowercase alphanumeric with hyphens)
	slugRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	if !slugRegex.MatchString(req.Slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Slug must be lowercase alphanumeric with hyphens, not starting or ending with hyphen"})
		return
	}

	// Get current user
	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := c.Request.Context()

	// Check if slug already exists
	existing, err := h.repos.Teams.GetBySlug(ctx, req.Slug)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Team slug already exists"})
		return
	}

	// Create team
	team := &db.Team{
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		BillingEmail: req.BillingEmail,
		OwnerID:      &currentUserID,
	}

	if err := h.repos.Teams.Create(ctx, team); err != nil {
		h.logger.Error(ctx, "Failed to create team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	// Add creator as owner
	member := &db.TeamMember{
		TeamID: team.ID,
		UserID: currentUserID,
		Role:   "owner",
	}
	if err := h.repos.TeamMembers.Add(ctx, member); err != nil {
		h.logger.Error(ctx, "Failed to add owner to team", logging.Error("error", err))
		// Clean up the team if member creation fails
		if deleteErr := h.repos.Teams.Delete(ctx, team.ID); deleteErr != nil {
			h.logger.Error(ctx, "Failed to cleanup team after member creation failure",
				logging.String("team_id", team.ID.String()),
				logging.Error("error", deleteErr))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	c.JSON(http.StatusCreated, TeamResponse{
		ID:           team.ID,
		Name:         team.Name,
		Slug:         team.Slug,
		Description:  team.Description,
		BillingEmail: team.BillingEmail,
		MemberCount:  1,
		UserRole:     "owner",
		CreatedAt:    team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    team.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListTeams returns all teams the current user is a member of
func (h *Handler) ListTeams(c *gin.Context) {
	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := c.Request.Context()

	teams, err := h.repos.Teams.ListByUser(ctx, currentUserID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list teams", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	// Build response with member counts and user roles
	responses := make([]TeamResponse, 0, len(teams))
	for _, team := range teams {
		memberCount, _ := h.repos.TeamMembers.CountByTeam(ctx, team.ID)
		userRole, _ := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)

		responses = append(responses, TeamResponse{
			ID:           team.ID,
			Name:         team.Name,
			Slug:         team.Slug,
			Description:  team.Description,
			AvatarURL:    team.AvatarURL,
			BillingEmail: team.BillingEmail,
			MemberCount:  memberCount,
			UserRole:     userRole,
			CreatedAt:    team.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:    team.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"teams": responses})
}

// GetTeam returns a single team by slug
func (h *Handler) GetTeam(c *gin.Context) {
	slug := c.Param("slug")

	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := c.Request.Context()

	team, err := h.repos.Teams.GetBySlug(ctx, slug)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	// Check user is a member
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || userRole == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this team"})
		return
	}

	memberCount, _ := h.repos.TeamMembers.CountByTeam(ctx, team.ID)

	c.JSON(http.StatusOK, TeamResponse{
		ID:           team.ID,
		Name:         team.Name,
		Slug:         team.Slug,
		Description:  team.Description,
		AvatarURL:    team.AvatarURL,
		BillingEmail: team.BillingEmail,
		MemberCount:  memberCount,
		UserRole:     userRole,
		CreatedAt:    team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    team.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// UpdateTeam updates a team's settings
func (h *Handler) UpdateTeam(c *gin.Context) {
	slug := c.Param("slug")

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := c.Request.Context()

	team, err := h.repos.Teams.GetBySlug(ctx, slug)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	// Check user has admin+ role
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can update team settings"})
		return
	}

	// Apply updates
	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = req.Description
	}
	if req.BillingEmail != nil {
		team.BillingEmail = req.BillingEmail
	}
	if req.AvatarURL != nil {
		team.AvatarURL = req.AvatarURL
	}

	if err := h.repos.Teams.Update(ctx, team); err != nil {
		h.logger.Error(ctx, "Failed to update team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	memberCount, _ := h.repos.TeamMembers.CountByTeam(ctx, team.ID)

	c.JSON(http.StatusOK, TeamResponse{
		ID:           team.ID,
		Name:         team.Name,
		Slug:         team.Slug,
		Description:  team.Description,
		AvatarURL:    team.AvatarURL,
		BillingEmail: team.BillingEmail,
		MemberCount:  memberCount,
		UserRole:     userRole,
		CreatedAt:    team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    team.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// DeleteTeam deletes a team (owner only)
func (h *Handler) DeleteTeam(c *gin.Context) {
	slug := c.Param("slug")

	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx := c.Request.Context()

	team, err := h.repos.Teams.GetBySlug(ctx, slug)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get team"})
		return
	}

	// Only owner can delete
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || userRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owner can delete the team"})
		return
	}

	if err := h.repos.Teams.Delete(ctx, team.ID); err != nil {
		h.logger.Error(ctx, "Failed to delete team", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted successfully"})
}
