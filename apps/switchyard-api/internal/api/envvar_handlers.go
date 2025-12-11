package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CreateEnvVarRequest represents a request to create an environment variable
type CreateEnvVarRequest struct {
	Key           string  `json:"key" binding:"required"`
	Value         string  `json:"value" binding:"required"`
	EnvironmentID *string `json:"environment_id,omitempty"` // NULL = all environments
	IsSecret      bool    `json:"is_secret"`
}

// UpdateEnvVarRequest represents a request to update an environment variable
type UpdateEnvVarRequest struct {
	Key      *string `json:"key,omitempty"`
	Value    *string `json:"value,omitempty"`
	IsSecret *bool   `json:"is_secret,omitempty"`
}

// BulkEnvVarRequest represents a single env var in a bulk request
type BulkEnvVarRequest struct {
	Key      string `json:"key" binding:"required"`
	Value    string `json:"value" binding:"required"`
	IsSecret bool   `json:"is_secret"`
}

// BulkUpsertEnvVarsRequest represents a request to bulk upsert env vars
type BulkUpsertEnvVarsRequest struct {
	EnvironmentID *string             `json:"environment_id,omitempty"`
	Variables     []BulkEnvVarRequest `json:"variables" binding:"required,min=1"`
}

// ListEnvVars returns all environment variables for a service
// GET /v1/services/:id/env-vars
func (h *Handler) ListEnvVars(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")

	// Parse service ID
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Verify service exists
	_, err = h.repos.Services.GetByID(svcID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Parse optional environment filter
	var envID *uuid.UUID
	if envIDStr := c.Query("environment_id"); envIDStr != "" {
		parsed, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
			return
		}
		envID = &parsed
	}

	// Get env vars
	envVars, err := h.repos.EnvVars.List(ctx, svcID, envID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list env vars", logging.String("service_id", serviceID), logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environment variables"})
		return
	}

	// Convert to response format (mask secrets)
	response := make([]types.EnvironmentVariableResponse, len(envVars))
	for i, ev := range envVars {
		response[i] = toEnvVarResponse(ev)
	}

	c.JSON(http.StatusOK, gin.H{"environment_variables": response})
}

// CreateEnvVar creates a new environment variable
// POST /v1/services/:id/env-vars
func (h *Handler) CreateEnvVar(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")

	// Parse service ID
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Verify service exists
	_, err = h.repos.Services.GetByID(svcID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	var req CreateEnvVarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate key format
	if !isValidEnvVarKey(req.Key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment variable key. Must start with letter or underscore, and contain only alphanumeric characters and underscores."})
		return
	}

	// Parse optional environment ID
	var envID *uuid.UUID
	if req.EnvironmentID != nil && *req.EnvironmentID != "" {
		parsed, err := uuid.Parse(*req.EnvironmentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
			return
		}
		envID = &parsed

		// Verify environment exists
		_, err = h.repos.Environments.GetByID(ctx, parsed)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
			return
		}
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")

	var createdBy *uuid.UUID
	if userID != "" {
		parsed, _ := uuid.Parse(userID)
		createdBy = &parsed
	}

	// Create env var
	ev := &types.EnvironmentVariable{
		ServiceID:      svcID,
		EnvironmentID:  envID,
		Key:            req.Key,
		Value:          req.Value,
		IsSecret:       req.IsSecret,
		CreatedBy:      createdBy,
		CreatedByEmail: userEmail,
	}

	if err := h.repos.EnvVars.Create(ctx, ev); err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"error": "Environment variable with this key already exists"})
			return
		}
		h.logger.Error(ctx, "Failed to create env var", logging.String("service_id", serviceID), logging.String("key", req.Key), logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment variable"})
		return
	}

	// Log audit
	h.repos.EnvVars.LogAudit(ctx, &types.EnvVarAuditLog{
		EnvVarID:      ev.ID,
		ServiceID:     svcID,
		EnvironmentID: envID,
		Action:        "created",
		Key:           ev.Key,
		NewValueHash:  hashValue(ev.Value),
		ActorID:       createdBy,
		ActorEmail:    userEmail,
		ActorIP:       c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusCreated, toEnvVarResponse(ev))
}

