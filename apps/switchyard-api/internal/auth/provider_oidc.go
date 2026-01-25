package auth

import (
	"context"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/sirupsen/logrus"
)

// OIDCAuthProvider implements AuthenticationProvider for OIDC authentication mode.
// In OIDC mode, local authentication methods (Register, Login, RefreshToken) are not supported
// because users authenticate via an external identity provider (e.g., Janua SSO).
//
// The provider still supports:
//   - GenerateTokenPair: Creates local session tokens after successful OIDC callback
//   - Logout: Revokes local session tokens
//   - RevokeSessionFromToken: Session management for security events
type OIDCAuthProvider struct {
	repos       *db.Repositories
	oidcManager *OIDCManager
	logger      *logrus.Logger
}

// NewOIDCAuthProvider creates a new OIDC authentication provider.
func NewOIDCAuthProvider(
	repos *db.Repositories,
	oidcManager *OIDCManager,
	logger *logrus.Logger,
) *OIDCAuthProvider {
	return &OIDCAuthProvider{
		repos:       repos,
		oidcManager: oidcManager,
		logger:      logger,
	}
}

// Mode returns "oidc" to identify this as OIDC authentication.
func (p *OIDCAuthProvider) Mode() string {
	return "oidc"
}

// SupportsLocalAuth returns false - local auth is not supported in OIDC mode.
// Users must authenticate via the external identity provider.
func (p *OIDCAuthProvider) SupportsLocalAuth() bool {
	return false
}

// Register is not supported in OIDC mode.
// Returns ErrNotSupported with guidance to use the identity provider.
func (p *OIDCAuthProvider) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	p.logger.WithField("email", req.Email).Debug("Register attempt in OIDC mode - not supported")
	return nil, ErrNotSupported
}

// Login is not supported in OIDC mode.
// Returns ErrNotSupported with guidance to use the identity provider.
func (p *OIDCAuthProvider) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	p.logger.WithFields(logrus.Fields{
		"email": req.Email,
		"ip":    req.IP,
	}).Debug("Login attempt in OIDC mode - not supported")
	return nil, ErrNotSupported
}

// RefreshToken is not supported in OIDC mode.
// Users must re-authenticate via the identity provider.
func (p *OIDCAuthProvider) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error) {
	p.logger.Debug("RefreshToken attempt in OIDC mode - not supported")
	return nil, ErrNotSupported
}

// Logout revokes a session by session ID.
// This works in OIDC mode for revoking local session tokens.
func (p *OIDCAuthProvider) Logout(ctx context.Context, sessionID string) error {
	return p.oidcManager.Logout(ctx, sessionID)
}

// GenerateTokenPair creates access and refresh tokens for a user.
// This is used after successful OIDC callback to create local session tokens.
func (p *OIDCAuthProvider) GenerateTokenPair(user *User) (*TokenPair, error) {
	// The OIDC manager has an internal JWT manager for session token generation
	return p.oidcManager.jwtManager.GenerateTokenPair(user)
}

// RevokeSessionFromToken extracts session ID from token and revokes it.
func (p *OIDCAuthProvider) RevokeSessionFromToken(ctx context.Context, tokenString string) error {
	return p.oidcManager.jwtManager.RevokeSessionFromToken(ctx, tokenString)
}
