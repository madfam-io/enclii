package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User         *types.User `json:"user"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    time.Time   `json:"expires_at"`
	TokenType    string      `json:"token_type"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshResponse represents a successful token refresh response
type RefreshResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	TokenType   string    `json:"token_type"`
}

// Login handles user authentication with email and password
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Get user by email
	user, err := h.repos.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't reveal whether user exists or not
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		h.logger.Error(ctx, "Failed to get user by email",
			"error", err,
			"email", req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Check if user is active
	if !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is disabled"})
		return
	}

	// Compare password
	if err := auth.ComparePassword(user.PasswordHash, req.Password); err != nil {
		// Log failed login attempt
		h.repos.AuditLogs.Log(ctx, &types.AuditLog{
			ActorID:      user.ID,
			ActorEmail:   user.Email,
			ActorRole:    types.RoleViewer, // Unknown role at this point
			Action:       "login_failed",
			ResourceType: "user",
			ResourceID:   user.ID.String(),
			ResourceName: user.Email,
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Outcome:      "failure",
			Context: map[string]interface{}{
				"reason": "invalid_password",
			},
		})

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Get user's default role (from first project access or default to viewer)
	// For now, we'll use a simple default role
	// TODO: Get actual role from project_access
	userRole := string(types.RoleViewer)

	// Generate token pair
	tokenPair, err := h.auth.GenerateTokenPair(&auth.User{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       userRole,
		ProjectIDs: []string{}, // TODO: Populate from project_access
		CreatedAt:  user.CreatedAt,
		Active:     user.Active,
	})
	if err != nil {
		h.logger.Error(ctx, "Failed to generate token pair",
			"error", err,
			"user_id", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Update last login timestamp
	if err := h.repos.Users.UpdateLastLogin(ctx, user.ID); err != nil {
		h.logger.Warn(ctx, "Failed to update last login time",
			"error", err,
			"user_id", user.ID)
		// Non-fatal, continue
	}

	// Log successful login
	h.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      user.ID,
		ActorEmail:   user.Email,
		ActorRole:    types.Role(userRole),
		Action:       "login_success",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Email,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Outcome:      "success",
		Context: map[string]interface{}{
			"method": "password",
		},
	})

	// Return response
	c.JSON(http.StatusOK, LoginResponse{
		User:         user,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    "Bearer",
	})
}

// Logout handles user logout by revoking the refresh token
func (h *Handler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userEmail, _ := c.Get("email")

	// TODO: Implement session revocation
	// For now, we'll just log the logout event
	// In a full implementation, we would:
	// 1. Get refresh token from request
	// 2. Mark session as revoked in database
	// 3. Optionally add token to a blacklist/cache

	// Log logout event
	h.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      userID.(string),
		ActorEmail:   userEmail.(string),
		ActorRole:    types.Role(c.GetString("role")),
		Action:       "logout",
		ResourceType: "user",
		ResourceID:   userID.(string),
		ResourceName: userEmail.(string),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Outcome:      "success",
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Verify refresh token
	claims, err := h.auth.VerifyToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Check if token type is refresh
	if claims.TokenType != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
		return
	}

	// TODO: Check if session is revoked in database

	// Get user to verify they still exist and are active
	user, err := h.repos.Users.GetByID(ctx, claims.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error(ctx, "Failed to get user by ID",
			"error", err,
			"user_id", claims.UserID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is disabled"})
		return
	}

	// Generate new token pair
	tokenPair, err := h.auth.GenerateTokenPair(&auth.User{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       claims.Role,
		ProjectIDs: claims.ProjectIDs,
		CreatedAt:  user.CreatedAt,
		Active:     user.Active,
	})
	if err != nil {
		h.logger.Error(ctx, "Failed to generate token pair",
			"error", err,
			"user_id", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Log token refresh
	h.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      user.ID,
		ActorEmail:   user.Email,
		ActorRole:    types.Role(claims.Role),
		Action:       "token_refresh",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Email,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Outcome:      "success",
	})

	c.JSON(http.StatusOK, RefreshResponse{
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
		TokenType:   "Bearer",
	})
}

// Register handles new user registration
func (h *Handler) Register(c *gin.Context) {
	type RegisterRequest struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
		Name     string `json:"name" binding:"required"`
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Validate password strength
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	existingUser, err := h.repos.Users.GetByEmail(ctx, req.Email)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error(ctx, "Failed to check existing user",
			"error", err,
			"email", req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logger.Error(ctx, "Failed to hash password", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Create user
	user := &types.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Name:         req.Name,
		Active:       true,
	}

	if err := h.repos.Users.Create(ctx, user); err != nil {
		h.logger.Error(ctx, "Failed to create user",
			"error", err,
			"email", req.Email)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Log registration
	h.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      user.ID,
		ActorEmail:   user.Email,
		ActorRole:    types.RoleViewer,
		Action:       "user_register",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Email,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Outcome:      "success",
	})

	// Generate token pair for immediate login
	tokenPair, err := h.auth.GenerateTokenPair(&auth.User{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       string(types.RoleViewer),
		ProjectIDs: []string{},
		CreatedAt:  user.CreatedAt,
		Active:     user.Active,
	})
	if err != nil {
		h.logger.Error(ctx, "Failed to generate token pair",
			"error", err,
			"user_id", user.ID)
		// User created successfully, but token generation failed
		// Return user info but no tokens
		c.JSON(http.StatusCreated, gin.H{
			"user":    user,
			"message": "User created successfully. Please login.",
		})
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		User:         user,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		TokenType:    "Bearer",
	})
}
