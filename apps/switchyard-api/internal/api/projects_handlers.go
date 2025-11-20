package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/services"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CreateProject creates a new project
func (h *Handler) CreateProject(c *gin.Context) {
	ctx := c.Request.Context()
	var req types.CreateProjectRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if err := h.validator.Validate(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use service layer for project creation
	createReq := &services.CreateProjectRequest{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		UserID:      c.GetString("user_id"),
		UserEmail:   c.GetString("user_email"),
		UserRole:    c.GetString("user_role"),
	}

	resp, err := h.projectService.CreateProject(ctx, createReq)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, errors.ErrSlugAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "A project with this slug already exists"})
		} else if errors.Is(err, errors.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		}
		return
	}

	// Clear cache
	h.cache.DelByTag(ctx, "projects")

	// Record metrics
	h.metrics.RecordProjectCreated()

	c.JSON(http.StatusCreated, resp.Project)
}

// ListProjects returns all projects
func (h *Handler) ListProjects(c *gin.Context) {
	ctx := c.Request.Context()

	// Use service layer for listing projects
	projects, err := h.projectService.ListProjects(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// GetProject returns a project by slug
func (h *Handler) GetProject(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Use service layer for getting project
	project, err := h.projectService.GetProject(ctx, slug)
	if err != nil {
		if errors.Is(err, errors.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		}
		return
	}

	c.JSON(http.StatusOK, project)
}
