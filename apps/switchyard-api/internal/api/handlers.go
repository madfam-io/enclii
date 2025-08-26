package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/builder"
	"github.com/madfam/enclii/apps/switchyard-api/internal/cache"
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/apps/switchyard-api/internal/monitoring"
	"github.com/madfam/enclii/apps/switchyard-api/internal/reconciler"
	"github.com/madfam/enclii/apps/switchyard-api/internal/validation"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

type Handler struct {
	repos       *db.Repositories
	config      *config.Config
	auth        *auth.JWTManager
	cache       cache.CacheService
	builder     *builder.BuildpacksBuilder
	k8sClient   *k8s.Client
	controller  *reconciler.Controller
	metrics     *monitoring.MetricsCollector
	logger      logging.Logger
	validator   *validation.Validator
}

func NewHandler(
	repos *db.Repositories,
	config *config.Config,
	auth *auth.JWTManager,
	cache cache.CacheService,
	builder *builder.BuildpacksBuilder,
	k8sClient *k8s.Client,
	controller *reconciler.Controller,
	metrics *monitoring.MetricsCollector,
	logger logging.Logger,
	validator *validation.Validator,
) *Handler {
	return &Handler{
		repos:      repos,
		config:     config,
		auth:       auth,
		cache:      cache,
		builder:    builder,
		k8sClient:  k8sClient,
		controller: controller,
		metrics:    metrics,
		logger:     logger,
		validator:  validator,
	}
}

