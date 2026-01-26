package api

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
)

// ListTeamMembers returns all members of a team
func (h *Handler) ListTeamMembers(c *gin.Context) {
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

// RemoveTeamMember removes a member from the team
func (h *Handler) RemoveTeamMember(c *gin.Context) {
	slug := c.Param("slug")
	memberIDStr := c.Param("member_id")

	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member ID"})
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
