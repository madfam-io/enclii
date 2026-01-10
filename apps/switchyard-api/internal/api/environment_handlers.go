package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CreateEnvironment creates a new environment for a project
func (h *Handler) CreateEnvironment(c *gin.Context) {
	ctx := c.Request.Context()
	projectSlug := c.Param("slug")

	var req struct {
		Name          string `json:"name" binding:"required"`
		KubeNamespace string `json:"kube_namespace"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get project by slug
	project, err := h.repos.Projects.GetBySlug(projectSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Check if environment already exists
	existing, _ := h.repos.Environments.GetByProjectAndName(project.ID, req.Name)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Environment already exists"})
		return
	}

	// Generate kube_namespace if not provided
	// Use consistent pattern: enclii-{project_slug}-{env_name}
	kubeNamespace := req.KubeNamespace
	if kubeNamespace == "" {
		envNameNormalized := strings.ToLower(strings.ReplaceAll(req.Name, "_", "-"))
		kubeNamespace = fmt.Sprintf("enclii-%s-%s", projectSlug, envNameNormalized)
	}

	env := &types.Environment{
		ProjectID:     project.ID,
		Name:          req.Name,
		KubeNamespace: kubeNamespace,
	}

	if err := h.repos.Environments.Create(env); err != nil {
		h.logger.Error(ctx, "Failed to create environment",
			logging.Error("error", err),
			logging.String("project", projectSlug),
			logging.String("environment", req.Name),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	c.JSON(http.StatusCreated, env)
}

// ListEnvironments returns all environments for a project
func (h *Handler) ListEnvironments(c *gin.Context) {
	projectSlug := c.Param("slug")

	// Get project by slug
	project, err := h.repos.Projects.GetBySlug(projectSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	environments, err := h.repos.Environments.ListByProject(project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"environments": environments})
}

// GetEnvironment returns a specific environment
func (h *Handler) GetEnvironment(c *gin.Context) {
	ctx := c.Request.Context()
	projectSlug := c.Param("slug")
	envName := c.Param("env_name")

	// Get project by slug
	project, err := h.repos.Projects.GetBySlug(projectSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	env, err := h.repos.Environments.GetByProjectAndName(project.ID, envName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Optionally get ID-based lookup
	if envName == "" {
		envIDStr := c.Param("env_id")
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
			return
		}
		env, err = h.repos.Environments.GetByID(ctx, envID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
			return
		}
	}

	c.JSON(http.StatusOK, env)
}