// GetEnvVar returns a single environment variable
// GET /v1/services/:id/env-vars/:var_id
func (h *Handler) GetEnvVar(c *gin.Context) {
	ctx := c.Request.Context()
	varID := c.Param("var_id")

	// Parse var ID
	evID, err := uuid.Parse(varID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variable ID"})
		return
	}

	// Get env var
	ev, err := h.repos.EnvVars.GetByID(ctx, evID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment variable not found"})
		return
	}

	c.JSON(http.StatusOK, toEnvVarResponse(ev))
}

// UpdateEnvVar updates an environment variable
// PUT /v1/services/:id/env-vars/:var_id
func (h *Handler) UpdateEnvVar(c *gin.Context) {
	ctx := c.Request.Context()
	varID := c.Param("var_id")

	// Parse var ID
	evID, err := uuid.Parse(varID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variable ID"})
		return
	}

	// Get existing env var
	ev, err := h.repos.EnvVars.GetByID(ctx, evID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment variable not found"})
		return
	}

	oldValueHash := hashValue(ev.Value)

	var req UpdateEnvVarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.Key != nil {
		if !isValidEnvVarKey(*req.Key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment variable key"})
			return
		}
		ev.Key = *req.Key
	}
	if req.Value != nil {
		ev.Value = *req.Value
	}
	if req.IsSecret != nil {
		ev.IsSecret = *req.IsSecret
	}

	if err := h.repos.EnvVars.Update(ctx, ev); err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			c.JSON(http.StatusConflict, gin.H{"error": "Environment variable with this key already exists"})
			return
		}
		h.logger.Error(ctx, "Failed to update env var", logging.String("var_id", varID), logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment variable"})
		return
	}

	// Get user info
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	var actorID *uuid.UUID
	if userID != "" {
		parsed, _ := uuid.Parse(userID)
		actorID = &parsed
	}

	// Log audit
	h.repos.EnvVars.LogAudit(ctx, &types.EnvVarAuditLog{
		EnvVarID:      ev.ID,
		ServiceID:     ev.ServiceID,
		EnvironmentID: ev.EnvironmentID,
		Action:        "updated",
		Key:           ev.Key,
		OldValueHash:  oldValueHash,
		NewValueHash:  hashValue(ev.Value),
		ActorID:       actorID,
		ActorEmail:    userEmail,
		ActorIP:       c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, toEnvVarResponse(ev))
}

