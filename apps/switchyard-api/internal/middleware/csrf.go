package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CSRFMiddleware provides Cross-Site Request Forgery protection
type CSRFMiddleware struct {
	tokens     map[string]*csrfToken
	mutex      sync.RWMutex
	cookieName string
	headerName string
	tokenTTL   time.Duration
}

type csrfToken struct {
	value     string
	createdAt time.Time
}

// NewCSRFMiddleware creates a new CSRF protection middleware
func NewCSRFMiddleware() *CSRFMiddleware {
	csrf := &CSRFMiddleware{
		tokens:     make(map[string]*csrfToken),
		cookieName: "csrf_token",
		headerName: "X-CSRF-Token",
		tokenTTL:   24 * time.Hour,
	}

	// Start cleanup routine
	go csrf.cleanupRoutine()

	return csrf
}

// Middleware returns a Gin middleware function for CSRF protection
func (c *CSRFMiddleware) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Skip CSRF check for safe methods
		if ctx.Request.Method == "GET" || ctx.Request.Method == "HEAD" || ctx.Request.Method == "OPTIONS" {
			// Generate and set CSRF token for safe methods
			token := c.generateToken()
			ctx.SetCookie(
				c.cookieName,
				token,
				int(c.tokenTTL.Seconds()),
				"/",
				"", // domain (empty = current domain)
				true, // secure (HTTPS only)
				true, // httpOnly
			)
			ctx.Header(c.headerName, token)
			ctx.Next()
			return
		}

		// For unsafe methods (POST, PUT, DELETE, PATCH), validate CSRF token
		cookieToken, err := ctx.Cookie(c.cookieName)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":   ctx.Request.URL.Path,
				"method": ctx.Request.Method,
				"ip":     ctx.ClientIP(),
			}).Warn("CSRF token missing from cookie")

			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing",
			})
			ctx.Abort()
			return
		}

		// Check token in header
		headerToken := ctx.GetHeader(c.headerName)
		if headerToken == "" {
			logrus.WithFields(logrus.Fields{
				"path":   ctx.Request.URL.Path,
				"method": ctx.Request.Method,
				"ip":     ctx.ClientIP(),
			}).Warn("CSRF token missing from header")

			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token required in header",
				"header": c.headerName,
			})
			ctx.Abort()
			return
		}

		// Validate tokens match
		if cookieToken != headerToken {
			logrus.WithFields(logrus.Fields{
				"path":   ctx.Request.URL.Path,
				"method": ctx.Request.Method,
				"ip":     ctx.ClientIP(),
			}).Warn("CSRF token mismatch")

			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token mismatch",
			})
			ctx.Abort()
			return
		}

		// Validate token exists and is not expired
		if !c.validateToken(headerToken) {
			logrus.WithFields(logrus.Fields{
				"path":   ctx.Request.URL.Path,
				"method": ctx.Request.Method,
				"ip":     ctx.ClientIP(),
			}).Warn("Invalid or expired CSRF token")

			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "Invalid or expired CSRF token",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

// generateToken creates a new CSRF token
func (c *CSRFMiddleware) generateToken() string {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		logrus.WithError(err).Error("Failed to generate CSRF token")
		// Fallback to timestamp-based token (less secure)
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))
	}

	token := base64.URLEncoding.EncodeToString(b)

	// Store token with timestamp
	c.mutex.Lock()
	c.tokens[token] = &csrfToken{
		value:     token,
		createdAt: time.Now(),
	}
	c.mutex.Unlock()

	return token
}

// validateToken checks if a token is valid and not expired
func (c *CSRFMiddleware) validateToken(token string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	t, exists := c.tokens[token]
	if !exists {
		return false
	}

	// Check if token is expired
	if time.Since(t.createdAt) > c.tokenTTL {
		return false
	}

	return true
}

// cleanupRoutine periodically removes expired tokens
func (c *CSRFMiddleware) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()

		// Remove expired tokens
		now := time.Now()
		for token, t := range c.tokens {
			if now.Sub(t.createdAt) > c.tokenTTL {
				delete(c.tokens, token)
			}
		}

		logrus.WithField("remaining_tokens", len(c.tokens)).Debug("Cleaned up expired CSRF tokens")

		c.mutex.Unlock()
	}
}
