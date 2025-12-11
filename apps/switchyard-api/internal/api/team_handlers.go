package api

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
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
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Name      *string   `json:"name,omitempty"`
	Role      string    `json:"role"`
	JoinedAt  string    `json:"joined_at"`
}

type TeamInvitationResponse struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	ExpiresAt  string    `json:"expires_at"`
	InviterID  uuid.UUID `json:"inviter_id"`
	CreatedAt  string    `json:"created_at"`
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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
		_ = h.repos.Teams.Delete(ctx, team.ID)
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
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

// ============================================================================
// MEMBER MANAGEMENT HANDLERS
// ============================================================================

// ListTeamMembers returns all members of a team
func (h *Handler) ListTeamMembers(c *gin.Context) {
	slug := c.Param("slug")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	members, err := h.repos.TeamMembers.ListByTeam(ctx, team.ID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list team members", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list team members"})
		return
	}

	responses := make([]TeamMemberResponse, 0, len(members))
	for _, m := range members {
		responses = append(responses, TeamMemberResponse{
			ID:       m.ID,
			UserID:   m.UserID,
			Email:    m.UserEmail,
			Name:     m.UserName,
			Role:     m.Role,
			JoinedAt: m.JoinedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"members": responses})
}

// UpdateMemberRole changes a member's role in the team
func (h *Handler) UpdateMemberRole(c *gin.Context) {
	slug := c.Param("slug")
	memberIDStr := c.Param("member_id")

	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID"})
		return
	}

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	// Check current user has admin+ role
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can change member roles"})
		return
	}

	// Prevent changing owner role unless you're the owner
	if req.Role == "owner" && userRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owner can transfer ownership"})
		return
	}

	// Prevent demoting yourself if you're the only owner
	if memberID == currentUserID && req.Role != "owner" {
		members, _ := h.repos.TeamMembers.ListByTeam(ctx, team.ID)
		ownerCount := 0
		for _, m := range members {
			if m.Role == "owner" {
				ownerCount++
			}
		}
		if ownerCount == 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change role of the only owner. Transfer ownership first."})
			return
		}
	}

	if err := h.repos.TeamMembers.UpdateRole(ctx, team.ID, memberID, req.Role); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	} else if err != nil {
		h.logger.Error(ctx, "Failed to update member role", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update member role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member role updated successfully"})
}

// RemoveMember removes a member from the team
func (h *Handler) RemoveTeamMember(c *gin.Context) {
	slug := c.Param("slug")
	memberIDStr := c.Param("member_id")

	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	// Check current user has admin+ role (or is removing themselves)
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (memberID != currentUserID && userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can remove members"})
		return
	}

	// Prevent owner from leaving if they're the only owner
	if memberID == currentUserID {
		members, _ := h.repos.TeamMembers.ListByTeam(ctx, team.ID)
		ownerCount := 0
		for _, m := range members {
			if m.Role == "owner" {
				ownerCount++
			}
		}
		if ownerCount == 1 && userRole == "owner" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot leave team as the only owner. Transfer ownership first."})
			return
		}
	}

	if err := h.repos.TeamMembers.Remove(ctx, team.ID, memberID); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	} else if err != nil {
		h.logger.Error(ctx, "Failed to remove member", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// ============================================================================
// INVITATION HANDLERS
// ============================================================================

// InviteMember creates an invitation to join the team
func (h *Handler) InviteTeamMember(c *gin.Context) {
	slug := c.Param("slug")

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	// Check current user has admin+ role
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can invite members"})
		return
	}

	// Check if user is already a member (look up by email)
	existingUser, _ := h.repos.Users.GetByEmail(ctx, req.Email)
	if existingUser != nil {
		existingRole, _ := h.repos.TeamMembers.GetUserRole(ctx, team.ID, existingUser.ID)
		if existingRole != "" {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this team"})
			return
		}
	}

	// Check if pending invitation already exists
	hasPending, _ := h.repos.TeamInvitations.HasPendingInvitation(ctx, team.ID, req.Email)
	if hasPending {
		c.JSON(http.StatusConflict, gin.H{"error": "A pending invitation already exists for this email"})
		return
	}

	// Create invitation
	invitation := &db.TeamInvitation{
		TeamID:    team.ID,
		Email:     req.Email,
		Role:      req.Role,
		InvitedBy: currentUserID,
	}

	if err := h.repos.TeamInvitations.Create(ctx, invitation); err != nil {
		h.logger.Error(ctx, "Failed to create invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	// TODO: Send invitation email with invitation.Token

	c.JSON(http.StatusCreated, TeamInvitationResponse{
		ID:        invitation.ID,
		Email:     invitation.Email,
		Role:      invitation.Role,
		Status:    invitation.Status,
		ExpiresAt: invitation.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		InviterID: invitation.InvitedBy,
		CreatedAt: invitation.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// ListTeamInvitations returns pending invitations for a team
func (h *Handler) ListTeamInvitations(c *gin.Context) {
	slug := c.Param("slug")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	// Check current user has admin+ role
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can view invitations"})
		return
	}

	invitations, err := h.repos.TeamInvitations.ListPendingByTeam(ctx, team.ID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list invitations", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list invitations"})
		return
	}

	responses := make([]TeamInvitationResponse, 0, len(invitations))
	for _, inv := range invitations {
		responses = append(responses, TeamInvitationResponse{
			ID:        inv.ID,
			Email:     inv.Email,
			Role:      inv.Role,
			Status:    inv.Status,
			ExpiresAt: inv.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			InviterID: inv.InvitedBy,
			CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"invitations": responses})
}

// CancelInvitation cancels a pending invitation
func (h *Handler) CancelTeamInvitation(c *gin.Context) {
	slug := c.Param("slug")
	invitationIDStr := c.Param("invitation_id")

	invitationID, err := uuid.Parse(invitationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

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

	// Check current user has admin+ role
	userRole, err := h.repos.TeamMembers.GetUserRole(ctx, team.ID, currentUserID)
	if err != nil || (userRole != "owner" && userRole != "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only team owners and admins can cancel invitations"})
		return
	}

	// Verify invitation belongs to this team
	invitation, err := h.repos.TeamInvitations.GetByID(ctx, invitationID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invitation"})
		return
	}

	if invitation.TeamID != team.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if err := h.repos.TeamInvitations.Delete(ctx, invitationID); err != nil {
		h.logger.Error(ctx, "Failed to delete invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation cancelled successfully"})
}

// GetInvitationByToken retrieves invitation details for accepting
func (h *Handler) GetInvitationByToken(c *gin.Context) {
	token := c.Param("token")

	ctx := c.Request.Context()

	invitation, err := h.repos.TeamInvitations.GetByTokenWithDetails(ctx, token)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found or expired"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invitation"})
		return
	}

	if invitation.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation is no longer pending"})
		return
	}

	c.JSON(http.StatusOK, InvitationDetailsResponse{
		ID:          invitation.ID,
		Email:       invitation.Email,
		Role:        invitation.Role,
		Status:      invitation.Status,
		TeamName:    invitation.TeamName,
		TeamSlug:    invitation.TeamSlug,
		InviterName: invitation.InviterName,
		ExpiresAt:   invitation.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		CreatedAt:   invitation.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// AcceptInvitation accepts a team invitation
func (h *Handler) AcceptInvitation(c *gin.Context) {
	token := c.Param("token")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

	ctx := c.Request.Context()

	// Get current user email
	user, err := h.repos.Users.GetByID(ctx, currentUserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	invitation, err := h.repos.TeamInvitations.GetByToken(ctx, token)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found or expired"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invitation"})
		return
	}

	if invitation.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation is no longer pending"})
		return
	}

	// Verify email matches
	if strings.ToLower(user.Email) != strings.ToLower(invitation.Email) {
		c.JSON(http.StatusForbidden, gin.H{"error": "This invitation was sent to a different email address"})
		return
	}

	// Add user to team
	member := &db.TeamMember{
		TeamID:    invitation.TeamID,
		UserID:    currentUserID,
		Role:      invitation.Role,
		InvitedBy: &invitation.InvitedBy,
	}
	if err := h.repos.TeamMembers.Add(ctx, member); err != nil {
		h.logger.Error(ctx, "Failed to add member", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join team"})
		return
	}

	// Mark invitation as accepted
	if err := h.repos.TeamInvitations.Accept(ctx, invitation.ID); err != nil {
		h.logger.Error(ctx, "Failed to accept invitation", logging.Error("error", err))
		// Member was already added, so continue
	}

	// Get team for response
	team, _ := h.repos.Teams.GetByID(ctx, invitation.TeamID)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Successfully joined team",
		"team_slug": team.Slug,
		"team_name": team.Name,
	})
}

