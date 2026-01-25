package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ComponentHealth represents the health status of a component
type ComponentHealth struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HealthResponse represents the detailed health response
type HealthResponse struct {
	Status     string                     `json:"status"`
	Service    string                     `json:"service"`
	Version    string                     `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
}

// Health returns the health status of the API with component details
func (h *Handler) Health(c *gin.Context) {
	ctx := c.Request.Context()

	response := HealthResponse{
		Status:     "healthy",
		Service:    "switchyard-api",
		Version:    "0.1.0",
		Components: make(map[string]ComponentHealth),
	}

	// Check database health
	dbHealth := h.checkDatabaseHealth(ctx)
	response.Components["database"] = dbHealth
	if dbHealth.Status != "healthy" {
		response.Status = "degraded"
	}

	// Check cache health
	cacheHealth := h.checkCacheHealth(ctx)
	response.Components["cache"] = cacheHealth
	if cacheHealth.Status != "healthy" && response.Status == "healthy" {
		response.Status = "degraded"
	}

	// Check Kubernetes connectivity
	k8sHealth := h.checkK8sHealth(ctx)
	response.Components["kubernetes"] = k8sHealth
	if k8sHealth.Status != "healthy" && response.Status == "healthy" {
		response.Status = "degraded"
	}

	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusOK // Still return 200, but indicate degraded in body
	}

	c.JSON(statusCode, response)
}

// checkDatabaseHealth checks database connectivity with timeout
func (h *Handler) checkDatabaseHealth(ctx context.Context) (result ComponentHealth) {
	// Recover from any panics
	defer func() {
		if r := recover(); r != nil {
			result = ComponentHealth{
				Status: "unhealthy",
				Error:  fmt.Sprintf("database health check panicked: %v", r),
			}
		}
	}()

	// Defensive nil check - prevent panic if repos not initialized
	if h.repos == nil {
		return ComponentHealth{
			Status: "unhealthy",
			Error:  "database connection not initialized",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()
	err := h.repos.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return ComponentHealth{
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     err.Error(),
		}
	}

	return ComponentHealth{
		Status:    "healthy",
		LatencyMs: latency,
	}
}

// checkCacheHealth checks Redis cache connectivity with timeout
func (h *Handler) checkCacheHealth(ctx context.Context) (result ComponentHealth) {
	// Recover from any panics
	defer func() {
		if r := recover(); r != nil {
			result = ComponentHealth{
				Status: "unhealthy",
				Error:  fmt.Sprintf("cache health check panicked: %v", r),
			}
		}
	}()

	if h.cache == nil {
		return ComponentHealth{
			Status: "disabled",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	err := h.cache.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return ComponentHealth{
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     err.Error(),
		}
	}

	return ComponentHealth{
		Status:    "healthy",
		LatencyMs: latency,
	}
}

// checkK8sHealth checks Kubernetes API connectivity
// This function includes panic recovery to handle unexpected errors from the K8s client
func (h *Handler) checkK8sHealth(ctx context.Context) (result ComponentHealth) {
	// Recover from any panics in the K8s client calls
	defer func() {
		if r := recover(); r != nil {
			result = ComponentHealth{
				Status: "unhealthy",
				Error:  fmt.Sprintf("kubernetes health check panicked: %v", r),
			}
		}
	}()

	if h.k8sClient == nil {
		return ComponentHealth{
			Status: "disabled",
			Error:  "kubernetes client not configured",
		}
	}

	if !h.k8sClient.IsValid() {
		return ComponentHealth{
			Status: "unhealthy",
			Error:  "kubernetes client not properly initialized",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()
	// Use MetricsServerAvailable as a proxy for K8s API health
	// It makes an API call to check the metrics server
	available := h.k8sClient.MetricsServerAvailable(ctx)
	latency := time.Since(start).Milliseconds()

	// If metrics server is available, K8s API is definitely reachable
	if available {
		return ComponentHealth{
			Status:    "healthy",
			LatencyMs: latency,
		}
	}

	// If not, K8s API might still be reachable but metrics-server isn't installed
	// Try a simple pod list to verify K8s connectivity
	_, err := h.k8sClient.ListPods(ctx, "default", "")
	if err != nil {
		return ComponentHealth{
			Status:    "unhealthy",
			LatencyMs: latency,
			Error:     "kubernetes API unreachable: " + err.Error(),
		}
	}

	// K8s API is reachable, just metrics-server is missing
	return ComponentHealth{
		Status:    "healthy",
		LatencyMs: latency,
	}
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
	// Defensive nil check - return 503 if dependencies not initialized
	if h.repos == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   "database connection not initialized",
			"message": "service initialization incomplete",
		})
		return
	}

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
	// Defensive nil check
	if h.builder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unavailable",
			"error":   "build service not initialized",
			"message": "Build pipeline is not configured",
		})
		return
	}

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
