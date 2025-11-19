package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/sirupsen/logrus"
)

type JWTManager struct {
	privateKey      *rsa.PrivateKey
	publicKey       *rsa.PublicKey
	tokenDuration   time.Duration
	refreshDuration time.Duration
	repos           *db.Repositories
}

type Claims struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	ProjectIDs  []string  `json:"project_ids,omitempty"`
	TokenType   string    `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

type User struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	ProjectIDs []string  `json:"project_ids"`
	CreatedAt  time.Time `json:"created_at"`
	Active     bool      `json:"active"`
}

func NewJWTManager(tokenDuration, refreshDuration time.Duration, repos *db.Repositories) (*JWTManager, error) {
	privateKey, err := generateRSAKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	return &JWTManager{
		privateKey:      privateKey,
		publicKey:       &privateKey.PublicKey,
		tokenDuration:   tokenDuration,
		refreshDuration: refreshDuration,
		repos:           repos,
	}, nil
}

func generateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func (j *JWTManager) GenerateTokenPair(user *User) (*TokenPair, error) {
	now := time.Now()
	
	// Generate access token
	accessClaims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		ProjectIDs: user.ProjectIDs,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.tokenDuration)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "enclii-switchyard",
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(j.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.refreshDuration)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "enclii-switchyard",
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(j.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    now.Add(j.tokenDuration),
		TokenType:    "Bearer",
	}, nil
}

func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("failed to parse claims")
	}

	// Additional validation
	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

func (j *JWTManager) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := j.validateRefreshToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Create user from claims
	user := &User{
		ID:    claims.UserID,
		Email: claims.Email,
		Role:  claims.Role,
		ProjectIDs: claims.ProjectIDs,
		Active: true,
	}

	return j.GenerateTokenPair(user)
}

func (j *JWTManager) validateRefreshToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("failed to parse claims")
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

// Middleware functions
func (j *JWTManager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := j.ValidateToken(bearerToken[1])
		if err != nil {
			logrus.Warnf("Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("project_ids", claims.ProjectIDs)
		c.Set("claims", claims)

		c.Next()
	}
}

func (j *JWTManager) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role format"})
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range roles {
			if roleStr == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Required role: %v, current role: %s", roles, roleStr),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (j *JWTManager) RequireProjectAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get project slug from URL params
		projectSlug := c.Param("slug")
		if projectSlug == "" {
			// No project in URL, skip check
			c.Next()
			return
		}

		// Get user ID from context (set by AuthMiddleware)
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Get user role from context
		roleStr, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}

		// Admin users have access to all projects
		if roleStr == "admin" {
			c.Next()
			return
		}

		// Check if repos are available
		if j.repos == nil {
			logrus.Warn("Project access repository not available, allowing request")
			c.Next()
			return
		}

		// Get project by slug
		project, err := j.repos.Projects.GetBySlug(ctx, projectSlug)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			} else {
				logrus.WithError(err).Error("Failed to get project by slug")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve project"})
			}
			c.Abort()
			return
		}

		// Check if user has access to this specific project
		hasAccess, err := j.repos.ProjectAccess.UserHasAccess(ctx, userID, project.ID)
		if err != nil {
			logrus.WithError(err).Error("Failed to check project access")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify project access"})
			c.Abort()
			return
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("You don't have access to project '%s'", projectSlug),
			})
			c.Abort()
			return
		}

		// User has access, store project ID in context for later use
		c.Set("project_id", project.ID)
		c.Next()
	}
}

// Context helpers
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}

	id, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid user ID format")
	}

	return id, nil
}

func GetUserEmailFromContext(c *gin.Context) (string, error) {
	email, exists := c.Get("user_email")
	if !exists {
		return "", fmt.Errorf("user email not found in context")
	}

	emailStr, ok := email.(string)
	if !ok {
		return "", fmt.Errorf("invalid email format")
	}

	return emailStr, nil
}

func GetClaimsFromContext(c *gin.Context) (*Claims, error) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, fmt.Errorf("claims not found in context")
	}

	claimsObj, ok := claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	return claimsObj, nil
}

// Export public key for verification by other services
func (j *JWTManager) ExportPublicKey() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(j.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}