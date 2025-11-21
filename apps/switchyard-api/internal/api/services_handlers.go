package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/services"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// CreateService creates a new service in a project
func (h *Handler) CreateService(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Get project first to get project ID
	project, err := h.projectService.GetProject(ctx, slug)
	if err != nil {
		if errors.Is(err, errors.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		}
		return
	}

	var req struct {
		Name        string            `json:"name" binding:"required"`
		GitRepo     string            `json:"git_repo" binding:"required"`
		BuildConfig types.BuildConfig `json:"build_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use service layer for service creation
	createReq := &services.CreateServiceRequest{
		ProjectID:   project.ID.String(),
		Name:        req.Name,
		GitRepo:     req.GitRepo,
		BuildConfig: req.BuildConfig,
		UserID:      c.GetString("user_id"),
		UserEmail:   c.GetString("user_email"),
		UserRole:    c.GetString("user_role"),
	}

	resp, err := h.projectService.CreateService(ctx, createReq)
	if err != nil {
		// Map service errors to HTTP status codes
		if errors.Is(err, errors.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		} else if errors.Is(err, errors.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		}
		return
	}

	c.JSON(http.StatusCreated, resp.Service)
}

// ListServices returns all services in a project
func (h *Handler) ListServices(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Use service layer for listing services
	svcList, err := h.projectService.ListServices(ctx, slug)
	if err != nil {
		if errors.Is(err, errors.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": svcList})
}

// GetService returns a service by ID
func (h *Handler) GetService(c *gin.Context) {
	ctx := c.Request.Context()
	serviceID := c.Param("id")

	// Use service layer for getting service
	service, err := h.projectService.GetService(ctx, serviceID)
	if err != nil {
		if errors.Is(err, errors.ErrServiceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get service"})
		}
		return
	}

	c.JSON(http.StatusOK, service)
}