// DeclineInvitation declines a team invitation
func (h *Handler) DeclineInvitation(c *gin.Context) {
	token := c.Param("token")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

	ctx := c.Request.Context()

	// Get current user email
	user, err := h.repos.Users.GetByID(ctx, currentUserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	invitation, err := h.repos.TeamInvitations.GetByToken(ctx, token)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found or expired"})
		return
	}
	if err != nil {
		h.logger.Error(ctx, "Failed to get invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invitation"})
		return
	}

	if invitation.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation is no longer pending"})
		return
	}

	// Verify email matches
	if strings.ToLower(user.Email) != strings.ToLower(invitation.Email) {
		c.JSON(http.StatusForbidden, gin.H{"error": "This invitation was sent to a different email address"})
		return
	}

	if err := h.repos.TeamInvitations.Decline(ctx, invitation.ID); err != nil {
		h.logger.Error(ctx, "Failed to decline invitation", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decline invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation declined"})
}

// ListMyInvitations returns all pending invitations for the current user
func (h *Handler) ListMyInvitations(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	currentUserID := userID.(uuid.UUID)

	ctx := c.Request.Context()

	// Get current user email
	user, err := h.repos.Users.GetByID(ctx, currentUserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	invitations, err := h.repos.TeamInvitations.ListPendingByEmail(ctx, user.Email)
	if err != nil {
		h.logger.Error(ctx, "Failed to list invitations", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list invitations"})
		return
	}

	responses := make([]InvitationDetailsResponse, 0, len(invitations))
	for _, inv := range invitations {
		responses = append(responses, InvitationDetailsResponse{
			ID:          inv.ID,
			Email:       inv.Email,
			Role:        inv.Role,
			Status:      inv.Status,
			TeamName:    inv.TeamName,
			TeamSlug:    inv.TeamSlug,
			InviterName: inv.InviterName,
			ExpiresAt:   inv.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			CreatedAt:   inv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"invitations": responses})
}
