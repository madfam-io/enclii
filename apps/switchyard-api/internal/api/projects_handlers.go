package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CreateProject creates a new project
func (h *Handler) CreateProject(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.CreateProjectRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "Invalid request body", logging.Error("bind_error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		h.logger.Error(ctx, "Validation failed", logging.Error("validation_error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := &types.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.repos.Project.Create(ctx, project); err != nil {
		h.logger.Error(ctx, "Failed to create project", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	// Clear cache
	h.cache.DelByTag(ctx, "projects")

	// Record metrics
	h.metrics.RecordProjectCreated()

	h.logger.Info(ctx, "Project created", logging.String("project_id", project.ID))
	c.JSON(http.StatusCreated, project)
}

// ListProjects returns all projects
func (h *Handler) ListProjects(c *gin.Context) {
	ctx := c.Request.Context()
	projects, err := h.repos.Project.List(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to list projects", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// GetProject returns a project by slug
func (h *Handler) GetProject(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	project, err := h.repos.Project.GetBySlug(ctx, slug)
	if err != nil {
		h.logger.Error(ctx, "Failed to get project", logging.String("slug", slug), logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, project)
}
