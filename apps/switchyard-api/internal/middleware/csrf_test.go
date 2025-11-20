package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware_GetRequest(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test router
	router := gin.New()
	csrf := NewCSRFMiddleware()
	router.Use(csrf.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Test GET request (should succeed and set CSRF token)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-CSRF-Token"))

	// Check cookie is set
	cookies := w.Result().Cookies()
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			found = true
			assert.True(t, cookie.HttpOnly)
			assert.True(t, cookie.Secure)
			break
		}
	}
	assert.True(t, found, "CSRF cookie should be set")
}

func TestCSRFMiddleware_PostWithoutToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test router
	router := gin.New()
	csrf := NewCSRFMiddleware()
	router.Use(csrf.Middleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Test POST request without CSRF token
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "CSRF token missing")
}

func TestCSRFMiddleware_PostWithValidToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test router
	router := gin.New()
	csrf := NewCSRFMiddleware()
	router.Use(csrf.Middleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// First, get a CSRF token
	getReq := httptest.NewRequest("GET", "/test-get", nil)
	getW := httptest.NewRecorder()

	router.GET("/test-get", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.ServeHTTP(getW, getReq)

	token := getW.Header().Get("X-CSRF-Token")
	assert.NotEmpty(t, token)

	// Get cookie value
	var cookieValue string
	for _, cookie := range getW.Result().Cookies() {
		if cookie.Name == "csrf_token" {
			cookieValue = cookie.Value
			break
		}
	}

	// Now POST with the token
	postReq := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	postReq.Header.Set("X-CSRF-Token", token)
	postReq.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: cookieValue,
	})
	postW := httptest.NewRecorder()

	router.ServeHTTP(postW, postReq)

	assert.Equal(t, http.StatusOK, postW.Code)
}

func TestCSRFMiddleware_PostWithMismatchedToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test router
	router := gin.New()
	csrf := NewCSRFMiddleware()
	router.Use(csrf.Middleware())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// POST with mismatched tokens
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
	req.Header.Set("X-CSRF-Token", "token-in-header")
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: "different-token-in-cookie",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "CSRF token mismatch")
}
