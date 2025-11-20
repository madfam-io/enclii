package api

import (
	"github.com/gin-gonic/gin"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam/enclii/apps/switchyard-api/internal/builder"
	"github.com/madfam/enclii/apps/switchyard-api/internal/cache"
	"github.com/madfam/enclii/apps/switchyard-api/internal/compliance"
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/apps/switchyard-api/internal/monitoring"
	"github.com/madfam/enclii/apps/switchyard-api/internal/provenance"
	"github.com/madfam/enclii/apps/switchyard-api/internal/reconciler"
	"github.com/madfam/enclii/apps/switchyard-api/internal/services"
	"github.com/madfam/enclii/apps/switchyard-api/internal/topology"
	"github.com/madfam/enclii/apps/switchyard-api/internal/validation"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Handler contains all dependencies for HTTP handlers
type Handler struct {
	// Repositories (legacy - prefer using services)
	repos *db.Repositories

	// Service Layer (business logic)
	authService       *services.AuthService
	projectService    *services.ProjectService
	deploymentService *services.DeploymentService

	// Infrastructure
	config             *config.Config
	auth               auth.AuthManager // Interface supporting both JWTManager and OIDCManager
	auditMiddleware    *audit.Middleware
	cache              cache.CacheService
	builder            *builder.Service
	k8sClient          *k8s.Client
	reconciler         *reconciler.Controller
	serviceReconciler  *reconciler.ServiceReconciler
	metrics            *monitoring.MetricsCollector
	logger             logging.Logger
	validator          *validation.Validator
	provenanceChecker  *provenance.Checker
	complianceExporter *compliance.Exporter
	topologyBuilder    *topology.GraphBuilder
}

// NewHandler creates a new API handler with all dependencies
func NewHandler(
	repos *db.Repositories,
	config *config.Config,
	auth auth.AuthManager, // Can be JWTManager or OIDCManager
	cache cache.CacheService,
	builder *builder.Service,
	k8sClient *k8s.Client,
	reconciler *reconciler.Controller,
	serviceReconciler *reconciler.ServiceReconciler,
	metrics *monitoring.MetricsCollector,
	logger logging.Logger,
	validator *validation.Validator,
	provenanceChecker *provenance.Checker,
	complianceExporter *compliance.Exporter,
	topologyBuilder *topology.GraphBuilder,
	// Service layer
	authService *services.AuthService,
	projectService *services.ProjectService,
	deploymentService *services.DeploymentService,
) *Handler {
	return &Handler{
		// Repositories
		repos: repos,

		// Services
		authService:       authService,
		projectService:    projectService,
		deploymentService: deploymentService,

		// Infrastructure
		config:             config,
		auth:               auth,
		auditMiddleware:    audit.NewMiddleware(repos),
		cache:              cache,
		builder:            builder,
		k8sClient:          k8sClient,
		reconciler:         reconciler,
		serviceReconciler:  serviceReconciler,
		metrics:            metrics,
		logger:             logger,
		validator:          validator,
		provenanceChecker:  provenanceChecker,
		complianceExporter: complianceExporter,
		topologyBuilder:    topologyBuilder,
	}
}

// SetupRoutes configures all API routes
// Handler methods are implemented in separate files:
// - auth_handlers.go: Authentication endpoints
// - health_handlers.go: Health check endpoints
// - projects_handlers.go: Project CRUD operations
// - services_handlers.go: Service CRUD operations
// - build_handlers.go: Build and release management
// - deployment_handlers.go: Deployment operations
// - domain_handlers.go: Custom domain management
// - topology_handlers.go: Service dependency graph
func SetupRoutes(router *gin.Engine, h *Handler) {
	// Health check (no auth required)
	router.GET("/health", h.Health)
	router.GET("/v1/build/status", h.GetBuildStatus)

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Auth routes - Different endpoints based on auth mode
		if h.config.AuthMode == "oidc" {
			// ===== OIDC Mode (Production with Plinto) =====
			// Redirect to OIDC provider for login
			v1.GET("/auth/login", h.OIDCLogin)

			// OAuth callback from OIDC provider
			v1.GET("/auth/callback", h.OIDCCallback)

			// Registration is handled by OIDC provider (Plinto)
			// POST /auth/register is not available in OIDC mode

		} else {
			// ===== Local Mode (Bootstrap) =====
			// Local user registration
			v1.POST("/auth/register", h.auditMiddleware.AuditMiddleware(), h.Register)

			// Local login with email/password
			v1.POST("/auth/login", h.auditMiddleware.AuditMiddleware(), h.Login)

			// JWKS endpoint for external services to verify our tokens
			v1.GET("/auth/jwks", h.JWKS)
		}

		// Common auth endpoints (both modes)
		v1.POST("/auth/refresh", h.RefreshToken)
		v1.POST("/auth/logout", h.auth.AuthMiddleware(), h.auditMiddleware.AuditMiddleware(), h.Logout)

		// Protected routes (require authentication + audit)
		// These work the same way in both local and OIDC modes
		protected := v1.Group("")
		protected.Use(h.auth.AuthMiddleware())
		protected.Use(h.auditMiddleware.AuditMiddleware())
		{
			// Projects
			protected.POST("/projects", h.auth.RequireRole(types.RoleAdmin), h.CreateProject)
			protected.GET("/projects", h.ListProjects)
			protected.GET("/projects/:slug", h.GetProject)

			// Services
			protected.POST("/projects/:slug/services", h.auth.RequireRole(types.RoleDeveloper), h.CreateService)
			protected.GET("/projects/:slug/services", h.ListServices)
			protected.GET("/services/:id", h.GetService)

			// Build & Deploy
			protected.POST("/services/:id/build", h.auth.RequireRole(types.RoleDeveloper), h.BuildService)
			protected.GET("/services/:id/releases", h.ListReleases)
			protected.POST("/services/:id/deploy", h.auth.RequireRole(types.RoleDeveloper), h.DeployService)

			// Status & Deployments
			protected.GET("/services/:id/status", h.GetServiceStatus)
			protected.GET("/services/:id/deployments", h.ListServiceDeployments)
			protected.GET("/services/:id/deployments/latest", h.GetLatestDeployment)
			protected.GET("/deployments/:id", h.GetDeployment)
			protected.GET("/deployments/:id/logs", h.GetLogs)
			protected.POST("/deployments/:id/rollback", h.auth.RequireRole(types.RoleDeveloper), h.RollbackDeployment)

			// Topology
			protected.GET("/topology", h.GetTopology)
			protected.GET("/topology/services/:id/dependencies", h.GetServiceDependencies)
			protected.GET("/topology/services/:id/impact", h.GetServiceImpact)
			protected.GET("/topology/path", h.FindDependencyPath)

			// Custom Domains
			protected.POST("/services/:service_id/domains", h.auth.RequireRole(types.RoleDeveloper), h.AddCustomDomain)
			protected.GET("/services/:service_id/domains", h.ListCustomDomains)
			protected.GET("/services/:service_id/domains/:domain_id", h.GetCustomDomain)
			protected.PATCH("/services/:service_id/domains/:domain_id", h.auth.RequireRole(types.RoleDeveloper), h.UpdateCustomDomain)
			protected.DELETE("/services/:service_id/domains/:domain_id", h.auth.RequireRole(types.RoleDeveloper), h.DeleteCustomDomain)
			protected.POST("/services/:service_id/domains/:domain_id/verify", h.auth.RequireRole(types.RoleDeveloper), h.VerifyCustomDomain)
		}
	}
}
