package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
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
	cache           SessionRevoker // For session revocation

	// External JWKS validation (for CLI/API direct access with external tokens)
	externalJWKSURL   string
	externalIssuer    string
	externalJWKSCache *jwksCache

	// Admin email mapping for external tokens (grants admin role based on email)
	adminEmails map[string]bool
}

// jwksCache caches external JWKS keys with TTL and stale-while-revalidate support
type jwksCache struct {
	mu               sync.RWMutex
	keys             map[string]*rsa.PublicKey // kid -> public key
	expiresAt        time.Time
	cacheTTL         time.Duration
	lastFetchTime    time.Time
	lastFetchError   error
	consecutiveFails int
	staleThreshold   time.Duration // Warn if cache is stale beyond this
}

// SessionRevoker defines the interface for revoking sessions
// This is typically implemented using Redis/cache for fast lookups
type SessionRevoker interface {
	RevokeSession(ctx context.Context, sessionID string, ttl time.Duration) error
	IsSessionRevoked(ctx context.Context, sessionID string) (bool, error)
}

type Claims struct {
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	ProjectIDs []string  `json:"project_ids,omitempty"`
	SessionID  string    `json:"session_id"` // Unique session identifier for revocation
	TokenType  string    `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	// IDPToken is the access token from the identity provider (e.g., Janua)
	// Used for calling IDP-specific APIs like OAuth account linking
	IDPToken string `json:"idp_token,omitempty"`
	// IDPTokenExpiresAt is when the IDP token expires
	IDPTokenExpiresAt *time.Time `json:"idp_token_expires_at,omitempty"`
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

func NewJWTManager(tokenDuration, refreshDuration time.Duration, repos *db.Repositories, cache SessionRevoker) (*JWTManager, error) {
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
		cache:           cache,
	}, nil
}

// NewJWTManagerWithExternalJWKS creates a JWT manager with external JWKS validation support
func NewJWTManagerWithExternalJWKS(
	tokenDuration, refreshDuration time.Duration,
	repos *db.Repositories,
	cache SessionRevoker,
	externalJWKSURL string,
	externalIssuer string,
	jwksCacheTTL time.Duration,
) (*JWTManager, error) {
	manager, err := NewJWTManager(tokenDuration, refreshDuration, repos, cache)
	if err != nil {
		return nil, err
	}

	if externalJWKSURL != "" {
		manager.externalJWKSURL = externalJWKSURL
		manager.externalIssuer = externalIssuer
		manager.externalJWKSCache = &jwksCache{
			keys:           make(map[string]*rsa.PublicKey),
			cacheTTL:       jwksCacheTTL,
			staleThreshold: time.Hour, // Warn if cache is stale for more than 1 hour
		}
		logrus.WithFields(logrus.Fields{
			"jwks_url": externalJWKSURL,
			"issuer":   externalIssuer,
		}).Info("External JWKS validation enabled")
	}

	// Load admin emails from environment variable (comma-separated)
	// This grants admin role to users with these emails from external tokens
	manager.adminEmails = make(map[string]bool)
	if adminEmailsEnv := os.Getenv("ENCLII_ADMIN_EMAILS"); adminEmailsEnv != "" {
		for _, email := range strings.Split(adminEmailsEnv, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				manager.adminEmails[email] = true
				logrus.WithField("email", email).Info("Registered admin email for external tokens")
			}
		}
	}

	return manager, nil
}

func generateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func (j *JWTManager) GenerateTokenPair(user *User) (*TokenPair, error) {
	now := time.Now()

	// Generate unique session ID for this token pair
	// This allows us to revoke both access and refresh tokens together
	sessionID := uuid.New().String()

	// Generate access token
	accessClaims := &Claims{
		UserID:     user.ID,
		Email:      user.Email,
		Role:       user.Role,
		ProjectIDs: user.ProjectIDs,
		SessionID:  sessionID,
		TokenType:  "access",
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

	// Generate refresh token with same session ID
	refreshClaims := &Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		SessionID: sessionID,
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

	// SECURITY FIX: Check if session has been revoked (logout, security event, etc.)
	if j.cache != nil && claims.SessionID != "" {
		revoked, err := j.cache.IsSessionRevoked(context.Background(), claims.SessionID)
		if err != nil {
			logrus.Warnf("Failed to check session revocation: %v", err)
			// Don't fail validation on cache errors to prevent DoS, but log it
		} else if revoked {
			return nil, fmt.Errorf("session has been revoked")
		}
	}

	return claims, nil
}

func (j *JWTManager) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := j.validateRefreshToken(refreshTokenString)
	if err != nil {
		// Audit: Log refresh failure
		LogTokenRefreshFailed(err.Error(), "")
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Revoke old session (token rotation for security)
	if j.cache != nil && claims.SessionID != "" {
		if err := j.cache.RevokeSession(context.Background(), claims.SessionID, j.refreshDuration); err != nil {
			logrus.WithError(err).Warn("Failed to revoke old session during token refresh")
			// Continue anyway - new tokens will be valid
		}
	}

	// Create user from claims
	user := &User{
		ID:         claims.UserID,
		Email:      claims.Email,
		Role:       claims.Role,
		ProjectIDs: claims.ProjectIDs,
		Active:     true,
	}

	newTokens, err := j.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Audit: Log successful token refresh
	LogTokenRefreshed(claims.UserID, claims.SessionID)

	return newTokens, nil
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
// AuthMiddleware supports both Authorization header and query parameter (for WebSocket connections)
func (j *JWTManager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// Try Authorization header first (standard method)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			bearerToken := strings.Split(authHeader, " ")
			if len(bearerToken) == 2 && bearerToken[0] == "Bearer" {
				tokenString = bearerToken[1]
			}
		}

		// Fall back to query parameter (for WebSocket connections)
		// WebSocket API doesn't support custom headers, so token is passed via query param
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required (header or token query param)"})
			c.Abort()
			return
		}

		// Try local token validation first
		claims, err := j.ValidateToken(tokenString)
		if err == nil {
			// Local token validated successfully
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("project_ids", claims.ProjectIDs)
			c.Set("claims", claims)
			c.Next()
			return
		}

		// Local token validation failed - try external JWKS validation if configured
		if j.HasExternalJWKS() {
			externalClaims, externalErr := j.ValidateExternalToken(tokenString)
			if externalErr == nil {
				// External token validated successfully
				logrus.WithFields(logrus.Fields{
					"email":  externalClaims.Email,
					"issuer": externalClaims.Issuer,
				}).Debug("User authenticated via external token")

				// Parse the subject UUID
				userID := uuid.Nil
				if externalClaims.Subject != "" {
					if parsed, parseErr := uuid.Parse(externalClaims.Subject); parseErr == nil {
						userID = parsed
					}
				}

				// Determine role - default to developer, but check admin email mapping
				userRole := "developer"
				if j.adminEmails != nil && j.adminEmails[externalClaims.Email] {
					userRole = "admin"
					logrus.WithFields(logrus.Fields{
						"email":         externalClaims.Email,
						"original_role": "developer",
						"new_role":      "admin",
					}).Info("Applied admin role based on email mapping")
				}

				c.Set("user_id", userID)
				c.Set("user_email", externalClaims.Email)
				c.Set("user_role", userRole)
				c.Set("project_ids", []string{})
				c.Set("external_token", true)

				c.Next()
				return
			}
			logrus.WithError(externalErr).Debug("External token validation also failed")
		}

		// Both validations failed
		logrus.Warnf("Token validation failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		c.Abort()
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

		// Check if user has required role with hierarchy support
		// Role hierarchy: admin > developer > viewer
		// admin can do anything developer or viewer can do
		// developer can do anything viewer can do
		hasRole := false
		for _, role := range roles {
			if roleStr == role {
				hasRole = true
				break
			}
			// Apply role hierarchy: admin has all permissions
			if roleStr == "admin" {
				hasRole = true
				break
			}
			// developer can do viewer tasks
			if roleStr == "developer" && role == "viewer" {
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
		project, err := j.repos.Projects.GetBySlug(projectSlug)
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

// RevokeSession revokes a session by session ID
// The TTL should match the longest-lived token in the session (typically refresh token duration)
func (j *JWTManager) RevokeSession(ctx context.Context, sessionID string) error {
	if j.cache == nil {
		return fmt.Errorf("session revocation not available: cache not configured")
	}

	// Revoke for the duration of the refresh token (longest-lived)
	err := j.cache.RevokeSession(ctx, sessionID, j.refreshDuration)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	logrus.Infof("Session revoked: %s", sessionID)
	return nil
}

// RevokeSessionFromToken extracts the session ID from a token and revokes it
func (j *JWTManager) RevokeSessionFromToken(ctx context.Context, tokenString string) error {
	// Parse token without full validation (we just need the session ID)
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.publicKey, nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || claims.SessionID == "" {
		return fmt.Errorf("invalid token or missing session ID")
	}

	return j.RevokeSession(ctx, claims.SessionID)
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

// GetJWKS returns the JSON Web Key Set for token verification
// This allows external services to verify tokens we issue
func (j *JWTManager) GetJWKS() map[string]interface{} {
	// Convert RSA public key to JWK format
	// The public key components: n (modulus) and e (exponent)
	n := base64.RawURLEncoding.EncodeToString(j.publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(j.publicKey.E)).Bytes())

	return map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": "enclii-jwt-key-1",
				"n":   n, // RSA modulus
				"e":   e, // RSA public exponent
			},
		},
	}
}

// =============================================================================
// EXTERNAL JWKS VALIDATION (for CLI/API direct access with external tokens)
// =============================================================================

// ExternalClaims represents claims from external JWT tokens (e.g., Janua)
type ExternalClaims struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id,omitempty"`
	jwt.RegisteredClaims
}

