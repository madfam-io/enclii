package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/testutil"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

func newTestAuthService() (*AuthService, *db.Repositories) {
	repos := testutil.MockRepositories()
	jwtManager, _ := auth.NewJWTManager("test-secret-key-32-chars-long!!")
	auditLogger := audit.NewAsyncLogger(repos.AuditLogs, logrus.New(), 100)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs in tests

	service := NewAuthService(repos, jwtManager, auditLogger, logger)
	return service, repos
}

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name    string
		req     *RegisterRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid registration",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			req: &RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "Test User",
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "short password",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "short",
				Name:     "Test User",
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "empty name",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "",
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "name too long",
			req: &RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     string(make([]byte, 101)), // 101 chars
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newTestAuthService()
			ctx := context.Background()

			resp, err := service.Register(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Register() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("Register() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("Register() unexpected error: %v", err)
				return
			}

			if resp.User == nil {
				t.Error("Register() user is nil")
				return
			}

			if resp.User.Email != tt.req.Email {
				t.Errorf("Register() email = %v, want %v", resp.User.Email, tt.req.Email)
			}

			if resp.User.PasswordHash != "" {
				t.Error("Register() password hash should be cleared")
			}

			if resp.AccessToken == "" {
				t.Error("Register() access token is empty")
			}

			if resp.RefreshToken == "" {
				t.Error("Register() refresh token is empty")
			}
		})
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	service, repos := newTestAuthService()
	ctx := context.Background()

	// Create first user
	existingUser := &types.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "hash",
		Name:         "Existing",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repos.Users.Create(existingUser)

	// Try to register with same email
	req := &RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "New User",
	}

	_, err := service.Register(ctx, req)

	if err == nil {
		t.Error("Register() expected error for duplicate email, got nil")
		return
	}

	if !errors.Is(err, errors.ErrEmailAlreadyExists) {
		t.Errorf("Register() error = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestAuthService_Login(t *testing.T) {
	service, repos := newTestAuthService()
	ctx := context.Background()

	// Create a user
	password := "password123"
	hashedPassword, _ := auth.HashPassword(password)
	user := &types.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Name:         "Test User",
		DefaultRole:  types.RoleDeveloper,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repos.Users.Create(user)

	tests := []struct {
		name    string
		req     *LoginRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid login",
			req: &LoginRequest{
				Email:    "test@example.com",
				Password: password,
				IP:       "127.0.0.1",
				UserAgent: "test-agent",
			},
			wantErr: false,
		},
		{
			name: "wrong password",
			req: &LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
				IP:       "127.0.0.1",
				UserAgent: "test-agent",
			},
			wantErr: true,
			errType: errors.ErrInvalidCredentials,
		},
		{
			name: "non-existent user",
			req: &LoginRequest{
				Email:    "nonexistent@example.com",
				Password: password,
				IP:       "127.0.0.1",
				UserAgent: "test-agent",
			},
			wantErr: true,
			errType: errors.ErrInvalidCredentials,
		},
		{
			name: "empty email",
			req: &LoginRequest{
				Email:    "",
				Password: password,
				IP:       "127.0.0.1",
				UserAgent: "test-agent",
			},
			wantErr: true,
			errType: errors.ErrInvalidCredentials,
		},
		{
			name: "empty password",
			req: &LoginRequest{
				Email:    "test@example.com",
				Password: "",
				IP:       "127.0.0.1",
				UserAgent: "test-agent",
			},
			wantErr: true,
			errType: errors.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Login(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Login() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("Login() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("Login() unexpected error: %v", err)
				return
			}

			if resp.User == nil {
				t.Error("Login() user is nil")
				return
			}

			if resp.User.PasswordHash != "" {
				t.Error("Login() password hash should be cleared")
			}

			if resp.AccessToken == "" {
				t.Error("Login() access token is empty")
			}

			if resp.RefreshToken == "" {
				t.Error("Login() refresh token is empty")
			}
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	service, repos := newTestAuthService()
	ctx := context.Background()

	// Create a user
	user := &types.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "hash",
		Name:         "Test User",
		DefaultRole:  types.RoleDeveloper,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repos.Users.Create(user)

	// Generate refresh token
	jwtManager, _ := auth.NewJWTManager("test-secret-key-32-chars-long!!")
	refreshToken, _ := jwtManager.GenerateRefreshToken(user.ID)

	tests := []struct {
		name    string
		token   string
		userID  uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid refresh token",
			token:   refreshToken,
			userID:  user.ID,
			wantErr: false,
		},
		{
			name:    "invalid token",
			token:   "invalid-token",
			userID:  user.ID,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RefreshTokenRequest{
				RefreshToken: tt.token,
				UserID:       tt.userID,
			}

			resp, err := service.RefreshToken(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Error("RefreshToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("RefreshToken() unexpected error: %v", err)
				return
			}

			if resp.AccessToken == "" {
				t.Error("RefreshToken() access token is empty")
			}

			if resp.RefreshToken == "" {
				t.Error("RefreshToken() refresh token is empty")
			}
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	service, _ := newTestAuthService()
	ctx := context.Background()

	userID := uuid.New()
	req := &LogoutRequest{
		UserID:    userID,
		IP:        "127.0.0.1",
		UserAgent: "test-agent",
	}

	err := service.Logout(ctx, req)
	if err != nil {
		t.Errorf("Logout() unexpected error: %v", err)
	}
}

func TestAuthService_CheckAccess(t *testing.T) {
	service, repos := newTestAuthService()
	ctx := context.Background()

	userID := uuid.New()
	projectID := uuid.New()

	// Mock returns true by default
	err := service.CheckAccess(ctx, userID, projectID, nil, types.RoleDeveloper)
	if err != nil {
		t.Errorf("CheckAccess() unexpected error: %v", err)
	}
}

func Test_isValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"test@example.com", true},
		{"user.name@domain.com", true},
		{"user+tag@example.org", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"test@", false},
		{"", false},
		{"test @example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := isValidEmail(tt.email); got != tt.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}
