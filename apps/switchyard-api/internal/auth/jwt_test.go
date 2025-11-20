package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewJWTManager(t *testing.T) {
	tests := []struct {
		name      string
		secretKey string
		wantErr   bool
	}{
		{
			name:      "valid secret key",
			secretKey: "test-secret-key-32-chars-long!!",
			wantErr:   false,
		},
		{
			name:      "short secret key",
			secretKey: "short",
			wantErr:   true,
		},
		{
			name:      "empty secret key",
			secretKey: "",
			wantErr:   true,
		},
		{
			name:      "exactly 32 chars",
			secretKey: "12345678901234567890123456789012",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewJWTManager(tt.secretKey)

			if tt.wantErr {
				if err == nil {
					t.Error("NewJWTManager() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewJWTManager() unexpected error: %v", err)
				return
			}

			if manager == nil {
				t.Error("NewJWTManager() returned nil manager")
			}
		})
	}
}

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret-key-32-chars-long!!")
	if err != nil {
		t.Fatalf("NewJWTManager() failed: %v", err)
	}

	userID := uuid.New()
	email := "test@example.com"
	role := "developer"

	token, err := manager.GenerateAccessToken(userID, email, role)

	if err != nil {
		t.Errorf("GenerateAccessToken() unexpected error: %v", err)
		return
	}

	if token == "" {
		t.Error("GenerateAccessToken() returned empty token")
		return
	}

	// Token should have 3 parts separated by dots (header.payload.signature)
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("GenerateAccessToken() token has %d dots, want 2 (JWT format)", parts)
	}
}

func TestJWTManager_ValidateAccessToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()
	email := "test@example.com"
	role := "developer"

	token, _ := manager.GenerateAccessToken(userID, email, role)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   token,
			wantErr: false,
		},
		{
			name:    "invalid token",
			token:   "invalid.token.here",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "malformed token",
			token:   "not-a-jwt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ValidateAccessToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateAccessToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateAccessToken() unexpected error: %v", err)
				return
			}

			if claims == nil {
				t.Error("ValidateAccessToken() returned nil claims")
				return
			}

			if claims.UserID != userID {
				t.Errorf("ValidateAccessToken() userID = %v, want %v", claims.UserID, userID)
			}

			if claims.Email != email {
				t.Errorf("ValidateAccessToken() email = %v, want %v", claims.Email, email)
			}

			if claims.Role != role {
				t.Errorf("ValidateAccessToken() role = %v, want %v", claims.Role, role)
			}
		})
	}
}

func TestJWTManager_ValidateAccessToken_DifferentSecret(t *testing.T) {
	manager1, _ := NewJWTManager("test-secret-key-32-chars-long!!")
	manager2, _ := NewJWTManager("different-secret-key-32-chars!")

	userID := uuid.New()
	token, _ := manager1.GenerateAccessToken(userID, "test@example.com", "developer")

	// Token signed with manager1's secret should fail validation with manager2
	_, err := manager2.ValidateAccessToken(token)

	if err == nil {
		t.Error("ValidateAccessToken() should fail with different secret key")
	}
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()

	token, err := manager.GenerateRefreshToken(userID)

	if err != nil {
		t.Errorf("GenerateRefreshToken() unexpected error: %v", err)
		return
	}

	if token == "" {
		t.Error("GenerateRefreshToken() returned empty token")
	}
}

func TestJWTManager_ValidateRefreshToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()
	token, _ := manager.GenerateRefreshToken(userID)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid refresh token",
			token:   token,
			wantErr: false,
		},
		{
			name:    "invalid token",
			token:   "invalid.token.here",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ValidateRefreshToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateRefreshToken() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateRefreshToken() unexpected error: %v", err)
				return
			}

			if claims == nil {
				t.Error("ValidateRefreshToken() returned nil claims")
				return
			}

			if claims.UserID != userID {
				t.Errorf("ValidateRefreshToken() userID = %v, want %v", claims.UserID, userID)
			}
		})
	}
}

func TestJWTManager_AccessToken_CannotBeUsedAsRefreshToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()
	accessToken, _ := manager.GenerateAccessToken(userID, "test@example.com", "developer")

	// Trying to validate access token as refresh token should fail
	_, err := manager.ValidateRefreshToken(accessToken)

	if err == nil {
		t.Error("ValidateRefreshToken() should reject access token")
	}
}

func TestJWTManager_RefreshToken_CannotBeUsedAsAccessToken(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()
	refreshToken, _ := manager.GenerateRefreshToken(userID)

	// Trying to validate refresh token as access token should fail
	_, err := manager.ValidateAccessToken(refreshToken)

	if err == nil {
		t.Error("ValidateAccessToken() should reject refresh token")
	}
}

func TestJWTManager_TokenExpiration(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	// Create a manager with very short expiration for testing
	manager.accessTokenDuration = 1 * time.Millisecond

	userID := uuid.New()
	token, _ := manager.GenerateAccessToken(userID, "test@example.com", "developer")

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err := manager.ValidateAccessToken(token)

	if err == nil {
		t.Error("ValidateAccessToken() should reject expired token")
	}
}

func TestJWTManager_TokenUniqueness(t *testing.T) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")

	userID := uuid.New()
	email := "test@example.com"
	role := "developer"

	token1, _ := manager.GenerateAccessToken(userID, email, role)
	// Small delay to ensure different timestamp
	time.Sleep(1 * time.Millisecond)
	token2, _ := manager.GenerateAccessToken(userID, email, role)

	if token1 == token2 {
		t.Error("GenerateAccessToken() produced identical tokens (should have different timestamps)")
	}

	// Both tokens should validate correctly
	claims1, err1 := manager.ValidateAccessToken(token1)
	claims2, err2 := manager.ValidateAccessToken(token2)

	if err1 != nil || err2 != nil {
		t.Fatalf("ValidateAccessToken() errors: %v, %v", err1, err2)
	}

	if claims1.UserID != claims2.UserID {
		t.Error("Tokens should have same user ID")
	}
}

func BenchmarkGenerateAccessToken(b *testing.B) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GenerateAccessToken(userID, "test@example.com", "developer")
	}
}

func BenchmarkValidateAccessToken(b *testing.B) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")
	userID := uuid.New()
	token, _ := manager.GenerateAccessToken(userID, "test@example.com", "developer")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ValidateAccessToken(token)
	}
}

func BenchmarkGenerateRefreshToken(b *testing.B) {
	manager, _ := NewJWTManager("test-secret-key-32-chars-long!!")
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GenerateRefreshToken(userID)
	}
}
