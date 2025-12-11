package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/sirupsen/logrus"
)

// AuthManager is the interface for all authentication methods
// This allows switching between local JWT and OIDC authentication
type AuthManager interface {
	// AuthMiddleware returns the Gin middleware for authentication
	AuthMiddleware() gin.HandlerFunc

	// RequireRole returns a middleware that requires specific roles
	RequireRole(roles ...string) gin.HandlerFunc
}

// NewAuthManager creates the appropriate auth manager based on configuration
// This is the factory pattern that enables Phase C bootstrap authentication
func NewAuthManager(
	ctx context.Context,
	cfg *config.Config,
	repos *db.Repositories,
	cache SessionRevoker,
) (AuthManager, error) {
	logrus.WithField("auth_mode", cfg.AuthMode).Info("Initializing authentication manager")

	switch cfg.AuthMode {
	case "local", "":
		// Bootstrap mode - local JWT authentication with optional external JWKS validation
		if cfg.ExternalJWKSURL != "" {
			// External JWKS configured - use enhanced JWT manager with external token support
			logrus.WithFields(logrus.Fields{
				"external_jwks_url": cfg.ExternalJWKSURL,
				"external_issuer":   cfg.ExternalIssuer,
			}).Info("Using local JWT authentication with external JWKS validation")
			jwksCacheTTL := time.Duration(cfg.ExternalJWKSCacheTTL) * time.Second
			return NewJWTManagerWithExternalJWKS(
				15*time.Minute,  // access token duration
				7*24*time.Hour,  // refresh token duration
				repos,
				cache,
				cfg.ExternalJWKSURL,
				cfg.ExternalIssuer,
				jwksCacheTTL,
			)
		}
		// No external JWKS - use basic local JWT
		logrus.Info("Using local JWT authentication (bootstrap mode)")
		return NewJWTManager(
			15*time.Minute,  // access token duration
			7*24*time.Hour,  // refresh token duration
			repos,
			cache,
		)

	case "oidc":
		// Production mode - OIDC authentication (after Janua is deployed)
		logrus.WithFields(logrus.Fields{
			"issuer":            cfg.OIDCIssuer,
			"client_id":         cfg.OIDCClientID,
			"redirect_url":      cfg.OIDCRedirectURL,
			"external_jwks_url": cfg.ExternalJWKSURL,
			"external_issuer":   cfg.ExternalIssuer,
		}).Info("Using OIDC authentication (production mode)")

		if cfg.OIDCIssuer == "" {
			return nil, fmt.Errorf("OIDC mode requires ENCLII_OIDC_ISSUER")
		}
		if cfg.OIDCClientID == "" {
			return nil, fmt.Errorf("OIDC mode requires ENCLII_OIDC_CLIENT_ID")
		}
		if cfg.OIDCClientSecret == "" {
			return nil, fmt.Errorf("OIDC mode requires ENCLII_OIDC_CLIENT_SECRET")
		}

		// Use external JWKS validation if configured (for CLI/API direct access)
		jwksCacheTTL := time.Duration(cfg.ExternalJWKSCacheTTL) * time.Second
		return NewOIDCManagerWithExternalJWKS(
			ctx,
			cfg.OIDCIssuer,
			cfg.OIDCClientID,
			cfg.OIDCClientSecret,
			cfg.OIDCRedirectURL,
			repos,
			cache,
			cfg.ExternalJWKSURL,
			cfg.ExternalIssuer,
			jwksCacheTTL,
		)

	default:
		return nil, fmt.Errorf("invalid auth mode: %s (must be 'local' or 'oidc')", cfg.AuthMode)
	}
}
