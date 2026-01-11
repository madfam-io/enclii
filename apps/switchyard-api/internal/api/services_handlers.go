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

// BulkServiceRequest represents a single service in a bulk import request
type BulkServiceRequest struct {
	Name             string `json:"name" binding:"required"`
	AppPath          string `json:"app_path" binding:"required"`
	Port             int    `json:"port"`
	BuildCommand     string `json:"build_command"`
	StartCommand     string `json:"start_command"`
	AutoDeploy       *bool  `json:"auto_deploy"`        // Enable auto-deploy (defaults to true)
	AutoDeployBranch string `json:"auto_deploy_branch"` // Override branch for this service
	AutoDeployEnv    string `json:"auto_deploy_env"`    // Target environment (e.g., "production")
}

// BulkCreateServicesRequest represents a request to create multiple services at once
type BulkCreateServicesRequest struct {
	GitRepo   string               `json:"git_repo" binding:"required"`
	GitBranch string               `json:"git_branch"`
	Services  []BulkServiceRequest `json:"services" binding:"required,min=1"`
}

// BulkCreateServicesResponse represents the response from bulk service creation
type BulkCreateServicesResponse struct {
	Services []types.Service `json:"services"`
	Errors   []string        `json:"errors,omitempty"`
}

// BulkCreateServices creates multiple services in a project from a monorepo import
// POST /v1/projects/:slug/services/bulk
func (h *Handler) BulkCreateServices(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Get project first
	project, err := h.projectService.GetProject(ctx, slug)
	if err != nil {
		if errors.Is(err, errors.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		}
		return
	}

	var req BulkCreateServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if len(req.Services) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one service is required"})
		return
	}

	if len(req.Services) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 20 services can be created at once"})
		return
	}

	// Create services one by one (could be optimized with batch insert)
	createdServices := make([]types.Service, 0, len(req.Services))
	var createErrors []string

	for _, svc := range req.Services {
		// Normalize app path
		appPath := svc.AppPath
		if appPath == "." {
			appPath = ""
		}

		// Determine auto-deploy branch (service-level overrides request-level)
		autoDeployBranch := req.GitBranch
		if svc.AutoDeployBranch != "" {
			autoDeployBranch = svc.AutoDeployBranch
		}

		createReq := &services.CreateServiceRequest{
			ProjectID:        project.ID.String(),
			Name:             svc.Name,
			GitRepo:          req.GitRepo,
			AppPath:          appPath,
			AutoDeploy:       svc.AutoDeploy,
			AutoDeployBranch: autoDeployBranch,
			AutoDeployEnv:    svc.AutoDeployEnv,
			BuildConfig: types.BuildConfig{
				Type: types.BuildTypeBuildpack, // Default to buildpack
			},
			UserID:    c.GetString("user_id"),
			UserEmail: c.GetString("user_email"),
			UserRole:  c.GetString("user_role"),
		}

		resp, err := h.projectService.CreateService(ctx, createReq)
		if err != nil {
			createErrors = append(createErrors, svc.Name+": "+err.Error())
			continue
		}

		createdServices = append(createdServices, *resp.Service)
	}

	// Return partial success if some services were created
	response := BulkCreateServicesResponse{
		Services: createdServices,
		Errors:   createErrors,
	}

	if len(createdServices) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create any services",
			"details": createErrors,
		})
		return
	}

	if len(createErrors) > 0 {
		c.JSON(http.StatusMultiStatus, response)
		return
	}

	c.JSON(http.StatusCreated, response)
}
