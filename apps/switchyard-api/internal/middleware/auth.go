package middleware

import (
	"crypto/rsa"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	publicKey     *rsa.PublicKey
	publicPaths   map[string]bool
	requiredRoles map[string][]string // path -> required roles
	adminEmails   map[string]bool     // email -> is admin (for OIDC fallback)
}

// NewAuthMiddleware creates a new authentication middleware
// publicKey is used to validate RS256 JWT tokens
func NewAuthMiddleware(publicKey *rsa.PublicKey) *AuthMiddleware {
	am := &AuthMiddleware{
		publicKey:     publicKey,
		publicPaths:   make(map[string]bool),
		requiredRoles: make(map[string][]string),
		adminEmails:   make(map[string]bool),
	}
	// Load admin emails from environment variable (comma-separated)
	// Example: ENCLII_ADMIN_EMAILS=admin@madfam.io,superuser@example.com
	if adminEmailsEnv := os.Getenv("ENCLII_ADMIN_EMAILS"); adminEmailsEnv != "" {
		for _, email := range strings.Split(adminEmailsEnv, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				am.adminEmails[email] = true
				logrus.WithField("email", email).Info("Registered admin email")
			}
		}
	}
	return am
}

// AddPublicPath adds a path that doesn't require authentication
func (a *AuthMiddleware) AddPublicPath(path string) {
	a.publicPaths[path] = true
}

// AddRoleRequirement adds a role requirement for a specific path
func (a *AuthMiddleware) AddRoleRequirement(path string, roles []string) {
	a.requiredRoles[path] = roles
}

// Middleware returns a Gin middleware function for authentication
func (a *AuthMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check if path is public
		if a.publicPaths[path] {
			c.Next()
			return
		}

		// Skip auth for health check
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/metrics") {
			c.Next()
			return
		}

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logrus.WithFields(logrus.Fields{
				"path":   path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Warn("Missing Authorization header")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			logrus.WithFields(logrus.Fields{
				"path":   path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Warn("Invalid Authorization header format")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header format. Expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate JWT token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method - tokens are signed with RS256
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.publicKey, nil
		})

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
				"error":  err.Error(),
			}).Warn("Invalid JWT token")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		if !token.Valid {
			logrus.WithFields(logrus.Fields{
				"path":   path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Warn("Invalid JWT token")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			logrus.WithFields(logrus.Fields{
				"path":   path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Warn("Invalid token claims")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			c.Abort()
			return
		}

		// Store user information in context
		if userID, ok := claims["sub"].(string); ok {
			c.Set("user_id", userID)
		}
		var userEmail string
		if email, ok := claims["email"].(string); ok {
			userEmail = email
			c.Set("user_email", email)
		}

		// Extract roles from JWT claims
		var rolesStr []string
		if roles, ok := claims["roles"].([]interface{}); ok {
			rolesStr = make([]string, len(roles))
			for i, role := range roles {
				if roleStr, ok := role.(string); ok {
					rolesStr[i] = roleStr
				}
			}
		}

		// If no roles in JWT but email matches admin list, grant admin+developer roles
		// This enables OIDC providers (like Janua) that don't include roles in tokens
		if len(rolesStr) == 0 && userEmail != "" && a.adminEmails[userEmail] {
			rolesStr = []string{"admin", "developer"}
			logrus.WithFields(logrus.Fields{
				"email": userEmail,
				"roles": rolesStr,
			}).Debug("Applied admin roles based on email mapping")
		} else if len(rolesStr) == 0 {
			// Default to developer role for authenticated users
			rolesStr = []string{"developer"}
		}

		c.Set("user_roles", rolesStr)

		// Check role requirements for this path
		if requiredRoles, exists := a.requiredRoles[path]; exists {
			userRoles := c.GetStringSlice("user_roles")
			if !hasRequiredRole(userRoles, requiredRoles) {
				logrus.WithFields(logrus.Fields{
					"path":           path,
					"method":         c.Request.Method,
					"user_id":        c.GetString("user_id"),
					"user_roles":     userRoles,
					"required_roles": requiredRoles,
				}).Warn("Insufficient permissions")

				c.JSON(http.StatusForbidden, gin.H{
					"error": "Insufficient permissions",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// hasRequiredRole checks if user has at least one of the required roles
func hasRequiredRole(userRoles, requiredRoles []string) bool {
	for _, required := range requiredRoles {
		for _, userRole := range userRoles {
			if userRole == required {
				return true
			}
		}
	}
	return false
}

// RequireAuth is a convenience middleware to enforce authentication
func RequireAuth(publicKey *rsa.PublicKey) gin.HandlerFunc {
	auth := NewAuthMiddleware(publicKey)
	return auth.Middleware()
}

// RequireRole creates a middleware that requires specific roles
// Note: This should be chained after RequireAuth to ensure user_roles are set
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := c.GetStringSlice("user_roles")
		if !hasRequiredRole(userRoles, roles) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "Insufficient permissions",
				"required_roles": roles,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
