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
		"version": "0.1.0",
	})
}

// LivenessProbe returns a simple health check for Kubernetes liveness probe
// This checks if the process is running - it doesn't check dependencies
func (h *Handler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// ReadinessProbe checks if the service is ready to accept traffic
// This checks critical dependencies (database) before returning healthy
func (h *Handler) ReadinessProbe(c *gin.Context) {
	// Check database connectivity via a simple query
	ctx := c.Request.Context()
	if err := h.repos.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unavailable",
			"message": "database not ready",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
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
