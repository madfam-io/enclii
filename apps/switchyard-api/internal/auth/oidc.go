package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// OIDCManager handles authentication via external OIDC providers (like Janua)
type OIDCManager struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	repos        *db.Repositories
	jwtManager   *JWTManager // Used for issuing local session tokens AND validating external tokens
	adminEmails  map[string]bool // email -> is admin (for OIDC fallback when tokens don't include roles)
}

// NewOIDCManager creates a new OIDC authentication manager
func NewOIDCManager(
	ctx context.Context,
	issuer string,
	clientID string,
	clientSecret string,
	redirectURL string,
	repos *db.Repositories,
	cache SessionRevoker,
) (*OIDCManager, error) {
	return NewOIDCManagerWithExternalJWKS(ctx, issuer, clientID, clientSecret, redirectURL, repos, cache, "", "", 0)
}

// NewOIDCManagerWithExternalJWKS creates an OIDC manager with external JWKS validation support
// This allows CLI/API clients to authenticate directly with external tokens (e.g., Janua)
func NewOIDCManagerWithExternalJWKS(
	ctx context.Context,
	issuer string,
	clientID string,
	clientSecret string,
	redirectURL string,
	repos *db.Repositories,
	cache SessionRevoker,
	externalJWKSURL string,
	externalIssuer string,
	jwksCacheTTL time.Duration,
) (*OIDCManager, error) {
	// Discover OIDC provider configuration
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: clientID,
	})

	// Configure OAuth2 client
	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Create JWT manager with external JWKS support
	var jwtManager *JWTManager
	if externalJWKSURL != "" {
		jwtManager, err = NewJWTManagerWithExternalJWKS(
			15*time.Minute,  // access token duration
			7*24*time.Hour,  // refresh token duration
			repos,
			cache,
			externalJWKSURL,
			externalIssuer,
			jwksCacheTTL,
		)
	} else {
		jwtManager, err = NewJWTManager(
			15*time.Minute,  // access token duration
			7*24*time.Hour,  // refresh token duration
			repos,
			cache,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	// Load admin emails from environment variable (comma-separated)
	// Example: ENCLII_ADMIN_EMAILS=admin@madfam.io,superuser@example.com
	adminEmails := make(map[string]bool)
	if adminEmailsEnv := os.Getenv("ENCLII_ADMIN_EMAILS"); adminEmailsEnv != "" {
		for _, email := range strings.Split(adminEmailsEnv, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				adminEmails[email] = true
				logrus.WithField("email", email).Info("Registered OIDC admin email")
			}
		}
	}

	return &OIDCManager{
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
		repos:        repos,
		jwtManager:   jwtManager,
		adminEmails:  adminEmails,
	}, nil
}

// GetAuthURL returns the OAuth authorization URL for redirecting users
func (o *OIDCManager) GetAuthURL(state string) string {
	return o.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// HandleCallback processes the OAuth callback and creates/updates user
func (o *OIDCManager) HandleCallback(ctx context.Context, code string) (*TokenPair, error) {
	// Exchange authorization code for token
	oauth2Token, err := o.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	// Extract ID token from OAuth2 token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	// Verify ID token signature and claims
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract standard claims
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Sub           string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// Require verified email
	if !claims.EmailVerified {
		return nil, fmt.Errorf("email not verified by OIDC provider")
	}

	// Get or create user
	user, err := o.getOrCreateUser(ctx, &claims, idToken.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create user: %w", err)
	}

	// Generate local JWT session tokens
	tokens, err := o.jwtManager.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session tokens: %w", err)
	}

	// Preserve the IDP access token for calling IDP-specific APIs
	// (e.g., Janua's OAuth account linking endpoint)
	if oauth2Token.AccessToken != "" {
		tokens.IDPToken = oauth2Token.AccessToken
		if !oauth2Token.Expiry.IsZero() {
			tokens.IDPTokenExpiresAt = &oauth2Token.Expiry
		}
	}

	logrus.WithFields(logrus.Fields{
		"user_id":       user.ID,
		"email":         user.Email,
		"oidc_subject":  claims.Sub,
		"oidc_issuer":   idToken.Issuer,
		"has_idp_token": tokens.IDPToken != "",
	}).Info("User authenticated via OIDC")

	return tokens, nil
}