// ValidateExternalToken validates a token against the external JWKS (e.g., Janua)
// Returns the claims if valid, nil otherwise
func (j *JWTManager) ValidateExternalToken(tokenString string) (*ExternalClaims, error) {
	if j.externalJWKSCache == nil || j.externalJWKSURL == "" {
		return nil, fmt.Errorf("external JWKS validation not configured")
	}

	// Parse token header to get kid
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &ExternalClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token header: %w", err)
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		kid = "" // Some providers don't use kid
	}

	// Get public key from cache or fetch
	publicKey, err := j.getExternalPublicKey(kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get external public key: %w", err)
	}

	// Parse and validate token with external key
	parsedToken, err := jwt.ParseWithClaims(tokenString, &ExternalClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := parsedToken.Claims.(*ExternalClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse claims")
	}

	// Verify issuer if configured
	if j.externalIssuer != "" && claims.Issuer != j.externalIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", j.externalIssuer, claims.Issuer)
	}

	logrus.WithFields(logrus.Fields{
		"email":  claims.Email,
		"issuer": claims.Issuer,
		"sub":    claims.Subject,
	}).Debug("External token validated successfully")

	return claims, nil
}

// getExternalPublicKey retrieves the public key for the given kid from cache or fetches from JWKS
func (j *JWTManager) getExternalPublicKey(kid string) (*rsa.PublicKey, error) {
	j.externalJWKSCache.mu.RLock()
	if time.Now().Before(j.externalJWKSCache.expiresAt) {
		if key, ok := j.externalJWKSCache.keys[kid]; ok {
			j.externalJWKSCache.mu.RUnlock()
			return key, nil
		}
		// If no kid specified, try first key
		if kid == "" && len(j.externalJWKSCache.keys) > 0 {
			for _, key := range j.externalJWKSCache.keys {
				j.externalJWKSCache.mu.RUnlock()
				return key, nil
			}
		}
	}
	j.externalJWKSCache.mu.RUnlock()

	// Fetch fresh JWKS
	if err := j.refreshExternalJWKS(); err != nil {
		return nil, err
	}

	j.externalJWKSCache.mu.RLock()
	defer j.externalJWKSCache.mu.RUnlock()

	if key, ok := j.externalJWKSCache.keys[kid]; ok {
		return key, nil
	}
	// If no kid specified or not found, try first key
	if len(j.externalJWKSCache.keys) > 0 {
		for _, key := range j.externalJWKSCache.keys {
			return key, nil
		}
	}

	return nil, fmt.Errorf("key not found in JWKS: %s", kid)
}

