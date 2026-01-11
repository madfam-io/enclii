package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
)

// API Token Management Handlers
// Provides endpoints for creating, listing, and revoking API tokens

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// CreateAPITokenRequest represents the request to create a new API token
type CreateAPITokenRequest struct {
	Name      string   `json:"name" binding:"required,min=1,max=100"`
	Scopes    []string `json:"scopes,omitempty"`          // Optional scopes (empty = full access)
	ExpiresIn *int     `json:"expires_in_days,omitempty"` // Optional expiration in days
}

// APITokenResponse represents a token in list responses (without the actual token)
type APITokenResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	Revoked    bool       `json:"revoked"`
}

// ============================================================================
// TOKEN CRUD HANDLERS
// ============================================================================

// CreateAPIToken creates a new API token for the authenticated user
// @Summary Create API token
// @Description Create a new API token for programmatic access
// @Tags tokens
// @Accept json
// @Produce json
// @Param request body CreateAPITokenRequest true "Token creation request"
// @Success 201 {object} types.APITokenCreateResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /v1/user/tokens [post]
func (h *Handler) CreateAPIToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		// Try parsing as string
		uidStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			return
		}
		var err error
		uid, err = uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}
	}

	var req CreateAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Check token limit (max 10 active tokens per user)
	count, err := h.repos.APITokens.CountByUser(ctx, uid)
	if err != nil {
		h.logger.Error(ctx, "Failed to count user tokens", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}
	if count >= 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum of 10 active tokens allowed. Please revoke unused tokens."})
		return
	}

	// Calculate expiration
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = &exp
	}

	// Create the token
	tokenResp, err := h.repos.APITokens.Create(ctx, uid, req.Name, req.Scopes, expiresAt)
	if err != nil {
		h.logger.Error(ctx, "Failed to create API token", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create token"})
		return
	}

	h.logger.Info(ctx, "API token created",
		logging.Field{Key: "token_id", Value: tokenResp.ID},
		logging.Field{Key: "name", Value: req.Name})

	c.JSON(http.StatusCreated, tokenResp)
}

// ListAPITokens lists all API tokens for the authenticated user
// @Summary List API tokens
// @Description Get all API tokens for the authenticated user
// @Tags tokens
// @Produce json
// @Param include_revoked query bool false "Include revoked tokens"
// @Success 200 {array} APITokenResponse
// @Failure 401 {object} map[string]string
// @Router /v1/user/tokens [get]
func (h *Handler) ListAPITokens(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		uidStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			return
		}
		var err error
		uid, err = uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}
	}

	// Check if we should include revoked tokens
	includeRevoked := c.Query("include_revoked") == "true"

	var tokens []*APITokenResponse

	if includeRevoked {
		dbTokens, err := h.repos.APITokens.ListByUser(ctx, uid)
		if err != nil {
			h.logger.Error(ctx, "Failed to list tokens", logging.Error("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tokens"})
			return
		}
		for _, t := range dbTokens {
			tokens = append(tokens, &APITokenResponse{
				ID:         t.ID,
				Name:       t.Name,
				Prefix:     t.Prefix,
				Scopes:     t.Scopes,
				ExpiresAt:  t.ExpiresAt,
				LastUsedAt: t.LastUsedAt,
				CreatedAt:  t.CreatedAt,
				Revoked:    t.Revoked,
			})
		}
	} else {
		dbTokens, err := h.repos.APITokens.ListActiveByUser(ctx, uid)
		if err != nil {
			h.logger.Error(ctx, "Failed to list tokens", logging.Error("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tokens"})
			return
		}
		for _, t := range dbTokens {
			tokens = append(tokens, &APITokenResponse{
				ID:         t.ID,
				Name:       t.Name,
				Prefix:     t.Prefix,
				Scopes:     t.Scopes,
				ExpiresAt:  t.ExpiresAt,
				LastUsedAt: t.LastUsedAt,
				CreatedAt:  t.CreatedAt,
				Revoked:    t.Revoked,
			})
		}
	}

	// Return empty array instead of null
	if tokens == nil {
		tokens = []*APITokenResponse{}
	}

	c.JSON(http.StatusOK, tokens)
}

// GetAPIToken gets details for a specific token
// @Summary Get API token
// @Description Get details for a specific API token
// @Tags tokens
// @Produce json
// @Param token_id path string true "Token ID"
// @Success 200 {object} APITokenResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /v1/user/tokens/{token_id} [get]
func (h *Handler) GetAPIToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		uidStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			return
		}
		var err error
		uid, err = uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}
	}

	// Parse token ID from path
	tokenIDStr := c.Param("token_id")
	tokenID, err := uuid.Parse(tokenIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID format"})
		return
	}

	// Get the token
	token, err := h.repos.APITokens.GetByID(ctx, tokenID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get token", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	// Verify ownership
	if token.UserID != uid {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	c.JSON(http.StatusOK, APITokenResponse{
		ID:         token.ID,
		Name:       token.Name,
		Prefix:     token.Prefix,
		Scopes:     token.Scopes,
		ExpiresAt:  token.ExpiresAt,
		LastUsedAt: token.LastUsedAt,
		CreatedAt:  token.CreatedAt,
		Revoked:    token.Revoked,
	})
}

// RevokeAPIToken revokes (soft deletes) an API token
// @Summary Revoke API token
// @Description Revoke an API token, preventing future use
// @Tags tokens
// @Param token_id path string true "Token ID"
// @Success 204 "Token revoked"
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /v1/user/tokens/{token_id} [delete]
func (h *Handler) RevokeAPIToken(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		uidStr, ok := userID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			return
		}
		var err error
		uid, err = uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}
	}

	// Parse token ID from path
	tokenIDStr := c.Param("token_id")
	tokenID, err := uuid.Parse(tokenIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID format"})
		return
	}

	// Revoke the token
	err = h.repos.APITokens.Revoke(ctx, tokenID, uid)
	if err != nil {
		h.logger.Error(ctx, "Failed to revoke token", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found or already revoked"})
		return
	}

	h.logger.Info(ctx, "API token revoked", logging.Field{Key: "token_id", Value: tokenID})

	c.Status(http.StatusNoContent)
}
