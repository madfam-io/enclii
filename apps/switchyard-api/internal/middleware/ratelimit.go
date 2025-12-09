package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	// Requests per window
	Limit int
	// Time window duration
	Window time.Duration
	// Key function to identify clients (returns key string)
	KeyFunc func(*gin.Context) string
	// Whether to skip successful requests (only count failures)
	SkipSuccessful bool
}

// visitor tracks request counts for a single client
type visitor struct {
	count     int
	windowEnd time.Time
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	mu       sync.RWMutex
	visitors map[string]*visitor
	config   RateLimitConfig
	// Cleanup goroutine control
	stopCleanup chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		visitors:    make(map[string]*visitor),
		config:      config,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes expired visitors
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes expired visitors
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, v := range rl.visitors {
		if now.After(v.windowEnd) {
			delete(rl.visitors, key)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Allow checks if a request should be allowed
// Returns: allowed (bool), remaining requests (int), reset time (time.Time)
func (rl *RateLimiter) Allow(key string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[key]

	// Create new visitor or reset if window expired
	if !exists || now.After(v.windowEnd) {
		rl.visitors[key] = &visitor{
			count:     1,
			windowEnd: now.Add(rl.config.Window),
		}
		return true, rl.config.Limit - 1, now.Add(rl.config.Window)
	}

	// Check if limit exceeded
	if v.count >= rl.config.Limit {
		return false, 0, v.windowEnd
	}

	// Increment count
	v.count++
	return true, rl.config.Limit - v.count, v.windowEnd
}

// Middleware returns a Gin middleware function for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client key
		key := rl.config.KeyFunc(c)

		// Check rate limit
		allowed, remaining, resetTime := rl.Allow(key)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.config.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if !allowed {
			// Calculate retry-after in seconds
			retryAfter := int(time.Until(resetTime).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			c.Header("Retry-After", strconv.Itoa(retryAfter))

			logrus.WithFields(logrus.Fields{
				"key":         key,
				"limit":       rl.config.Limit,
				"window":      rl.config.Window.String(),
				"retry_after": retryAfter,
				"path":        c.Request.URL.Path,
				"method":      c.Request.Method,
			}).Warn("Rate limit exceeded")

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"retry_after": retryAfter,
				"message":     "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// =============================================================================
// Pre-configured rate limiters for common use cases
// =============================================================================

// IPKeyFunc returns client IP as the rate limit key
func IPKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// UserKeyFunc returns user ID as the rate limit key (requires auth middleware)
func UserKeyFunc(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case string:
			return "user:" + v
		default:
			return "user:" + c.ClientIP() // Fallback to IP
		}
	}
	return "anon:" + c.ClientIP()
}

// NewAuthRateLimiter creates a rate limiter for authentication endpoints
// Default: 10 requests per minute per IP
func NewAuthRateLimiter() *RateLimiter {
	return NewRateLimiter(RateLimitConfig{
		Limit:   10,
		Window:  time.Minute,
		KeyFunc: IPKeyFunc,
	})
}

// NewAPIRateLimiter creates a rate limiter for API endpoints
// Default: 100 requests per minute per user
func NewAPIRateLimiter() *RateLimiter {
	return NewRateLimiter(RateLimitConfig{
		Limit:   100,
		Window:  time.Minute,
		KeyFunc: UserKeyFunc,
	})
}

// NewStrictAuthRateLimiter creates a stricter rate limiter for sensitive auth endpoints
// Default: 5 requests per minute per IP (for login failures, password reset, etc.)
func NewStrictAuthRateLimiter() *RateLimiter {
	return NewRateLimiter(RateLimitConfig{
		Limit:   5,
		Window:  time.Minute,
		KeyFunc: IPKeyFunc,
	})
}

// =============================================================================
// Gin middleware factory functions
// =============================================================================

// RateLimit returns a rate limiting middleware with custom configuration
func RateLimit(limit int, window time.Duration, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	rl := NewRateLimiter(RateLimitConfig{
		Limit:   limit,
		Window:  window,
		KeyFunc: keyFunc,
	})
	return rl.Middleware()
}

// RateLimitByIP returns a rate limiting middleware keyed by client IP
func RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	return RateLimit(limit, window, IPKeyFunc)
}

// RateLimitByUser returns a rate limiting middleware keyed by user ID
func RateLimitByUser(limit int, window time.Duration) gin.HandlerFunc {
	return RateLimit(limit, window, UserKeyFunc)
}

// AuthRateLimit returns the default auth rate limiting middleware
// 10 requests/minute per IP
func AuthRateLimit() gin.HandlerFunc {
	return NewAuthRateLimiter().Middleware()
}

// APIRateLimit returns the default API rate limiting middleware
// 100 requests/minute per user
func APIRateLimit() gin.HandlerFunc {
	return NewAPIRateLimiter().Middleware()
}

// StrictAuthRateLimit returns strict rate limiting for sensitive endpoints
// 5 requests/minute per IP
func StrictAuthRateLimit() gin.HandlerFunc {
	return NewStrictAuthRateLimiter().Middleware()
}
