package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from our known origins
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:4201",
			"https://app.enclii.dev",
			"https://app.enclii.io",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// LogStreamMessage represents a WebSocket message for log streaming
type LogStreamMessage struct {
	Type      string    `json:"type"` // "log", "error", "info", "connected", "disconnected"
	Pod       string    `json:"pod,omitempty"`
	Container string    `json:"container,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// StreamLogsWS handles WebSocket connections for real-time log streaming
// GET /v1/deployments/:id/logs/stream
func (h *Handler) StreamLogsWS(c *gin.Context) {
	ctx := c.Request.Context()
	deploymentID := c.Param("id")

	// Get deployment
	deployment, err := h.repos.Deployments.GetByID(ctx, deploymentID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get deployment for log streaming", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Get release to find service ID
	release, err := h.repos.Releases.GetByID(deployment.ReleaseID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get release for log streaming", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get release"})
		return
	}

	// Get service for namespace information
	service, err := h.repos.Services.GetByID(release.ServiceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service for log streaming", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get service"})
		return
	}

	// Get project for namespace
	project, err := h.repos.Projects.GetByID(ctx, service.ProjectID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get project for log streaming", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(ctx, deployment.EnvironmentID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get environment for log streaming", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get environment"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error(ctx, "Failed to upgrade to WebSocket", logging.Error("error", err))
		return
	}
	defer conn.Close()

	// Parse query parameters
	tailLines := int64(100)
	if tl := c.Query("lines"); tl != "" {
		if parsed, err := strconv.ParseInt(tl, 10, 64); err == nil && parsed > 0 {
			tailLines = parsed
		}
	}

	timestamps := c.Query("timestamps") == "true"

	// Build namespace and label selector
	namespace := fmt.Sprintf("enclii-%s-%s", project.Slug, env.Name)
	labelSelector := fmt.Sprintf("app=%s", service.Name)

	// Send connected message
	connMsg := LogStreamMessage{
		Type:      "connected",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Connected to logs for %s in %s", service.Name, namespace),
	}
	if err := conn.WriteJSON(connMsg); err != nil {
		h.logger.Error(ctx, "Failed to send connected message", logging.Error("error", err))
		return
	}

	// Create cancellable context for the stream
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle WebSocket close
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Create channels for log streaming
	logChan := make(chan k8s.LogLine, 100)
	errChan := make(chan error, 10)

	// Start streaming logs
	go h.k8sClient.StreamLogs(streamCtx, k8s.LogStreamOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
		TailLines:     tailLines,
		Follow:        true,
		Timestamps:    timestamps,
	}, logChan, errChan)

	// Process logs and send to WebSocket
	for {
		select {
		case <-streamCtx.Done():
			// Send disconnected message
			disconnMsg := LogStreamMessage{
				Type:      "disconnected",
				Timestamp: time.Now(),
				Message:   "Log stream disconnected",
			}
			conn.WriteJSON(disconnMsg)
			return

		case logLine, ok := <-logChan:
			if !ok {
				// Channel closed, stream ended
				return
			}
			msg := LogStreamMessage{
				Type:      "log",
				Pod:       logLine.Pod,
				Container: logLine.Container,
				Timestamp: logLine.Timestamp,
				Message:   logLine.Message,
			}
			if err := conn.WriteJSON(msg); err != nil {
				h.logger.Error(ctx, "Failed to write log message", logging.Error("error", err))
				return
			}

		case err, ok := <-errChan:
			if !ok {
				continue
			}
			errMsg := LogStreamMessage{
				Type:      "error",
				Timestamp: time.Now(),
				Message:   err.Error(),
			}
			if err := conn.WriteJSON(errMsg); err != nil {
				h.logger.Error(ctx, "Failed to write error message", logging.Error("error", err))
				return
			}
		}
	}
}

// StreamServiceLogsWS handles WebSocket connections for real-time log streaming by service ID
// GET /v1/services/:id/logs/stream
func (h *Handler) StreamServiceLogsWS(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")
	envName := c.DefaultQuery("env", "development")

	// Parse service ID
	serviceUUID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Get service
	service, err := h.repos.Services.GetByID(serviceUUID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service for log streaming", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Get project for namespace
	project, err := h.repos.Projects.GetByID(ctx, service.ProjectID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get project for log streaming", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error(ctx, "Failed to upgrade to WebSocket", logging.Error("error", err))
		return
	}
	defer conn.Close()

	// Parse query parameters
	tailLines := int64(100)
	if tl := c.Query("lines"); tl != "" {
		if parsed, err := strconv.ParseInt(tl, 10, 64); err == nil && parsed > 0 {
			tailLines = parsed
		}
	}

	timestamps := c.Query("timestamps") == "true"

	// Build namespace and label selector
	namespace := fmt.Sprintf("enclii-%s-%s", project.Slug, envName)
	labelSelector := fmt.Sprintf("app=%s", service.Name)

	// Send connected message
	connMsg := LogStreamMessage{
		Type:      "connected",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Connected to logs for %s in %s", service.Name, namespace),
	}
	if err := conn.WriteJSON(connMsg); err != nil {
		h.logger.Error(ctx, "Failed to send connected message", logging.Error("error", err))
		return
	}

	// Create cancellable context for the stream
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle WebSocket close
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Create channels for log streaming
	logChan := make(chan k8s.LogLine, 100)
	errChan := make(chan error, 10)

	// Start streaming logs
	go h.k8sClient.StreamLogs(streamCtx, k8s.LogStreamOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
		TailLines:     tailLines,
		Follow:        true,
		Timestamps:    timestamps,
	}, logChan, errChan)

	// Process logs and send to WebSocket
	for {
		select {
		case <-streamCtx.Done():
			disconnMsg := LogStreamMessage{
				Type:      "disconnected",
				Timestamp: time.Now(),
				Message:   "Log stream disconnected",
			}
			conn.WriteJSON(disconnMsg)
			return

		case logLine, ok := <-logChan:
			if !ok {
				return
			}
			msg := LogStreamMessage{
				Type:      "log",
				Pod:       logLine.Pod,
				Container: logLine.Container,
				Timestamp: logLine.Timestamp,
				Message:   logLine.Message,
			}
			if err := conn.WriteJSON(msg); err != nil {
				h.logger.Error(ctx, "Failed to write log message", logging.Error("error", err))
				return
			}

		case err, ok := <-errChan:
			if !ok {
				continue
			}
			errMsg := LogStreamMessage{
				Type:      "error",
				Timestamp: time.Now(),
				Message:   err.Error(),
			}
			if err := conn.WriteJSON(errMsg); err != nil {
				h.logger.Error(ctx, "Failed to write error message", logging.Error("error", err))
				return
			}
		}
	}
}

// GetLogsHistory returns historical logs (non-streaming)
// GET /v1/services/:id/logs/history
func (h *Handler) GetLogsHistory(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")
	envName := c.DefaultQuery("env", "development")

	// Parse service ID
	serviceUUID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Get service
	service, err := h.repos.Services.GetByID(serviceUUID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Get project
	project, err := h.repos.Projects.GetByID(ctx, service.ProjectID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get project", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	// Parse query parameters
	lines := 100
	if l := c.Query("lines"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 10000 {
			lines = parsed
		}
	}

	// Build namespace and label selector
	namespace := fmt.Sprintf("enclii-%s-%s", project.Slug, envName)
	labelSelector := fmt.Sprintf("app=%s", service.Name)

	// Get logs
	logs, err := h.k8sClient.GetLogs(ctx, namespace, labelSelector, lines, false)
	if err != nil {
		h.logger.Error(ctx, "Failed to get logs", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"service_id":   serviceID,
		"service_name": service.Name,
		"environment":  envName,
		"namespace":    namespace,
		"logs":         logs,
		"lines":        lines,
	})
}

// GetBuildLogs returns build logs from the builder service
// GET /v1/services/:id/builds/:build_id/logs
// NOTE: Requires Builds repository to be implemented
func (h *Handler) GetBuildLogs(c *gin.Context) {
	// TODO: Implement when Builds repository is available
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Build logs not yet implemented"})
}

// StreamBuildLogsWS handles WebSocket connections for real-time build log streaming
// GET /v1/services/:id/builds/:build_id/logs/stream
// NOTE: Requires Builds repository to be implemented
func (h *Handler) StreamBuildLogsWS(c *gin.Context) {
	// TODO: Implement when Builds repository is available
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Build log streaming not yet implemented"})
}

// LogSearchRequest represents a log search request
type LogSearchRequest struct {
	Query     string `json:"query" binding:"required"`
	StartTime string `json:"start_time"` // RFC3339 format
	EndTime   string `json:"end_time"`   // RFC3339 format
	Limit     int    `json:"limit"`
}

// SearchLogs searches through logs for a pattern
// POST /v1/services/:id/logs/search
func (h *Handler) SearchLogs(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")
	envName := c.DefaultQuery("env", "development")

	var req LogSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Parse service ID
	serviceUUID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Get service
	service, err := h.repos.Services.GetByID(serviceUUID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Get project
	project, err := h.repos.Projects.GetByID(ctx, service.ProjectID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get project", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	// Set default limit
	limit := 1000
	if req.Limit > 0 && req.Limit <= 10000 {
		limit = req.Limit
	}

	// Build namespace and label selector
	namespace := fmt.Sprintf("enclii-%s-%s", project.Slug, envName)
	labelSelector := fmt.Sprintf("app=%s", service.Name)

	// Get logs
	logs, err := h.k8sClient.GetLogs(ctx, namespace, labelSelector, limit, false)
	if err != nil {
		h.logger.Error(ctx, "Failed to get logs", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get logs"})
		return
	}

	// Simple grep-style search (in production, use a log aggregation service)
	var matchingLines []string
	for _, line := range splitLines(logs) {
		if containsIgnoreCase(line, req.Query) {
			matchingLines = append(matchingLines, line)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"service_id":   serviceID,
		"service_name": service.Name,
		"environment":  envName,
		"query":        req.Query,
		"matches":      len(matchingLines),
		"logs":         matchingLines,
	})
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}