// refreshExternalJWKS fetches the JWKS from the external provider with graceful failure handling
// Implements stale-while-revalidate: uses cached keys if fetch fails
func (j *JWTManager) refreshExternalJWKS() error {
	j.externalJWKSCache.mu.Lock()
	defer j.externalJWKSCache.mu.Unlock()

	logrus.WithField("url", j.externalJWKSURL).Debug("Fetching external JWKS")

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(j.externalJWKSURL)
	if err != nil {
		return j.handleJWKSFetchError(fmt.Errorf("failed to fetch JWKS: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return j.handleJWKSFetchError(fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return j.handleJWKSFetchError(fmt.Errorf("failed to read JWKS response: %w", err))
	}

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.Unmarshal(body, &jwks); err != nil {
		return j.handleJWKSFetchError(fmt.Errorf("failed to parse JWKS: %w", err))
	}

	// Parse each key
	newKeys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}

		// Decode n and e
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			logrus.WithError(err).WithField("kid", key.Kid).Warn("Failed to decode key modulus")
			continue
		}

		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			logrus.WithError(err).WithField("kid", key.Kid).Warn("Failed to decode key exponent")
			continue
		}

		// Convert to big.Int
		n := new(big.Int).SetBytes(nBytes)
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}

		pubKey := &rsa.PublicKey{N: n, E: e}
		newKeys[key.Kid] = pubKey

		logrus.WithField("kid", key.Kid).Debug("Loaded external JWKS key")
	}

	if len(newKeys) == 0 {
		return j.handleJWKSFetchError(fmt.Errorf("no valid RSA keys found in JWKS"))
	}

	// Success - update cache and reset failure counters
	j.externalJWKSCache.keys = newKeys
	j.externalJWKSCache.expiresAt = time.Now().Add(j.externalJWKSCache.cacheTTL)
	j.externalJWKSCache.lastFetchTime = time.Now()
	j.externalJWKSCache.lastFetchError = nil
	j.externalJWKSCache.consecutiveFails = 0

	logrus.WithField("key_count", len(newKeys)).Info("External JWKS cache refreshed")

	return nil
}

