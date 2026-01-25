package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Benchmark Tests for Security Middleware
// =============================================================================

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
