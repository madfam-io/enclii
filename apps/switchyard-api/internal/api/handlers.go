package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

type Handler struct {
	repos  *db.Repositories
	config *config.Config
}

func NewHandler(repos *db.Repositories, config *config.Config) *Handler {
	return &Handler{
		repos:  repos,
		config: config,
	}
}

func SetupRoutes(router *gin.Engine, h *Handler) {
	// Health check
	router.GET("/health", h.Health)

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Projects
		v1.POST("/projects", h.CreateProject)
		v1.GET("/projects", h.ListProjects)
		v1.GET("/projects/:slug", h.GetProject)

		// Services
		v1.POST("/projects/:slug/services", h.CreateService)
		v1.GET("/projects/:slug/services", h.ListServices)
		v1.GET("/services/:id", h.GetService)

		// Build & Deploy
		v1.POST("/services/:id/build", h.BuildService)
		v1.GET("/services/:id/releases", h.ListReleases)
		v1.POST("/services/:id/deploy", h.DeployService)

		// Status
		v1.GET("/services/:id/status", h.GetServiceStatus)
		v1.GET("/deployments/:id/logs", h.GetLogs)
		v1.POST("/deployments/:id/rollback", h.RollbackDeployment)
	}
}

// Health check endpoint
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "switchyard-api",
		"version": "1.0.0",
	})
}

// Project handlers
func (h *Handler) CreateProject(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Slug string `json:"slug" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := &types.Project{
		Name: req.Name,
		Slug: req.Slug,
	}

	if err := h.repos.Projects.Create(project); err != nil {
		logrus.Errorf("Failed to create project: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, project)
}

func (h *Handler) ListProjects(c *gin.Context) {
	projects, err := h.repos.Projects.List()
	if err != nil {
		logrus.Errorf("Failed to list projects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *Handler) GetProject(c *gin.Context) {
	slug := c.Param("slug")

	project, err := h.repos.Projects.GetBySlug(slug)
	if err != nil {
		logrus.Errorf("Failed to get project %s: %v", slug, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, project)
}

// Service handlers
func (h *Handler) CreateService(c *gin.Context) {
	slug := c.Param("slug")

	// Get project first
	project, err := h.repos.Projects.GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
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

	service := &types.Service{
		ProjectID:   project.ID,
		Name:        req.Name,
		GitRepo:     req.GitRepo,
		BuildConfig: req.BuildConfig,
	}

	if err := h.repos.Services.Create(service); err != nil {
		logrus.Errorf("Failed to create service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	c.JSON(http.StatusCreated, service)
}

func (h *Handler) ListServices(c *gin.Context) {
	slug := c.Param("slug")

	project, err := h.repos.Projects.GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	services, err := h.repos.Services.ListByProject(project.ID)
	if err != nil {
		logrus.Errorf("Failed to list services: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
}

func (h *Handler) GetService(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	service, err := h.repos.Services.GetByID(id)
	if err != nil {
		logrus.Errorf("Failed to get service %s: %v", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	c.JSON(http.StatusOK, service)
}

// Build & Deploy handlers
func (h *Handler) BuildService(c *gin.Context) {
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	var req struct {
		GitSHA string `json:"git_sha" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get service details
	service, err := h.repos.Services.GetByID(serviceID)
	if err != nil {
		logrus.Errorf("Failed to get service: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Create release record
	release := &types.Release{
		ServiceID: serviceID,
		Version:   "v" + time.Now().Format("20060102-150405") + "-" + req.GitSHA[:7],
		ImageURI:  h.config.Registry + "/" + service.Name + ":" + req.GitSHA[:7],
		GitSHA:    req.GitSHA,
		Status:    types.ReleaseStatusBuilding,
	}

	if err := h.repos.Releases.Create(release); err != nil {
		logrus.Errorf("Failed to create release: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create release"})
		return
	}

	// Trigger async build process
	go h.triggerBuild(service, release, req.GitSHA)

	c.JSON(http.StatusCreated, release)
}

func (h *Handler) triggerBuild(service *types.Service, release *types.Release, gitSHA string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// TODO: Clone repository and trigger build
	// For MVP, we'll simulate the build process
	time.Sleep(10 * time.Second) // Simulate build time

	// Update release status to ready (in real implementation, this would be based on actual build result)
	if err := h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusReady); err != nil {
		logrus.Errorf("Failed to update release status: %v", err)
		h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
	}

	logrus.Infof("Build completed for release %s", release.ID)
}

func (h *Handler) ListReleases(c *gin.Context) {
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	releases, err := h.repos.Releases.ListByService(serviceID)
	if err != nil {
		logrus.Errorf("Failed to list releases: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

func (h *Handler) DeployService(c *gin.Context) {
	// TODO: Implement deployment logic
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) GetServiceStatus(c *gin.Context) {
	// TODO: Implement service status
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) GetLogs(c *gin.Context) {
	// TODO: Implement log streaming
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}

func (h *Handler) RollbackDeployment(c *gin.Context) {
	// TODO: Implement rollback
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented yet"})
}