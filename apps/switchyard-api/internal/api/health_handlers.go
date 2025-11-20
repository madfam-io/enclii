package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health returns the health status of the API
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "switchyard-api",
		"version": "1.0.0",
	})
}

// GetBuildStatus returns the status of the build pipeline and available tools
func (h *Handler) GetBuildStatus(c *gin.Context) {
	status := h.builder.GetBuildStatus()

	// Determine overall health
	toolsAvailable, _ := status["tools_available"].(bool)
	overallStatus := "healthy"
	if !toolsAvailable {
		overallStatus = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         overallStatus,
		"build_pipeline": status,
		"message": func() string {
			if toolsAvailable {
				return "Build pipeline is ready"
			}
			return "Build tools not available. Run: ./scripts/setup-build-tools.sh"
		}(),
	})
}
