package services

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// AuthService handles authentication and authorization business logic
type AuthService struct {
	repos      *db.Repositories
	jwtManager *auth.JWTManager
	logger     *logrus.Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(
	repos *db.Repositories,
	jwtManager *auth.JWTManager,
	logger *logrus.Logger,
) *AuthService {
	return &AuthService{
		repos:      repos,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

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
	User         *types.User
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// Validate input
	if err := s.validateRegistrationInput(req); err != nil {
		return nil, err
	}

	// Check if email already exists
	existingUser, err := s.repos.Users.GetByEmail(ctx, req.Email)
	if err != nil && err != sql.ErrNoRows {
		s.logger.Error("Failed to check existing user", "error", err, "email", req.Email)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}
	if existingUser != nil {
		return nil, errors.ErrEmailAlreadyExists
	}

	s.logger.WithField("email", req.Email).Info("Registering new user")

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", "error", err)
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Create user
	user := &types.User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		Name:         req.Name,
		Active:       true,
	}

	if err := s.repos.Users.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", "error", err, "email", req.Email)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Get user's role and accessible projects from project_access table
	userRole, projectIDs := s.getUserRoleAndProjects(ctx, user.ID)

	// Generate token pair
	tokenPair, err := s.jwtManager.GenerateTokenPair(&auth.User{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       userRole,
		ProjectIDs: projectIDs,
		CreatedAt:  user.CreatedAt,
		Active:     user.Active,
	})
	if err != nil {
		s.logger.Error("Failed to generate token pair", "error", err, "user_id", user.ID)
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      user.ID,
		ActorEmail:   user.Email,
		ActorRole:    types.RoleViewer,
		Action:       "user_register",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Email,
		IPAddress:    req.IP,
		UserAgent:    req.UserAgent,
		Outcome:      "success",
	})

	// Clear password hash before returning
	user.PasswordHash = ""

	return &RegisterResponse{
		User:         user,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
	}, nil
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
	User         *types.User
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
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
	user, err := s.repos.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't reveal whether user exists or not
			return nil, errors.ErrInvalidCredentials
		}
		s.logger.Error("Failed to get user by email", "error", err, "email", req.Email)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Check if user is active
	if !user.Active {
		return nil, errors.ErrUnauthorized.WithDetails(map[string]any{
			"reason": "Account is disabled",
		})
	}

	// Verify password
	if err := auth.ComparePassword(user.PasswordHash, req.Password); err != nil {
		// Log failed login attempt
		s.repos.AuditLogs.Log(ctx, &types.AuditLog{
			ActorID:      user.ID,
			ActorEmail:   user.Email,
			ActorRole:    types.RoleViewer, // Unknown role at this point
			Action:       "login_failed",
			ResourceType: "user",
			ResourceID:   user.ID.String(),
			ResourceName: user.Email,
			IPAddress:    req.IP,
			UserAgent:    req.UserAgent,
			Outcome:      "failure",
			Context: map[string]interface{}{
				"reason": "invalid_password",
			},
		})
		return nil, errors.ErrInvalidCredentials
	}

	// Get user's role and accessible projects from project_access table
	userRole, projectIDs := s.getUserRoleAndProjects(ctx, user.ID)

	// Generate token pair
	tokenPair, err := s.jwtManager.GenerateTokenPair(&auth.User{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       userRole,
		ProjectIDs: projectIDs,
		CreatedAt:  user.CreatedAt,
		Active:     user.Active,
	})
	if err != nil {
		s.logger.Error("Failed to generate token pair", "error", err, "user_id", user.ID)
		return nil, errors.Wrap(err, errors.ErrInternal)
	}

	// Update last login timestamp
	if err := s.repos.Users.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.Warn("Failed to update last login time", "error", err, "user_id", user.ID)
		// Non-fatal, continue
	}

	// Log successful login
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      user.ID,
		ActorEmail:   user.Email,
		ActorRole:    types.Role(userRole),
		Action:       "login_success",
		ResourceType: "user",
		ResourceID:   user.ID.String(),
		ResourceName: user.Email,
		IPAddress:    req.IP,
		UserAgent:    req.UserAgent,
		Outcome:      "success",
		Context: map[string]interface{}{
			"method": "password",
		},
	})

	// Clear password hash before returning
	user.PasswordHash = ""

	return &LoginResponse{
		User:         user,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
	}, nil
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string
}

// RefreshTokenResponse represents the response from token refresh
type RefreshTokenResponse struct {
	AccessToken string
	ExpiresAt   time.Time
}

// RefreshToken generates new access and refresh tokens
func (s *AuthService) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*RefreshTokenResponse, error) {
	// Refresh token (validates token, checks revocation, and generates new token pair)
	tokenPair, err := s.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		s.logger.Warn("Token refresh failed", "error", err)
		return nil, errors.ErrTokenInvalid
	}

	// Note: RefreshToken creates a NEW session ID, which invalidates the old session
	// This provides automatic session rotation for security

	return &RefreshTokenResponse{
		AccessToken: tokenPair.AccessToken,
		ExpiresAt:   tokenPair.ExpiresAt,
	}, nil
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	UserID     string
	UserEmail  string
	UserRole   string
	TokenString string
	IP         string
	UserAgent  string
}

// Logout logs out a user (revokes session tokens)
func (s *AuthService) Logout(ctx context.Context, req *LogoutRequest) error {
	s.logger.WithField("user_id", req.UserID).Info("User logout")

	// Revoke the session (invalidates both access and refresh tokens with same session ID)
	if err := s.jwtManager.RevokeSessionFromToken(ctx, req.TokenString); err != nil {
		s.logger.Warn("Failed to revoke session during logout", "error", err, "user_id", req.UserID)
		// Continue with logout even if revocation fails
		// The token will still expire naturally
	}

	// Log logout event
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "logout",
		ResourceType: "user",
		ResourceID:   req.UserID,
		ResourceName: req.UserEmail,
		IPAddress:    req.IP,
		UserAgent:    req.UserAgent,
		Outcome:      "success",
		Context: map[string]interface{}{
			"session_revoked": true,
		},
	})

	return nil
}

// getUserRoleAndProjects retrieves the user's highest role and list of project IDs
// from their project access records.
func (s *AuthService) getUserRoleAndProjects(ctx context.Context, userID uuid.UUID) (string, []string) {
	// Query project access for this user
	accesses, err := s.repos.ProjectAccess.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to get user project access", "error", err, "user_id", userID)
		// Return viewer role and empty projects on error
		return string(types.RoleViewer), []string{}
	}

	// No project access - return viewer with no projects
	if len(accesses) == 0 {
		return string(types.RoleViewer), []string{}
	}

	// Collect unique project IDs and find highest role
	projectIDMap := make(map[string]bool)
	highestRole := types.RoleViewer // Start with lowest role

	for _, access := range accesses {
		// Add project ID to set
		projectIDMap[access.ProjectID.String()] = true

		// Determine highest role (admin > developer > viewer)
		switch access.Role {
		case types.RoleAdmin:
			highestRole = types.RoleAdmin
		case types.RoleDeveloper:
			if highestRole != types.RoleAdmin {
				highestRole = types.RoleDeveloper
			}
		case types.RoleViewer:
			// Already the lowest, no change needed
		}
	}

	// Convert map to slice
	projectIDs := make([]string, 0, len(projectIDMap))
	for projectID := range projectIDMap {
		projectIDs = append(projectIDs, projectID)
	}

	return string(highestRole), projectIDs
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

	// Validate password strength using auth package
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "password",
			"reason": err.Error(),
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
