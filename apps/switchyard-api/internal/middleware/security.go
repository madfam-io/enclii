package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// SecurityMiddleware provides various security-related middleware functions
type SecurityMiddleware struct {
	rateLimiters map[string]*rate.Limiter
	mutex        sync.RWMutex
	config       *SecurityConfig
}

type SecurityConfig struct {
	// Rate limiting
	RateLimit       int           // requests per second
	RateBurst       int           // burst capacity
	RateWindowSize  time.Duration // time window for rate limiting
	
	// Security headers
	EnableHSTS      bool
	EnableCSP       bool
	EnableXSSProtection bool
	EnableNoSniff   bool
	EnableFrameOptions bool
	
	// Request validation
	MaxRequestSize  int64         // in bytes
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	
	// IP filtering
	AllowedIPs      []string      // CIDR blocks
	BlockedIPs      []string      // CIDR blocks
	TrustedProxies  []string      // for X-Forwarded-For header
	
	// CORS
	AllowedOrigins  []string
	AllowedMethods  []string
	AllowedHeaders  []string
	AllowCredentials bool
	MaxAge          int
}

func NewSecurityMiddleware(config *SecurityConfig) *SecurityMiddleware {
	if config == nil {
		config = DefaultSecurityConfig()
	}
	
	return &SecurityMiddleware{
		rateLimiters: make(map[string]*rate.Limiter),
		config:       config,
	}
}

// Rate limiting middleware
func (s *SecurityMiddleware) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := s.getClientIP(c)
		
		s.mutex.RLock()
		limiter, exists := s.rateLimiters[clientIP]
		s.mutex.RUnlock()
		
		if !exists {
			s.mutex.Lock()
			// Double-check after acquiring write lock
			if limiter, exists = s.rateLimiters[clientIP]; !exists {
				limiter = rate.NewLimiter(rate.Limit(s.config.RateLimit), s.config.RateBurst)
				s.rateLimiters[clientIP] = limiter
			}
			s.mutex.Unlock()
		}
		
		if !limiter.Allow() {
			logrus.WithFields(logrus.Fields{
				"client_ip": clientIP,
				"path":      c.Request.URL.Path,
				"method":    c.Request.Method,
			}).Warn("Rate limit exceeded")
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "60",
			})
			c.Header("Retry-After", "60")
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Security headers middleware
func (s *SecurityMiddleware) SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.EnableHSTS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		if s.config.EnableCSP {
			c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		}
		
		if s.config.EnableXSSProtection {
			c.Header("X-XSS-Protection", "1; mode=block")
		}
		
		if s.config.EnableNoSniff {
			c.Header("X-Content-Type-Options", "nosniff")
		}
		
		if s.config.EnableFrameOptions {
			c.Header("X-Frame-Options", "DENY")
		}
		
		// Additional security headers
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		c.Next()
	}
}

// Request size limiting middleware
func (s *SecurityMiddleware) RequestSizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > s.config.MaxRequestSize {
			logrus.WithFields(logrus.Fields{
				"client_ip":      s.getClientIP(c),
				"content_length": c.Request.ContentLength,
				"max_size":       s.config.MaxRequestSize,
			}).Warn("Request size too large")
			
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Request size too large",
				"max_size": s.config.MaxRequestSize,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// IP filtering middleware
func (s *SecurityMiddleware) IPFilteringMiddleware() gin.HandlerFunc {
	// Parse allowed and blocked IP ranges
	allowedNets := make([]*net.IPNet, 0, len(s.config.AllowedIPs))
	for _, cidr := range s.config.AllowedIPs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			logrus.Errorf("Invalid allowed IP CIDR: %s", cidr)
			continue
		}
		allowedNets = append(allowedNets, ipNet)
	}
	
	blockedNets := make([]*net.IPNet, 0, len(s.config.BlockedIPs))
	for _, cidr := range s.config.BlockedIPs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			logrus.Errorf("Invalid blocked IP CIDR: %s", cidr)
			continue
		}
		blockedNets = append(blockedNets, ipNet)
	}
	
	return func(c *gin.Context) {
		clientIP := net.ParseIP(s.getClientIP(c))
		if clientIP == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address"})
			c.Abort()
			return
		}
		
		// Check blocked IPs first
		for _, blockedNet := range blockedNets {
			if blockedNet.Contains(clientIP) {
				logrus.WithField("client_ip", clientIP.String()).Warn("Blocked IP attempted access")
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				c.Abort()
				return
			}
		}
		
		// Check allowed IPs (if configured)
		if len(allowedNets) > 0 {
			allowed := false
			for _, allowedNet := range allowedNets {
				if allowedNet.Contains(clientIP) {
					allowed = true
					break
				}
			}
			
			if !allowed {
				logrus.WithField("client_ip", clientIP.String()).Warn("Non-whitelisted IP attempted access")
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

// CORS middleware
func (s *SecurityMiddleware) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is allowed
		if len(s.config.AllowedOrigins) > 0 {
			allowed := false
			for _, allowedOrigin := range s.config.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}
			
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{"error": "Origin not allowed"})
				c.Abort()
				return
			}
			
			c.Header("Access-Control-Allow-Origin", origin)
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		
		c.Header("Access-Control-Allow-Methods", strings.Join(s.config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(s.config.AllowedHeaders, ", "))
		
		if s.config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		
		if s.config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(s.config.MaxAge))
		}
		
		// Handle preflight request
		if c.Request.Method == "OPTIONS" {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Request logging middleware
func (s *SecurityMiddleware) RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		duration := time.Since(start)
		
		// Log suspicious requests
		if c.Writer.Status() >= 400 || duration > 5*time.Second {
			logrus.WithFields(logrus.Fields{
				"client_ip":    s.getClientIP(c),
				"method":       c.Request.Method,
				"path":         c.Request.URL.Path,
				"status":       c.Writer.Status(),
				"duration":     duration.String(),
				"user_agent":   c.Request.UserAgent(),
				"content_length": c.Request.ContentLength,
			}).Info("HTTP request")
		}
	}
}

