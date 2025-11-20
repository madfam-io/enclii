package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	jwtSecret     []byte
	publicPaths   map[string]bool
	requiredRoles map[string][]string // path -> required roles
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtSecret []byte) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret:     jwtSecret,
		publicPaths:   make(map[string]bool),
		requiredRoles: make(map[string][]string),
	}
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
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.jwtSecret, nil
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
		if email, ok := claims["email"].(string); ok {
			c.Set("user_email", email)
		}
		if roles, ok := claims["roles"].([]interface{}); ok {
			rolesStr := make([]string, len(roles))
			for i, role := range roles {
				if roleStr, ok := role.(string); ok {
					rolesStr[i] = roleStr
				}
			}
			c.Set("user_roles", rolesStr)
		}

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
func RequireAuth(jwtSecret []byte) gin.HandlerFunc {
	auth := NewAuthMiddleware(jwtSecret)
	return auth.Middleware()
}

// RequireRole creates a middleware that requires specific roles
func RequireRole(jwtSecret []byte, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := c.GetStringSlice("user_roles")
		if !hasRequiredRole(userRoles, roles) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
				"required_roles": roles,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
