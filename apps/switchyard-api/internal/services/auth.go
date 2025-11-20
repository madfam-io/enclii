package services

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// AuthService handles authentication and authorization business logic
type AuthService struct {
	repos       *db.Repositories
	jwtManager  *auth.JWTManager
	auditLogger *audit.AsyncLogger
	logger      *logrus.Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(
	repos *db.Repositories,
	jwtManager *auth.JWTManager,
	auditLogger *audit.AsyncLogger,
	logger *logrus.Logger,
) *AuthService {
	return &AuthService{
		repos:       repos,
		jwtManager:  jwtManager,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email    string
	Password string
	Name     string
}

// RegisterResponse represents the response from registration
type RegisterResponse struct {
	User         *types.User
	AccessToken  string
	RefreshToken string
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// Validate input
	if err := s.validateRegistrationInput(req); err != nil {
		return nil, err
	}

	// Check if email already exists
	existing, _ := s.repos.Users.GetByEmail(req.Email)
	if existing != nil {
		return nil, errors.ErrEmailAlreadyExists
	}

	s.logger.WithField("email", req.Email).Info("Registering new user")

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Create user
	user := &types.User{
		ID:             uuid.New(),
		Email:          strings.ToLower(req.Email),
		PasswordHash:   hashedPassword,
		Name:           req.Name,
		OIDCProvider:   "",
		OIDCSubject:    "",
		EmailVerified:  false,
		DefaultRole:    types.RoleDeveloper,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repos.Users.Create(user); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.DefaultRole))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        user.ID.String(),
		Action:       "user_registered",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		Outcome:      "success",
	})

	// Clear password hash before returning
	user.PasswordHash = ""

	return &RegisterResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string
	Password string
	IP       string
	UserAgent string
}

// LoginResponse represents the response from login
type LoginResponse struct {
	User         *types.User
	AccessToken  string
	RefreshToken string
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Validate input
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, errors.ErrInvalidCredentials
	}

	s.logger.WithFields(logrus.Fields{
		"email": req.Email,
		"ip":    req.IP,
	}).Info("User login attempt")

	// Get user by email
	user, err := s.repos.Users.GetByEmail(strings.ToLower(req.Email))
	if err != nil {
		// Log failed attempt
		s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
			Actor:        req.Email,
			Action:       "login_failed",
			ResourceType: "user",
			ResourceID:   "",
			Outcome:      "failure",
			IPAddress:    req.IP,
			UserAgent:    req.UserAgent,
			Context: map[string]interface{}{
				"reason": "user_not_found",
			},
		})
		return nil, errors.ErrInvalidCredentials
	}

	// Verify password
	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		// Log failed attempt
		s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
			Actor:        user.ID.String(),
			Action:       "login_failed",
			ResourceType: "user",
			ResourceID:   user.ID.String(),
			Outcome:      "failure",
			IPAddress:    req.IP,
			UserAgent:    req.UserAgent,
			Context: map[string]interface{}{
				"reason": "invalid_password",
			},
		})
		return nil, errors.ErrInvalidCredentials
	}

	// Generate tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.DefaultRole))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Log successful login
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        user.ID.String(),
		Action:       "login_success",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		Outcome:      "success",
		IPAddress:    req.IP,
		UserAgent:    req.UserAgent,
	})

	// Clear password hash before returning
	user.PasswordHash = ""

	return &LoginResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string
	UserID       uuid.UUID
}

// RefreshTokenResponse represents the response from token refresh
type RefreshTokenResponse struct {
	AccessToken  string
	RefreshToken string
}

// RefreshToken generates new access and refresh tokens
func (s *AuthService) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, errors.ErrTokenInvalid
	}

	// Get user
	user, err := s.repos.Users.GetByID(claims.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrUnauthorized)
	}

	// Generate new tokens
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email, string(user.DefaultRole))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	return &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	UserID uuid.UUID
	IP     string
	UserAgent string
}

// Logout logs out a user (revokes tokens in production)
func (s *AuthService) Logout(ctx context.Context, req *LogoutRequest) error {
	s.logger.WithField("user_id", req.UserID).Info("User logout")

	// In production, this would revoke the session/tokens in Redis
	// For now, just audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "logout",
		ResourceType: "user",
		ResourceID:   req.UserID.String(),
		Outcome:      "success",
		IPAddress:    req.IP,
		UserAgent:    req.UserAgent,
	})

	return nil
}

// CheckAccess verifies if a user has access to a project/environment
func (s *AuthService) CheckAccess(
	ctx context.Context,
	userID, projectID uuid.UUID,
	environmentID *uuid.UUID,
	requiredRole types.Role,
) error {
	hasAccess, err := s.repos.ProjectAccess.HasAccess(ctx, userID, projectID, environmentID, requiredRole)
	if err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError)
	}

	if !hasAccess {
		return errors.ErrInsufficientPermissions.WithDetails(map[string]any{
			"required_role": requiredRole,
			"project_id":    projectID,
		})
	}

	return nil
}

// validateRegistrationInput validates registration input
func (s *AuthService) validateRegistrationInput(req *RegisterRequest) error {
	// Validate email
	if !isValidEmail(req.Email) {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "email",
			"reason": "Invalid email format",
		})
	}

	// Validate password strength
	if len(req.Password) < 8 {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "password",
			"reason": "Password must be at least 8 characters",
		})
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name is required",
		})
	}

	if len(req.Name) > 100 {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name must be 100 characters or less",
		})
	}

	return nil
}

// isValidEmail checks if an email address is valid
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}

	// Simple email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