// getOrCreateUser finds an existing user or creates a new one from OIDC claims
func (o *OIDCManager) getOrCreateUser(ctx context.Context, claims *struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Sub           string `json:"sub"`
}, issuer string) (*User, error) {
	// Try to find user by OIDC identity (issuer + subject)
	user, err := o.repos.Users.GetByOIDCIdentity(ctx, issuer, claims.Sub)
	if err == nil {
		// User found by OIDC identity
		logrus.WithFields(logrus.Fields{
			"user_id":      user.ID,
			"oidc_subject": claims.Sub,
		}).Debug("Found user by OIDC identity")
		return &User{
			ID:         user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Role:       user.Role,
			ProjectIDs: []string{}, // TODO: Load from project_access
			Active:     user.Active,
		}, nil
	}

	// Try to find user by email (migration from local auth)
	user, err = o.repos.Users.GetByEmail(ctx, claims.Email)
	if err == nil {
		// User exists from local auth - link to OIDC identity
		logrus.WithFields(logrus.Fields{
			"user_id":      user.ID,
			"email":        claims.Email,
			"oidc_subject": claims.Sub,
		}).Info("Linking existing user to OIDC identity")

		// Update user with OIDC identity
		user.OIDCSubject = &claims.Sub
		user.OIDCIssuer = &issuer
		if err := o.repos.Users.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to link OIDC identity: %w", err)
		}

		return &User{
			ID:         user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Role:       user.Role,
			ProjectIDs: []string{},
			Active:     user.Active,
		}, nil
	}

	// User doesn't exist - create new user from OIDC claims
	logrus.WithFields(logrus.Fields{
		"email":        claims.Email,
		"oidc_subject": claims.Sub,
	}).Info("Creating new user from OIDC")

	newUser := &types.User{
		ID:           uuid.New(),
		Email:        claims.Email,
		Name:         claims.Name,
		Role:         "developer", // Default role for new OIDC users
		Active:       true,
		OIDCSubject:  &claims.Sub,
		OIDCIssuer:   &issuer,
		PasswordHash: "", // No password for OIDC-only users
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := o.repos.Users.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:         newUser.ID,
		Email:      newUser.Email,
		Name:       newUser.Name,
		Role:       newUser.Role,
		ProjectIDs: []string{},
		Active:     newUser.Active,
	}, nil
}

// AuthMiddleware returns a Gin middleware for OIDC authentication
// Implements dual-mode validation: local tokens first, then external tokens
func (o *OIDCManager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
			c.JSON(401, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := bearerToken[1]

		// Try local token validation first
		localClaims, localErr := o.jwtManager.ValidateToken(tokenString)
		if localErr == nil {
			// Local token valid - set context and continue
			c.Set("user_id", localClaims.UserID)
			c.Set("user_email", localClaims.Email)
			c.Set("user_role", localClaims.Role)
			c.Set("project_ids", localClaims.ProjectIDs)
			c.Set("claims", localClaims)
			c.Set("token_source", "local")

			// Audit: Log successful local token validation
			LogTokenValidated(localClaims.UserID, localClaims.Email, "local")

			c.Next()
			return
		}

		// Local validation failed - try external JWKS if configured
		if o.jwtManager.HasExternalJWKS() {
			externalClaims, externalErr := o.jwtManager.ValidateExternalToken(tokenString)
			if externalErr == nil {
				// External token valid - get or create user
				user, isNew, err := o.getOrCreateUserFromExternalTokenWithStatus(c.Request.Context(), externalClaims)
				if err != nil {
					logrus.WithError(err).Error("Failed to get/create user from external token")
					c.JSON(401, gin.H{"error": "Failed to process external token"})
					c.Abort()
					return
				}

				// Set context with user info from external token
				c.Set("user_id", user.ID.String())
				c.Set("user_email", user.Email)

				// Apply admin email override if user's email is in adminEmails map
				// This enables OIDC providers (like Janua) that don't include roles in tokens
				userRole := user.Role
				if o.adminEmails[user.Email] {
					userRole = "admin"
					logrus.WithFields(logrus.Fields{
						"email":         user.Email,
						"original_role": user.Role,
						"new_role":      "admin",
					}).Info("Applied admin role based on email mapping")
				}
				c.Set("user_role", userRole)
				c.Set("project_ids", user.ProjectIDs)
				c.Set("token_source", "external")
				c.Set("external_issuer", externalClaims.Issuer)

				// Audit: Log external token validation and user creation/linking
				LogExternalTokenValidated(user.ID, user.Email, externalClaims.Issuer)
				if isNew {
					LogExternalUserCreated(user.ID, user.Email, externalClaims.Issuer)
				}

				logrus.WithFields(logrus.Fields{
					"user_id":  user.ID,
					"email":    user.Email,
					"issuer":   externalClaims.Issuer,
				}).Info("User authenticated via external token")

				c.Next()
				return
			}

			logrus.WithFields(logrus.Fields{
				"local_error":    localErr.Error(),
				"external_error": externalErr.Error(),
			}).Debug("Both local and external token validation failed")
		}

		// Both validations failed
		logrus.Warnf("Token validation failed: %v", localErr)
		c.JSON(401, gin.H{"error": "Invalid or expired token"})
		c.Abort()
	}
}

