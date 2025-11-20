package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// OIDCManager handles authentication via external OIDC providers (like Plinto)
type OIDCManager struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	repos        *db.Repositories
	jwtManager   *JWTManager // Used for issuing local session tokens
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

	// Create JWT manager for local session tokens
	// Even in OIDC mode, we issue local JWT tokens for API access
	jwtManager, err := NewJWTManager(
		15*time.Minute,  // access token duration
		7*24*time.Hour,  // refresh token duration
		repos,
		cache,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return &OIDCManager{
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Config,
		repos:        repos,
		jwtManager:   jwtManager,
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

	logrus.WithFields(logrus.Fields{
		"user_id":      user.ID,
		"email":        user.Email,
		"oidc_subject": claims.Sub,
		"oidc_issuer":  idToken.Issuer,
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
	user, err := o.repos.User.GetByOIDCIdentity(ctx, issuer, claims.Sub)
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
	user, err = o.repos.User.GetByEmail(ctx, claims.Email)
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
		if err := o.repos.User.Update(ctx, user); err != nil {
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

	newUser := &db.User{
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

	if err := o.repos.User.Create(ctx, newUser); err != nil {
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
func (o *OIDCManager) AuthMiddleware() gin.HandlerFunc {
	// In OIDC mode, we still validate local JWT session tokens
	// The OIDC flow issues these tokens after successful OAuth callback
	return o.jwtManager.AuthMiddleware()
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
	return o.jwtManager.RefreshToken(ctx, refreshToken)
}