// Content type validation middleware
func (s *SecurityMiddleware) ContentTypeValidationMiddleware() gin.HandlerFunc {
	allowedTypes := map[string]bool{
		"application/json":                  true,
		"application/x-www-form-urlencoded": true,
		"multipart/form-data":               true,
		"text/plain":                        true,
	}
	
	return func(c *gin.Context) {
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" {
				// Parse content type (remove charset, etc.)
				parts := strings.Split(contentType, ";")
				mainType := strings.TrimSpace(parts[0])
				
				if !allowedTypes[mainType] {
					logrus.WithFields(logrus.Fields{
						"client_ip":    s.getClientIP(c),
						"content_type": contentType,
						"path":         c.Request.URL.Path,
					}).Warn("Invalid content type")
					
					c.JSON(http.StatusUnsupportedMediaType, gin.H{
						"error": "Unsupported content type",
						"allowed_types": []string{
							"application/json",
							"application/x-www-form-urlencoded",
							"multipart/form-data",
							"text/plain",
						},
					})
					c.Abort()
					return
				}
			}
		}
		
		c.Next()
	}
}

// User agent validation middleware (blocks known malicious user agents)
func (s *SecurityMiddleware) UserAgentValidationMiddleware() gin.HandlerFunc {
	suspiciousAgents := []string{
		"sqlmap",
		"nikto",
		"nmap",
		"masscan",
		"gobuster",
		"dirb",
		"dirbuster",
		"wpscan",
		"curl/7", // Be careful with this one - many legitimate tools use curl
	}
	
	return func(c *gin.Context) {
		userAgent := strings.ToLower(c.Request.UserAgent())
		
		for _, suspicious := range suspiciousAgents {
			if strings.Contains(userAgent, suspicious) {
				logrus.WithFields(logrus.Fields{
					"client_ip":  s.getClientIP(c),
					"user_agent": c.Request.UserAgent(),
					"path":       c.Request.URL.Path,
				}).Warn("Suspicious user agent detected")
				
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

// Get client IP address considering proxies
func (s *SecurityMiddleware) getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header (if using trusted proxies)
	if len(s.config.TrustedProxies) > 0 {
		forwarded := c.Request.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			// Get the first IP in the chain
			ips := strings.Split(forwarded, ",")
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if realIP := c.Request.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	
	// Fall back to remote address
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	
	return ip
}

// Cleanup routine to remove old rate limiters
func (s *SecurityMiddleware) CleanupRateLimiters() {
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			s.mutex.Lock()
			// In a real implementation, you'd track last access time
			// and remove limiters that haven't been used recently
			if len(s.rateLimiters) > 10000 { // Prevent memory leak
				s.rateLimiters = make(map[string]*rate.Limiter)
				logrus.Info("Cleared rate limiter cache")
			}
			s.mutex.Unlock()
		}
	}()
}

// Default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		RateLimit:       100,  // 100 requests per second
		RateBurst:       200,  // 200 burst capacity
		RateWindowSize:  time.Minute,
		EnableHSTS:      true,
		EnableCSP:       true,
		EnableXSSProtection: true,
		EnableNoSniff:   true,
		EnableFrameOptions: true,
		MaxRequestSize:  10 << 20, // 10MB
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		AllowedOrigins:  []string{"*"}, // Configure appropriately for production
		AllowedMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:  []string{"Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:          86400, // 24 hours
	}
}

// Security event logging
type SecurityEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"`
	ClientIP    string    `json:"client_ip"`
	UserAgent   string    `json:"user_agent"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	StatusCode  int       `json:"status_code"`
	Message     string    `json:"message"`
}

func LogSecurityEvent(eventType, clientIP, userAgent, path, method, message string, statusCode int) {
	event := SecurityEvent{
		Timestamp:  time.Now(),
		EventType:  eventType,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		Path:       path,
		Method:     method,
		StatusCode: statusCode,
		Message:    message,
	}
	
	logrus.WithFields(logrus.Fields{
		"security_event": event,
	}).Warn("Security event logged")
}