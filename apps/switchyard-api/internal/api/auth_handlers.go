package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/services"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
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

	// Use service layer for authentication
	loginReq := &services.LoginRequest{
		Email:     req.Email,
		Password:  req.Password,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}

	resp, err := h.authService.Login(ctx, loginReq)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, errors.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		} else if errors.Is(err, errors.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	// Return response
	c.JSON(http.StatusOK, LoginResponse{
		User:         resp.User,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		TokenType:    "Bearer",
	})
}

// Logout handles user logout by revoking the session
func (h *Handler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userEmail, _ := c.Get("user_email")

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization header required"})
		return
	}

	// Parse Bearer token
	tokenString := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorization header format"})
		return
	}

	// Use service layer for logout
	logoutReq := &services.LogoutRequest{
		UserID:      userID.(string),
		UserEmail:   userEmail.(string),
		UserRole:    c.GetString("user_role"),
		TokenString: tokenString,
		IP:          c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	}

	if err := h.authService.Logout(ctx, logoutReq); err != nil {
		h.logger.Error("Logout failed", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

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

	// Use service layer for token refresh
	refreshReq := &services.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	resp, err := h.authService.RefreshToken(ctx, refreshReq)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, errors.ErrTokenInvalid) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, RefreshResponse{
		AccessToken: resp.AccessToken,
		ExpiresAt:   resp.ExpiresAt,
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

	// Use service layer for registration
	registerReq := &services.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		Name:      req.Name,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}

	resp, err := h.authService.Register(ctx, registerReq)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, errors.ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		} else if errors.Is(err, errors.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		}
		return
	}

	c.JSON(http.StatusCreated, LoginResponse{
		User:         resp.User,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    resp.ExpiresAt,
		TokenType:    "Bearer",
	})
}

// =============== OIDC Authentication Handlers (Phase C) ===============

// OIDCLogin redirects to the OIDC provider (Plinto) for authentication
func (h *Handler) OIDCLogin(c *gin.Context) {
	// Check if OIDC mode is enabled
	if h.config.AuthMode != "oidc" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "OIDC authentication is not enabled",
			"hint":  "Set ENCLII_AUTH_MODE=oidc to enable OIDC authentication",
		})
		return
	}

	// Cast auth manager to OIDC manager
	oidcMgr, ok := h.auth.(*auth.OIDCManager)
	if !ok {
		logrus.Error("Auth manager is not OIDCManager despite OIDC mode being set")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Authentication system misconfigured",
		})
		return
	}

	// Generate random state for CSRF protection
	state, err := auth.GenerateState()
	if err != nil {
		logrus.WithError(err).Error("Failed to generate OAuth state")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authentication state",
		})
		return
	}

	// Store state in secure cookie (HttpOnly, Secure, SameSite)
	c.SetCookie(
		"oauth_state",
		state,
		300, // 5 minutes
		"/",
		"",    // domain (empty = current domain)
		true,  // secure (HTTPS only)
		true,  // httpOnly
	)
	c.SetSameSite(http.SameSiteLaxMode)

	// Get authorization URL from OIDC provider
	authURL := oidcMgr.GetAuthURL(state)

	logrus.WithFields(logrus.Fields{
		"auth_url": authURL,
		"state":    state[:10] + "...", // Log only first 10 chars for security
	}).Info("Redirecting to OIDC provider")

	// Redirect to OIDC provider (Plinto)
	c.Redirect(http.StatusFound, authURL)
}

// OIDCCallback handles the OAuth callback from the OIDC provider (Plinto)
func (h *Handler) OIDCCallback(c *gin.Context) {
	ctx := c.Request.Context()

	// Verify state parameter (CSRF protection)
	savedState, err := c.Cookie("oauth_state")
	if err != nil {
		logrus.WithError(err).Warn("Missing OAuth state cookie")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid authentication state",
			"hint":  "State cookie not found. Ensure cookies are enabled.",
		})
		return
	}

	receivedState := c.Query("state")
	if savedState != receivedState {
		logrus.WithFields(logrus.Fields{
			"saved_state":    savedState[:10] + "...",
			"received_state": receivedState[:10] + "...",
		}).Warn("OAuth state mismatch")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid state parameter",
			"hint":  "State mismatch. This could be a CSRF attack or expired session.",
		})
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", true, true)

	// Check for error response from OIDC provider
	if errorParam := c.Query("error"); errorParam != "" {
		errorDesc := c.Query("error_description")
		logrus.WithFields(logrus.Fields{
			"error":             errorParam,
			"error_description": errorDesc,
		}).Warn("OIDC provider returned error")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       errorParam,
			"description": errorDesc,
		})
		return
	}

	// Get authorization code
	code := c.Query("code")
	if code == "" {
		logrus.Warn("Missing authorization code in OAuth callback")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing authorization code",
			"hint":  "The OAuth provider did not return an authorization code.",
		})
		return
	}

	// Exchange code for tokens
	oidcMgr, ok := h.auth.(*auth.OIDCManager)
	if !ok {
		logrus.Error("Auth manager is not OIDCManager in callback")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Authentication system misconfigured",
		})
		return
	}

	tokens, err := oidcMgr.HandleCallback(ctx, code)
	if err != nil {
		logrus.WithError(err).Error("OIDC callback failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Authentication failed",
			"hint":  "Could not complete OIDC authentication. Please try again.",
		})
		return
	}

	// Return tokens to client
	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		TokenType:    tokens.TokenType,
	})
}

// JWKS returns the JSON Web Key Set for token verification
// This endpoint is used by external services (like Plinto) to verify tokens we issue
func (h *Handler) JWKS(c *gin.Context) {
	// JWKS only available in local auth mode
	// In OIDC mode, tokens are verified against the OIDC provider's JWKS
	if h.config.AuthMode != "local" && h.config.AuthMode != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "JWKS endpoint not available in OIDC mode",
			"hint":  "In OIDC mode, use the OIDC provider's JWKS endpoint for token verification",
		})
		return
	}

	jwtMgr, ok := h.auth.(*auth.JWTManager)
	if !ok {
		logrus.Error("Auth manager is not JWTManager despite local mode")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Authentication system misconfigured",
		})
		return
	}

	// Get JWKS from JWT manager
	jwks := jwtMgr.GetJWKS()
	c.JSON(http.StatusOK, jwks)
}