// getOrCreateUserFromExternalToken creates or updates a user from external token claims
func (o *OIDCManager) getOrCreateUserFromExternalToken(ctx context.Context, claims *ExternalClaims) (*User, error) {
	user, _, err := o.getOrCreateUserFromExternalTokenWithStatus(ctx, claims)
	return user, err
}

// getOrCreateUserFromExternalTokenWithStatus creates or updates a user from external token claims
// Returns the user, whether a new user was created, and any error
func (o *OIDCManager) getOrCreateUserFromExternalTokenWithStatus(ctx context.Context, claims *ExternalClaims) (*User, bool, error) {
	issuer := claims.Issuer

	// Try to find user by OIDC identity
	user, err := o.repos.Users.GetByOIDCIdentity(ctx, issuer, claims.Subject)
	if err == nil {
		return &User{
			ID:         user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Role:       user.Role,
			ProjectIDs: []string{},
			Active:     user.Active,
		}, false, nil
	}

	// Try to find by email
	user, err = o.repos.Users.GetByEmail(ctx, claims.Email)
	if err == nil {
		// Link existing user to external identity
		user.OIDCSubject = &claims.Subject
		user.OIDCIssuer = &issuer
		if err := o.repos.Users.Update(ctx, user); err != nil {
			return nil, false, fmt.Errorf("failed to link external identity: %w", err)
		}

		// Audit: Log user linking
		LogExternalUserLinked(user.ID, claims.Email, issuer)

		logrus.WithFields(logrus.Fields{
			"user_id": user.ID,
			"email":   claims.Email,
			"issuer":  issuer,
		}).Info("Linked existing user to external identity")

		return &User{
			ID:         user.ID,
			Email:      user.Email,
			Name:       user.Name,
			Role:       user.Role,
			ProjectIDs: []string{},
			Active:     user.Active,
		}, false, nil
	}

	// Create new user
	newUser := &types.User{
		ID:           uuid.New(),
		Email:        claims.Email,
		Name:         claims.Name,
		Role:         "developer",
		Active:       true,
		OIDCSubject:  &claims.Subject,
		OIDCIssuer:   &issuer,
		PasswordHash: "",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := o.repos.Users.Create(ctx, newUser); err != nil {
		return nil, false, fmt.Errorf("failed to create user: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"user_id": newUser.ID,
		"email":   claims.Email,
		"issuer":  issuer,
	}).Info("Created user from external token")

	return &User{
		ID:         newUser.ID,
		Email:      newUser.Email,
		Name:       newUser.Name,
		Role:       newUser.Role,
		ProjectIDs: []string{},
		Active:     newUser.Active,
	}, true, nil
}

// RequireRole returns a middleware that requires specific roles
func (o *OIDCManager) RequireRole(roles ...string) gin.HandlerFunc {
	return o.jwtManager.RequireRole(roles...)
}

// GenerateState generates a cryptographically random state parameter for CSRF protection
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Logout revokes the session (same as JWT manager)
func (o *OIDCManager) Logout(ctx context.Context, sessionID string) error {
	return o.jwtManager.RevokeSession(ctx, sessionID)
}

// ValidateToken validates a JWT token (for API access)
func (o *OIDCManager) ValidateToken(tokenString string) (*Claims, error) {
	return o.jwtManager.ValidateToken(tokenString)
}

// RefreshToken refreshes an access token using a refresh token
func (o *OIDCManager) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return o.jwtManager.RefreshToken(refreshToken)
}