func SetupRoutes(router *gin.Engine, h *Handler) {
	// Health check (no auth required)
	router.GET("/health", h.Health)

	// API v1 routes with authentication
	v1 := router.Group("/v1")
	v1.Use(h.auth.AuthMiddleware())
	{
		// Projects
		v1.POST("/projects", h.auth.RequireRole(types.RoleAdmin), h.CreateProject)
		v1.GET("/projects", h.ListProjects)
		v1.GET("/projects/:slug", h.GetProject)

		// Services
		v1.POST("/projects/:slug/services", h.auth.RequireRole(types.RoleDeveloper), h.CreateService)
		v1.GET("/projects/:slug/services", h.ListServices)
		v1.GET("/services/:id", h.GetService)

		// Build & Deploy
		v1.POST("/services/:id/build", h.auth.RequireRole(types.RoleDeveloper), h.BuildService)
		v1.GET("/services/:id/releases", h.ListReleases)
		v1.POST("/services/:id/deploy", h.auth.RequireRole(types.RoleDeveloper), h.DeployService)

		// Status
		v1.GET("/services/:id/status", h.GetServiceStatus)
		v1.GET("/deployments/:id/logs", h.GetLogs)
		v1.POST("/deployments/:id/rollback", h.auth.RequireRole(types.RoleDeveloper), h.RollbackDeployment)
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

// Service handlers
func (h *Handler) CreateService(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Get project first
	project, err := h.repos.Project.GetBySlug(ctx, slug)
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

	if err := h.repos.Service.Create(ctx, service); err != nil {
		h.logger.Error(ctx, "Failed to create service", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	c.JSON(http.StatusCreated, service)
}

func (h *Handler) ListServices(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	project, err := h.repos.Project.GetBySlug(ctx, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	services, err := h.repos.Service.ListByProject(ctx, project.ID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list services", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list services"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
}

func (h *Handler) GetService(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	service, err := h.repos.Service.GetByID(ctx, id.String())
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.String("service_id", id.String()), logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	c.JSON(http.StatusOK, service)
}

// Build & Deploy handlers
func (h *Handler) BuildService(c *gin.Context) {
	ctx := c.Request.Context()
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
	service, err := h.repos.Service.GetByID(ctx, serviceID.String())
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Create release record
	release := &types.Release{
		ID:        uuid.New().String(),
		ServiceID: serviceID.String(),
		Version:   "v" + time.Now().Format("20060102-150405") + "-" + req.GitSHA[:7],
		ImageURL:  h.config.Registry + "/" + service.Name + ":" + req.GitSHA[:7],
		GitSHA:    req.GitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repos.Release.Create(ctx, release); err != nil {
		h.logger.Error(ctx, "Failed to create release", logging.Error("db_error", err))
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
	if err := h.repos.Release.UpdateStatus(ctx, release.ID, types.ReleaseStatusReady); err != nil {
		h.logger.Error(ctx, "Failed to update release status", logging.Error("db_error", err))
		h.repos.Release.UpdateStatus(ctx, release.ID, types.ReleaseStatusFailed)
	}

	h.logger.Info(ctx, "Build completed", logging.String("release_id", release.ID))
}

func (h *Handler) ListReleases(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	releases, err := h.repos.Release.ListByService(ctx, serviceID.String())
	if err != nil {
		h.logger.Error(ctx, "Failed to list releases", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

func (h *Handler) DeployService(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	var req struct {
		ReleaseID   string            `json:"release_id" binding:"required"`
		Environment map[string]string `json:"environment"`
		Replicas    int               `json:"replicas,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get service details
	service, err := h.repos.Service.GetByID(ctx, serviceID.String())
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Get release details
	release, err := h.repos.Release.GetByID(ctx, req.ReleaseID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get release", logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Release not found"})
		return
	}

	// Verify release is ready
	if release.Status != types.ReleaseStatusReady {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Release is not ready for deployment"})
		return
	}

	// Create deployment record
	deployment := &types.Deployment{
		ID:          uuid.New().String(),
		ServiceID:   serviceID.String(),
		ReleaseID:   req.ReleaseID,
		Environment: req.Environment,
		Replicas:    req.Replicas,
		Status:      types.DeploymentStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if deployment.Replicas <= 0 {
		deployment.Replicas = 1 // Default to 1 replica
	}

	if err := h.repos.Deployment.Create(ctx, deployment); err != nil {
		h.logger.Error(ctx, "Failed to create deployment", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deployment"})
		return
	}

	// Schedule deployment with reconciler
	h.controller.ScheduleReconciliation(deployment.ID, 1) // High priority

	// Record metrics
	h.metrics.RecordDeployment(service.Name, release.Version)

	h.logger.Info(ctx, "Deployment created", 
		logging.String("deployment_id", deployment.ID),
		logging.String("service_id", serviceID.String()),
		logging.String("release_id", req.ReleaseID))

	c.JSON(http.StatusCreated, deployment)
}

func (h *Handler) GetServiceStatus(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	// Check cache first
	cacheKey := fmt.Sprintf("service:status:%s", serviceID.String())
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		c.Header("X-Cache", "hit")
		c.Data(http.StatusOK, "application/json", cached)
		return
	}

	// Get service
	service, err := h.repos.Service.GetByID(ctx, serviceID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Get latest deployment
	deployments, err := h.repos.Deployment.GetByServiceID(ctx, serviceID.String())
	if err != nil {
		h.logger.Error(ctx, "Failed to get deployments", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get service status"})
		return
	}

	status := gin.H{
		"service":     service,
		"deployments": deployments,
		"status":      "unknown",
	}

	if len(deployments) > 0 {
		latestDeployment := deployments[0]
		status["status"] = string(latestDeployment.Status)
		status["latest_deployment"] = latestDeployment

		// Get Kubernetes status if deployment is active
		if latestDeployment.Status == types.DeploymentStatusActive {
			namespace := fmt.Sprintf("enclii-%s", service.ProjectID)
			if pods, err := h.k8sClient.ListPods(ctx, namespace, fmt.Sprintf("enclii.dev/service=%s", service.Name)); err == nil {
				status["pods"] = pods.Items
				status["running_pods"] = len(pods.Items)
			}
		}
	}

	// Cache for 30 seconds
	if statusJSON, err := json.Marshal(status); err == nil {
		h.cache.Set(ctx, cacheKey, statusJSON, 30*time.Second)
	}

	c.JSON(http.StatusOK, status)
}

func (h *Handler) GetLogs(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	deploymentID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID"})
		return
	}

	// Get deployment
	deployment, err := h.repos.Deployment.GetByID(ctx, deploymentID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Get service to determine namespace
	service, err := h.repos.Service.GetByID(ctx, deployment.ServiceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get service"})
		return
	}

	// Get query parameters for log options
	lines := c.DefaultQuery("lines", "100")
	follow := c.Query("follow") == "true"
	
	linesInt, err := strconv.Atoi(lines)
	if err != nil {
		linesInt = 100
	}

	namespace := fmt.Sprintf("enclii-%s", service.ProjectID)
	labelSelector := fmt.Sprintf("enclii.dev/service=%s", service.Name)

	// Get logs from Kubernetes
	logs, err := h.k8sClient.GetLogs(ctx, namespace, labelSelector, linesInt, follow)
	if err != nil {
		h.logger.Error(ctx, "Failed to get logs", logging.Error("k8s_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
		return
	}

	if follow {
		// Stream logs via SSE
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

		// Stream logs (simplified implementation)
		c.String(http.StatusOK, logs)
	} else {
		// Return logs as JSON
		c.JSON(http.StatusOK, gin.H{
			"deployment_id": deployment.ID,
			"service_name":  service.Name,
			"logs":          logs,
			"lines":         linesInt,
		})
	}
}

func (h *Handler) RollbackDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	deploymentID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deployment ID"})
		return
	}

	// Get deployment
	deployment, err := h.repos.Deployment.GetByID(ctx, deploymentID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Get service
	service, err := h.repos.Service.GetByID(ctx, deployment.ServiceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get service"})
		return
	}

	// Find previous successful deployment
	deployments, err := h.repos.Deployment.GetByServiceID(ctx, deployment.ServiceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get deployments", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find previous deployment"})
		return
	}

	var previousDeployment *types.Deployment
	for _, d := range deployments {
		if d.ID != deployment.ID && d.Status == types.DeploymentStatusActive {
			previousDeployment = d
			break
		}
	}

	if previousDeployment == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No previous deployment found to rollback to"})
		return
	}

	// Update current deployment status
	if err := h.repos.Deployment.UpdateStatus(ctx, deployment.ID, types.DeploymentStatusRolledBack, "Rolled back by user"); err != nil {
		h.logger.Error(ctx, "Failed to update deployment status", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deployment status"})
		return
	}

	// Trigger rollback in Kubernetes
	namespace := fmt.Sprintf("enclii-%s", service.ProjectID)
	if err := h.reconciler.Rollback(ctx, namespace, service.Name); err != nil {
		h.logger.Error(ctx, "Failed to rollback in Kubernetes", logging.Error("k8s_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rollback deployment"})
		return
	}

	// Clear cache
	h.cache.DelByTag(ctx, fmt.Sprintf("service:%s", service.ID))

	// Record metrics
	h.metrics.RecordRollback(service.Name)

	h.logger.Info(ctx, "Deployment rolled back",
		logging.String("deployment_id", deployment.ID),
		logging.String("service_id", service.ID),
		logging.String("previous_deployment_id", previousDeployment.ID))

	c.JSON(http.StatusOK, gin.H{
		"message":            "Deployment rolled back successfully",
		"rolled_back_to":     previousDeployment,
		"current_deployment": deployment,
	})
}