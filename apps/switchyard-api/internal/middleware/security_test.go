package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
}

func TestNewSecurityMiddleware(t *testing.T) {
	tests := []struct {
		name   string
		config *SecurityConfig
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name: "with custom config",
			config: &SecurityConfig{
				RateLimit:      50,
				RateBurst:      100,
				MaxRequestSize: 5 << 20, // 5MB
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewSecurityMiddleware(tt.config)

			if middleware == nil {
				t.Error("NewSecurityMiddleware() returned nil")
				return
			}

			if middleware.rateLimiters == nil {
				t.Error("rateLimiters map is nil")
			}

			if middleware.config == nil {
				t.Error("config is nil")
			}

			// If nil config was passed, should use defaults
			if tt.config == nil {
				if middleware.config.RateLimit != 100 {
					t.Errorf("Default RateLimit = %d, want 100", middleware.config.RateLimit)
				}
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := &SecurityConfig{
		RateLimit: 2, // 2 requests per second
		RateBurst: 2, // 2 burst capacity
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First 2 requests should succeed (within burst limit)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Rate limit request: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After header = %s, want 60", w.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	config := &SecurityConfig{
		RateLimit: 1,
		RateBurst: 1,
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Requests from different IPs should not share rate limits
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12346", "192.168.1.3:12347"}

	for _, ip := range ips {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request from %s: got status %d, want %d", ip, w.Code, http.StatusOK)
		}
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	config := &SecurityConfig{
		EnableHSTS:          true,
		EnableCSP:           true,
		EnableXSSProtection: true,
		EnableNoSniff:       true,
		EnableFrameOptions:  true,
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'",
		"X-XSS-Protection":          "1; mode=block",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
	}

	for header, expected := range expectedHeaders {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("Header %s = %s, want %s", header, got, expected)
		}
	}
}

func TestSecurityHeadersMiddleware_Disabled(t *testing.T) {
	config := &SecurityConfig{
		EnableHSTS:          false,
		EnableCSP:           false,
		EnableXSSProtection: false,
		EnableNoSniff:       false,
		EnableFrameOptions:  false,
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// Optional security headers should not be present
	optionalHeaders := []string{
		"Strict-Transport-Security",
		"Content-Security-Policy",
		"X-XSS-Protection",
		"X-Content-Type-Options",
		"X-Frame-Options",
	}

	for _, header := range optionalHeaders {
		if w.Header().Get(header) != "" {
			t.Errorf("Header %s should not be set when disabled", header)
		}
	}

	// These headers should always be present
	if w.Header().Get("Referrer-Policy") == "" {
		t.Error("Referrer-Policy header missing")
	}
}

func TestRequestSizeLimitMiddleware(t *testing.T) {
	config := &SecurityConfig{
		MaxRequestSize: 100, // 100 bytes
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.RequestSizeLimitMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		contentLength  int64
		expectedStatus int
	}{
		{"small request", 50, http.StatusOK},
		{"exactly at limit", 100, http.StatusOK},
		{"too large", 101, http.StatusRequestEntityTooLarge},
		{"much too large", 1000, http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			body := bytes.NewReader(make([]byte, tt.contentLength))
			req, _ := http.NewRequest("POST", "/test", body)
			req.ContentLength = tt.contentLength
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestIPFilteringMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		allowedIPs     []string
		blockedIPs     []string
		clientIP       string
		expectedStatus int
	}{
		{
			name:           "no filtering",
			allowedIPs:     nil,
			blockedIPs:     nil,
			clientIP:       "192.168.1.1:12345",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "allowed IP",
			allowedIPs:     []string{"192.168.1.0/24"},
			blockedIPs:     nil,
			clientIP:       "192.168.1.100:12345",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not in allowed list",
			allowedIPs:     []string{"192.168.1.0/24"},
			blockedIPs:     nil,
			clientIP:       "10.0.0.1:12345",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "blocked IP",
			allowedIPs:     nil,
			blockedIPs:     []string{"10.0.0.0/8"},
			clientIP:       "10.0.0.1:12345",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not blocked",
			allowedIPs:     nil,
			blockedIPs:     []string{"10.0.0.0/8"},
			clientIP:       "192.168.1.1:12345",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityConfig{
				AllowedIPs: tt.allowedIPs,
				BlockedIPs: tt.blockedIPs,
			}
			middleware := NewSecurityMiddleware(config)

			router := gin.New()
			router.Use(middleware.IPFilteringMiddleware())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.clientIP
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name              string
		allowedOrigins    []string
		requestOrigin     string
		method            string
		expectedStatus    int
		expectAllowOrigin bool
	}{
		{
			name:              "wildcard origin",
			allowedOrigins:    []string{"*"},
			requestOrigin:     "https://example.com",
			method:            "GET",
			expectedStatus:    http.StatusOK,
			expectAllowOrigin: true,
		},
		{
			name:              "allowed origin",
			allowedOrigins:    []string{"https://example.com"},
			requestOrigin:     "https://example.com",
			method:            "GET",
			expectedStatus:    http.StatusOK,
			expectAllowOrigin: true,
		},
		{
			name:              "not allowed origin",
			allowedOrigins:    []string{"https://example.com"},
			requestOrigin:     "https://evil.com",
			method:            "GET",
			expectedStatus:    http.StatusForbidden,
			expectAllowOrigin: false,
		},
		{
			name:              "preflight request",
			allowedOrigins:    []string{"https://example.com"},
			requestOrigin:     "https://example.com",
			method:            "OPTIONS",
			expectedStatus:    http.StatusNoContent,
			expectAllowOrigin: true,
		},
		{
			name:              "no allowed origins configured",
			allowedOrigins:    nil,
			requestOrigin:     "https://example.com",
			method:            "GET",
			expectedStatus:    http.StatusOK,
			expectAllowOrigin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityConfig{
				AllowedOrigins:   tt.allowedOrigins,
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders:   []string{"Authorization", "Content-Type"},
				AllowCredentials: true,
				MaxAge:           3600,
			}
			middleware := NewSecurityMiddleware(config)

			router := gin.New()
			router.Use(middleware.CORSMiddleware())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.OPTIONS("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.expectAllowOrigin {
				allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
				if allowOrigin == "" {
					t.Error("Access-Control-Allow-Origin header missing")
				}
			}

			if tt.expectAllowOrigin && w.Code == http.StatusOK {
				if w.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Error("Access-Control-Allow-Methods header missing")
				}
				if w.Header().Get("Access-Control-Allow-Headers") == "" {
					t.Error("Access-Control-Allow-Headers header missing")
				}
				if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Error("Access-Control-Allow-Credentials should be true")
				}
				if w.Header().Get("Access-Control-Max-Age") != "3600" {
					t.Errorf("Access-Control-Max-Age = %s, want 3600", w.Header().Get("Access-Control-Max-Age"))
				}
			}
		})
	}
}

func TestRequestLoggingMiddleware(t *testing.T) {
	middleware := NewSecurityMiddleware(nil)

	router := gin.New()
	router.Use(middleware.RequestLoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
	})

	// Normal request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	// Error request (should trigger logging)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/error", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestContentTypeValidationMiddleware(t *testing.T) {
	middleware := NewSecurityMiddleware(nil)

	router := gin.New()
	router.Use(middleware.ContentTypeValidationMiddleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
	}{
		{"GET request no validation", "GET", "", http.StatusOK},
		{"POST with JSON", "POST", "application/json", http.StatusOK},
		{"POST with form data", "POST", "application/x-www-form-urlencoded", http.StatusOK},
		{"POST with multipart", "POST", "multipart/form-data", http.StatusOK},
		{"POST with text", "POST", "text/plain", http.StatusOK},
		{"POST with JSON and charset", "POST", "application/json; charset=utf-8", http.StatusOK},
		{"POST with invalid type", "POST", "application/xml", http.StatusUnsupportedMediaType},
		{"POST with no content type", "POST", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestUserAgentValidationMiddleware(t *testing.T) {
	middleware := NewSecurityMiddleware(nil)

	router := gin.New()
	router.Use(middleware.UserAgentValidationMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		userAgent      string
		expectedStatus int
	}{
		{"normal browser", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", http.StatusOK},
		{"sqlmap", "sqlmap/1.0", http.StatusForbidden},
		{"nikto", "Nikto/2.1.6", http.StatusForbidden},
		{"nmap", "Nmap Scripting Engine", http.StatusForbidden},
		{"empty user agent", "", http.StatusOK},
		{"legitimate curl", "curl/8.0.1", http.StatusOK},
		{"suspicious curl", "curl/7.68.0", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("User-Agent", tt.userAgent)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
		{
			name:           "X-Forwarded-For with trusted proxy",
			trustedProxies: []string{"10.0.0.0/8"},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "192.168.1.100, 10.0.0.2",
			expectedIP:     "192.168.1.100",
		},
		{
			name:          "X-Forwarded-For without trusted proxy",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "192.168.1.100",
			xRealIP:       "192.168.1.200",
			expectedIP:    "192.168.1.200",
		},
		{
			name:       "no port in remote addr",
			remoteAddr: "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityConfig{
				TrustedProxies: tt.trustedProxies,
			}
			middleware := NewSecurityMiddleware(config)

			router := gin.New()
			var capturedIP string
			router.GET("/test", func(c *gin.Context) {
				capturedIP = middleware.getClientIP(c)
				c.String(http.StatusOK, "ok")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			router.ServeHTTP(w, req)

			if capturedIP != tt.expectedIP {
				t.Errorf("getClientIP() = %s, want %s", capturedIP, tt.expectedIP)
			}
		})
	}
}

// TODO: TestCleanupRateLimiters is disabled because CleanupRateLimiters method is not implemented
// func TestCleanupRateLimiters(t *testing.T) {
// 	middleware := NewSecurityMiddleware(nil)
//
// 	// Add many rate limiters
// 	for i := 0; i < 100; i++ {
// 		key := string(rune(i))
// 		middleware.rateLimiters[key] = nil
// 	}
//
// 	if len(middleware.rateLimiters) != 100 {
// 		t.Errorf("Initial limiters count = %d, want 100", len(middleware.rateLimiters))
// 	}
//
// 	// Start cleanup
// 	middleware.CleanupRateLimiters()
//
// 	// Give the goroutine a moment to start
// 	time.Sleep(10 * time.Millisecond)
//
// 	// The cleanup should run periodically, but we can't easily test the automatic cleanup
// 	// Just verify the goroutine started without panicking
// }

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if config == nil {
		t.Fatal("DefaultSecurityConfig() returned nil")
	}

	// Verify default values
	if config.RateLimit != 100 {
		t.Errorf("RateLimit = %d, want 100", config.RateLimit)
	}

	if config.RateBurst != 200 {
		t.Errorf("RateBurst = %d, want 200", config.RateBurst)
	}

	if !config.EnableHSTS {
		t.Error("EnableHSTS should be true")
	}

	if !config.EnableCSP {
		t.Error("EnableCSP should be true")
	}

	if config.MaxRequestSize != 10<<20 {
		t.Errorf("MaxRequestSize = %d, want %d", config.MaxRequestSize, 10<<20)
	}

	if len(config.AllowedOrigins) == 0 {
		t.Error("AllowedOrigins should not be empty")
	}

	if len(config.AllowedMethods) == 0 {
		t.Error("AllowedMethods should not be empty")
	}

	if !config.AllowCredentials {
		t.Error("AllowCredentials should be true")
	}
}

func TestGetAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "no environment variable",
			envValue: "",
			expected: []string{
				"http://localhost:3000",
				"http://localhost:8080",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:8080",
			},
		},
		{
			name:     "single origin",
			envValue: "https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "multiple origins",
			envValue: "https://example.com, https://app.example.com",
			expected: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:     "origins with whitespace",
			envValue: "  https://example.com  ,  https://app.example.com  ",
			expected: []string{"https://example.com", "https://app.example.com"},
		},
		{
			name:     "empty strings filtered",
			envValue: "https://example.com,,https://app.example.com",
			expected: []string{"https://example.com", "https://app.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("ENCLII_ALLOWED_ORIGINS", tt.envValue)
				defer os.Unsetenv("ENCLII_ALLOWED_ORIGINS")
			} else {
				os.Unsetenv("ENCLII_ALLOWED_ORIGINS")
			}

			got := getAllowedOrigins()

			if len(got) != len(tt.expected) {
				t.Errorf("getAllowedOrigins() returned %d origins, want %d", len(got), len(tt.expected))
				return
			}

			for i, origin := range got {
				if origin != tt.expected[i] {
					t.Errorf("getAllowedOrigins()[%d] = %s, want %s", i, origin, tt.expected[i])
				}
			}
		})
	}
}

func TestLogSecurityEvent(t *testing.T) {
	// Test that LogSecurityEvent doesn't panic
	LogSecurityEvent(
		"test_event",
		"192.168.1.1",
		"Mozilla/5.0",
		"/test/path",
		"GET",
		"test message",
		200,
	)

	// If we got here without panicking, the test passes
}

func TestSecurityMiddleware_Concurrency(t *testing.T) {
	config := &SecurityConfig{
		RateLimit: 100,
		RateBurst: 200,
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test concurrent requests from different IPs
	var wg sync.WaitGroup
	numGoroutines := 50
	requestsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/test", nil)
				// Use different IP for each goroutine
				req.RemoteAddr = strings.Replace("192.168.1.X:12345", "X", string(rune(id)), 1)
				router.ServeHTTP(w, req)
			}
		}(i)
	}

	wg.Wait()

	// Verify rate limiters were created without race conditions
	middleware.mutex.RLock()
	numLimiters := len(middleware.rateLimiters)
	middleware.mutex.RUnlock()

	if numLimiters == 0 {
		t.Error("No rate limiters were created")
	}
}

func TestIPFilteringMiddleware_InvalidCIDR(t *testing.T) {
	config := &SecurityConfig{
		AllowedIPs: []string{"invalid-cidr"},
		BlockedIPs: []string{"also-invalid"},
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.IPFilteringMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Should still work (invalid CIDRs are logged and skipped)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	config := &SecurityConfig{
		RateLimit: 1000,
		RateBurst: 2000,
	}
	middleware := NewSecurityMiddleware(config)

	router := gin.New()
	router.Use(middleware.RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

func BenchmarkSecurityHeadersMiddleware(b *testing.B) {
	middleware := NewSecurityMiddleware(nil)

	router := gin.New()
	router.Use(middleware.SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}
