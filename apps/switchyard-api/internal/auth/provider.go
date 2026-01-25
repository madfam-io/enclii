package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cache"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/sirupsen/logrus"
)

// ErrNotSupported indicates an operation is not supported in the current authentication mode.
// This is returned by OIDC providers when local authentication methods (Register, Login, RefreshToken) are called.
var ErrNotSupported = errors.New("operation not supported in current authentication mode")

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email     string
	Password  string
	Name      string
	IP        string
	UserAgent string
}

// RegisterResponse represents the response from registration
type RegisterResponse struct {
	UserID       uuid.UUID
	Email        string
	Name         string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email     string
	Password  string
	IP        string
	UserAgent string
}

// LoginResponse represents the response from login
type LoginResponse struct {
	UserID       uuid.UUID
	Email        string
	Name         string
	Role         string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string
}

// RefreshTokenResponse represents the response from token refresh
type RefreshTokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// AuthenticationProvider defines the interface for authentication operations.
// This allows the AuthService to work with both JWT (local) and OIDC authentication modes
// through a unified interface.
//
// In JWT mode (local authentication):
//   - All methods are fully functional
//   - Users register/login with email and password
//   - Tokens are issued and refreshed locally
//
// In OIDC mode:
//   - Register(), Login(), RefreshToken() return ErrNotSupported
//   - Authentication happens via external identity provider
//   - GenerateTokenPair() still works for session management after OIDC callback
type AuthenticationProvider interface {
	// Mode returns the authentication mode identifier ("local" or "oidc")
	Mode() string

	// Register creates a new user account with email and password.
	// Returns ErrNotSupported in OIDC mode - users register via the identity provider.
	Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error)

	// Login authenticates a user with email and password.
	// Returns ErrNotSupported in OIDC mode - users authenticate via the identity provider.
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)

	// RefreshToken generates new tokens using a refresh token.
	// Returns ErrNotSupported in OIDC mode - users re-authenticate via the identity provider.
	RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error)

	// Logout revokes a session by session ID.
	// Works in both modes - used to invalidate local session tokens.
	Logout(ctx context.Context, sessionID string) error

	// GenerateTokenPair creates access and refresh tokens for a user.
	// Works in both modes - used after OIDC callback to create local session tokens.
	GenerateTokenPair(user *User) (*TokenPair, error)

	// SupportsLocalAuth returns true if local authentication (email/password) is supported.
	// Returns true for JWT mode, false for OIDC mode.
	SupportsLocalAuth() bool

	// RevokeSessionFromToken extracts session ID from token and revokes it.
	// Works in both modes for session management.
	RevokeSessionFromToken(ctx context.Context, tokenString string) error
}

// NewAuthProvider creates the appropriate AuthenticationProvider based on configuration.
// This is a factory function that returns either LocalAuthProvider (JWT mode) or
// OIDCAuthProvider (OIDC mode) based on cfg.AuthMode.
func NewAuthProvider(
	ctx context.Context,
	cfg *config.Config,
	repos *db.Repositories,
	cacheService *cache.RedisCache,
	authManager AuthManager,
	logger *logrus.Logger,
) (AuthenticationProvider, error) {
	switch cfg.AuthMode {
	case "local", "":
		// JWT mode - local authentication
		jwtManager, ok := authManager.(*JWTManager)
		if !ok {
			return nil, errors.New("expected JWTManager for local auth mode")
		}
		return NewLocalAuthProvider(repos, jwtManager, logger), nil

	case "oidc":
		// OIDC mode - external identity provider
		oidcManager, ok := authManager.(*OIDCManager)
		if !ok {
			return nil, errors.New("expected OIDCManager for OIDC auth mode")
		}
		return NewOIDCAuthProvider(repos, oidcManager, logger), nil

	default:
		return nil, errors.New("unknown auth mode: " + cfg.AuthMode)
	}
}
