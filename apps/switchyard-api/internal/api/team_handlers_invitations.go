package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/notifications"
)

// InviteTeamMember creates an invitation to join the team
func (h *Handler) InviteTeamMember(c *gin.Context) {
	slug := c.Param("slug")

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

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

	// Send invitation email (non-blocking, log failures)
	if h.emailService != nil {
		// Get inviter details for the email
		inviter, err := h.repos.Users.GetByID(ctx, currentUserID)
		if err != nil {
			h.logger.Warn(ctx, "Failed to get inviter details for email",
				logging.String("inviter_id", currentUserID.String()),
				logging.Error("error", err))
		} else {
			inviterName := inviter.Name
			if inviterName == "" {
				inviterName = inviter.Email // Fallback to email if name not set
			}

			emailData := notifications.TeamInvitationData{
				InviteeEmail:    invitation.Email,
				TeamName:        team.Name,
				TeamSlug:        team.Slug,
				InviterName:     inviterName,
				InviterEmail:    inviter.Email,
				Role:            invitation.Role,
				InvitationToken: invitation.Token,
				ExpiresAt:       invitation.ExpiresAt,
			}

			// Send asynchronously to not block the response
			go func() {
				emailCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := h.emailService.SendTeamInvitation(emailCtx, emailData); err != nil {
					h.logger.Error(emailCtx, "Failed to send invitation email",
						logging.String("email", invitation.Email),
						logging.String("team", team.Slug),
						logging.Error("error", err))
				}
			}()
		}
	}

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

// CancelTeamInvitation cancels a pending invitation
func (h *Handler) CancelTeamInvitation(c *gin.Context) {
	slug := c.Param("slug")
	invitationIDStr := c.Param("invitation_id")

	invitationID, err := uuid.Parse(invitationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid invitation ID"})
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

	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

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

	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

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
	currentUserID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

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