// DeleteEnvVar deletes an environment variable
// DELETE /v1/services/:id/env-vars/:var_id
func (h *Handler) DeleteEnvVar(c *gin.Context) {
	ctx := c.Request.Context()
	varID := c.Param("var_id")

	// Parse var ID
	evID, err := uuid.Parse(varID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variable ID"})
		return
	}

	// Get existing env var for audit log
	ev, err := h.repos.EnvVars.GetByID(ctx, evID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment variable not found"})
		return
	}

	if err := h.repos.EnvVars.Delete(ctx, evID); err != nil {
		h.logger.Error(ctx, "Failed to delete env var", logging.String("var_id", varID), logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete environment variable"})
		return
	}

	// Get user info
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	var actorID *uuid.UUID
	if userID != "" {
		parsed, _ := uuid.Parse(userID)
		actorID = &parsed
	}

	// Log audit
	h.repos.EnvVars.LogAudit(ctx, &types.EnvVarAuditLog{
		EnvVarID:      ev.ID,
		ServiceID:     ev.ServiceID,
		EnvironmentID: ev.EnvironmentID,
		Action:        "deleted",
		Key:           ev.Key,
		OldValueHash:  hashValue(ev.Value),
		ActorID:       actorID,
		ActorEmail:    userEmail,
		ActorIP:       c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Environment variable deleted"})
}

// BulkUpsertEnvVars creates or updates multiple environment variables
// POST /v1/services/:id/env-vars/bulk
func (h *Handler) BulkUpsertEnvVars(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")

	// Parse service ID
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Verify service exists
	_, err = h.repos.Services.GetByID(svcID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	var req BulkUpsertEnvVarsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate limit
	if len(req.Variables) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 100 variables can be created at once"})
		return
	}

	// Parse optional environment ID
	var envID *uuid.UUID
	if req.EnvironmentID != nil && *req.EnvironmentID != "" {
		parsed, err := uuid.Parse(*req.EnvironmentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
			return
		}
		envID = &parsed
	}

	// Validate all keys
	for _, v := range req.Variables {
		if !isValidEnvVarKey(v.Key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment variable key: " + v.Key})
			return
		}
	}

	// Get user info
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	var createdBy *uuid.UUID
	if userID != "" {
		parsed, _ := uuid.Parse(userID)
		createdBy = &parsed
	}

	// Convert to types
	vars := make([]types.EnvironmentVariable, len(req.Variables))
	for i, v := range req.Variables {
		vars[i] = types.EnvironmentVariable{
			Key:            v.Key,
			Value:          v.Value,
			IsSecret:       v.IsSecret,
			CreatedBy:      createdBy,
			CreatedByEmail: userEmail,
		}
	}

	if err := h.repos.EnvVars.BulkUpsert(ctx, svcID, envID, vars); err != nil {
		h.logger.Error(ctx, "Failed to bulk upsert env vars", logging.String("service_id", serviceID), logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment variables"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Environment variables created/updated",
		"count":   len(vars),
	})
}

// RevealEnvVar returns the unmasked value of a secret env var (with audit)
// POST /v1/services/:id/env-vars/:var_id/reveal
func (h *Handler) RevealEnvVar(c *gin.Context) {
	ctx := c.Request.Context()
	varID := c.Param("var_id")

	// Parse var ID
	evID, err := uuid.Parse(varID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variable ID"})
		return
	}

	// Get env var
	ev, err := h.repos.EnvVars.GetByID(ctx, evID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment variable not found"})
		return
	}

	// Get user info
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	var actorID *uuid.UUID
	if userID != "" {
		parsed, _ := uuid.Parse(userID)
		actorID = &parsed
	}

	// Log audit (important for security compliance)
	h.repos.EnvVars.LogAudit(ctx, &types.EnvVarAuditLog{
		EnvVarID:      ev.ID,
		ServiceID:     ev.ServiceID,
		EnvironmentID: ev.EnvironmentID,
		Action:        "revealed",
		Key:           ev.Key,
		ActorID:       actorID,
		ActorEmail:    userEmail,
		ActorIP:       c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	})

	// Return full response with unmasked value
	c.JSON(http.StatusOK, gin.H{
		"id":             ev.ID,
		"service_id":     ev.ServiceID,
		"environment_id": ev.EnvironmentID,
		"key":            ev.Key,
		"value":          ev.Value, // Not masked
		"is_secret":      ev.IsSecret,
		"created_at":     ev.CreatedAt,
		"updated_at":     ev.UpdatedAt,
	})
}

// toEnvVarResponse converts an EnvironmentVariable to response format (masks secrets)
func toEnvVarResponse(ev *types.EnvironmentVariable) types.EnvironmentVariableResponse {
	value := ev.Value
	if ev.IsSecret {
		value = "••••••••" // Mask secret values
	}

	return types.EnvironmentVariableResponse{
		ID:            ev.ID,
		ServiceID:     ev.ServiceID,
		EnvironmentID: ev.EnvironmentID,
		Key:           ev.Key,
		Value:         value,
		IsSecret:      ev.IsSecret,
		CreatedAt:     ev.CreatedAt,
		UpdatedAt:     ev.UpdatedAt,
	}
}

// isValidEnvVarKey validates environment variable key format
// Must start with letter or underscore, contain only alphanumeric and underscores
func isValidEnvVarKey(key string) bool {
	if len(key) == 0 || len(key) > 255 {
		return false
	}

	// First character must be letter or underscore
	first := key[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || first == '_') {
		return false
	}

	// Rest must be alphanumeric or underscore
	for _, c := range key[1:] {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}

// hashValue creates a SHA-256 hash for audit logging
func hashValue(value string) string {
	h := sha256.New()
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