// handleJWKSFetchError handles JWKS fetch failures with stale-while-revalidate semantics
// If we have cached keys, continue using them and log a warning
// Must be called with mutex held
func (j *JWTManager) handleJWKSFetchError(err error) error {
	j.externalJWKSCache.consecutiveFails++
	j.externalJWKSCache.lastFetchError = err

	// Check if we have cached keys to fall back to
	if len(j.externalJWKSCache.keys) > 0 {
		cacheAge := time.Since(j.externalJWKSCache.lastFetchTime)

		// Log warning about stale cache
		logrus.WithFields(logrus.Fields{
			"error":             err.Error(),
			"consecutive_fails": j.externalJWKSCache.consecutiveFails,
			"cache_age":         cacheAge.String(),
			"cached_keys":       len(j.externalJWKSCache.keys),
		}).Warn("JWKS fetch failed, using cached keys (stale-while-revalidate)")

		// Alert if cache is stale beyond threshold
		if cacheAge > j.externalJWKSCache.staleThreshold {
			logrus.WithFields(logrus.Fields{
				"cache_age":         cacheAge.String(),
				"stale_threshold":   j.externalJWKSCache.staleThreshold.String(),
				"consecutive_fails": j.externalJWKSCache.consecutiveFails,
			}).Error("CRITICAL: JWKS cache is stale beyond threshold - authentication may fail if keys rotate")
		}

		// Return nil to indicate we can continue with cached keys
		return nil
	}

	// No cached keys available - this is a hard failure
	logrus.WithFields(logrus.Fields{
		"error":             err.Error(),
		"consecutive_fails": j.externalJWKSCache.consecutiveFails,
	}).Error("JWKS fetch failed and no cached keys available")

	return err
}

// GetJWKSCacheStatus returns the current status of the JWKS cache for monitoring
func (j *JWTManager) GetJWKSCacheStatus() map[string]interface{} {
	if j.externalJWKSCache == nil {
		return nil
	}

	j.externalJWKSCache.mu.RLock()
	defer j.externalJWKSCache.mu.RUnlock()

	status := map[string]interface{}{
		"key_count":         len(j.externalJWKSCache.keys),
		"cache_ttl":         j.externalJWKSCache.cacheTTL.String(),
		"consecutive_fails": j.externalJWKSCache.consecutiveFails,
	}

	if !j.externalJWKSCache.lastFetchTime.IsZero() {
		status["last_fetch_time"] = j.externalJWKSCache.lastFetchTime.Format(time.RFC3339)
		status["cache_age_seconds"] = time.Since(j.externalJWKSCache.lastFetchTime).Seconds()
	}

	if !j.externalJWKSCache.expiresAt.IsZero() {
		status["expires_at"] = j.externalJWKSCache.expiresAt.Format(time.RFC3339)
		status["expired"] = time.Now().After(j.externalJWKSCache.expiresAt)
	}

	if j.externalJWKSCache.lastFetchError != nil {
		status["last_error"] = j.externalJWKSCache.lastFetchError.Error()
	}

	return status
}

// HasExternalJWKS returns true if external JWKS validation is configured
func (j *JWTManager) HasExternalJWKS() bool {
	return j.externalJWKSCache != nil && j.externalJWKSURL != ""
}

// GetExternalIssuer returns the expected issuer for external tokens
func (j *JWTManager) GetExternalIssuer() string {
	return j.externalIssuer
}
