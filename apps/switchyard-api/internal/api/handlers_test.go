package api

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Unit tests for pure functions and simple validation
// Handler tests that require full dependency setup are in handlers_integration_test.go

func TestHandlerSetup(t *testing.T) {
	// Verify test mode is set correctly
	if gin.Mode() != gin.TestMode {
		t.Errorf("Expected gin test mode, got %s", gin.Mode())
	}
}
