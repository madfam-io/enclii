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
	"github.com/madfam/enclii/apps/switchyard-api/internal/middleware"
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
	authService            *services.AuthService
	projectService         *services.ProjectService
	deploymentService      *services.DeploymentService
	deploymentGroupService *services.DeploymentGroupService

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

	// Build concurrency control - semaphore to limit concurrent builds (prevents OOM)
	buildSemaphore chan struct{}
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
	deploymentGroupService *services.DeploymentGroupService,
) *Handler {
	// Create build semaphore with capacity 1 to serialize builds
	// This prevents OOM when multiple webhook builds are triggered simultaneously
	buildSem := make(chan struct{}, 1)

	return &Handler{
		// Repositories
		repos: repos,

		// Services
		authService:            authService,
		projectService:         projectService,
		deploymentService:      deploymentService,
		deploymentGroupService: deploymentGroupService,

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

		// Build concurrency control
		buildSemaphore: buildSem,
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
// - webhook_handlers.go: GitHub webhook handlers
// - observability_handlers.go: Metrics and monitoring endpoints
func SetupRoutes(router *gin.Engine, h *Handler) {
	// HTTP metrics middleware (collect request metrics for all routes)
	router.Use(h.metrics.HTTPMetricsMiddleware())

	// Prometheus metrics endpoint (for scraping by Prometheus/Grafana)
	router.GET("/metrics", gin.WrapH(h.metrics.Handler()))

	// Health check (no auth required)
	router.GET("/health", h.Health)
	router.GET("/v1/build/status", h.GetBuildStatus)

	// Dashboard stats (public endpoint for local development)
	router.GET("/v1/dashboard/stats", h.GetDashboardStats)

	// GitHub webhook (no auth required - uses HMAC signature verification)
	// Endpoint for GitHub to send push events for auto-deployments
	router.POST("/v1/webhooks/github", h.GitHubWebhook)

	// Rate limiters for auth endpoints
	authRateLimiter := middleware.NewAuthRateLimiter()             // 10 req/min per IP
	strictAuthRateLimiter := middleware.NewStrictAuthRateLimiter() // 5 req/min per IP

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Auth routes - Different endpoints based on auth mode
		if h.config.AuthMode == "oidc" {
			// ===== OIDC Mode (Production with Janua) =====
			// Redirect to OIDC provider for login (rate limited)
			v1.GET("/auth/login", authRateLimiter.Middleware(), h.OIDCLogin)

			// OAuth callback from OIDC provider (rate limited)
			v1.GET("/auth/callback", authRateLimiter.Middleware(), h.OIDCCallback)

			// Silent auth check for detecting existing SSO sessions (no rate limit - iframe use)
			v1.GET("/auth/silent-check", h.OIDCSilentCheck)

			// Silent callback for iframe-based auth (no rate limit - iframe use)
			v1.GET("/auth/callback/silent", h.OIDCSilentCallback)

			// Registration is handled by OIDC provider (Janua)
			// POST /auth/register is not available in OIDC mode

		} else {
			// ===== Local Mode (Bootstrap) =====
			// Local user registration (strict rate limit - abuse prevention)
			v1.POST("/auth/register", strictAuthRateLimiter.Middleware(), h.auditMiddleware.AuditMiddleware(), h.Register)

			// Local login with email/password (strict rate limit - brute force prevention)
			v1.POST("/auth/login", strictAuthRateLimiter.Middleware(), h.auditMiddleware.AuditMiddleware(), h.Login)

			// JWKS endpoint for external services to verify our tokens
			v1.GET("/auth/jwks", h.JWKS)
		}

		// Common auth endpoints (both modes) - rate limited
		v1.POST("/auth/refresh", authRateLimiter.Middleware(), h.RefreshToken)
		v1.POST("/auth/logout", authRateLimiter.Middleware(), h.auth.AuthMiddleware(), h.auditMiddleware.AuditMiddleware(), h.Logout)

		// Protected routes (require authentication + audit)
		// These work the same way in both local and OIDC modes
		protected := v1.Group("")
		protected.Use(h.auth.AuthMiddleware())
		protected.Use(h.auditMiddleware.AuditMiddleware())
		{
			// Projects
			protected.POST("/projects", h.auth.RequireRole(string(types.RoleAdmin)), h.CreateProject)
			protected.GET("/projects", h.ListProjects)
			protected.GET("/projects/:slug", h.GetProject)

			// Environments
			protected.POST("/projects/:slug/environments", h.auth.RequireRole(string(types.RoleDeveloper)), h.CreateEnvironment)
			protected.GET("/projects/:slug/environments", h.ListEnvironments)
			protected.GET("/projects/:slug/environments/:env_name", h.GetEnvironment)

			// Services
			protected.POST("/projects/:slug/services", h.auth.RequireRole(string(types.RoleDeveloper)), h.CreateService)
			protected.POST("/projects/:slug/services/bulk", h.auth.RequireRole(string(types.RoleDeveloper)), h.BulkCreateServices)
			protected.GET("/projects/:slug/services", h.ListServices)
			protected.GET("/services/:id", h.GetService)
			protected.GET("/services/:id/settings", h.GetServiceSettings)
			protected.PATCH("/services/:id", h.auth.RequireRole(string(types.RoleDeveloper)), h.UpdateService)
			protected.DELETE("/services/:id", h.auth.RequireRole(string(types.RoleAdmin)), h.DeleteService)

			// Build & Deploy
			protected.POST("/services/:id/build", h.auth.RequireRole(string(types.RoleDeveloper)), h.BuildService)
			protected.GET("/services/:id/releases", h.ListReleases)
			protected.POST("/services/:id/deploy", h.auth.RequireRole(string(types.RoleDeveloper)), h.DeployService)

			// Status & Deployments
			protected.GET("/services/:id/status", h.GetServiceStatus)
			protected.GET("/services/:id/deployments", h.ListServiceDeployments)
			protected.GET("/services/:id/deployments/latest", h.GetLatestDeployment)
			protected.GET("/deployments/:id", h.GetDeployment)
			protected.GET("/deployments/:id/logs", h.GetLogs)
			protected.POST("/deployments/:id/rollback", h.auth.RequireRole(string(types.RoleDeveloper)), h.RollbackDeployment)

			// Real-time Logs (WebSocket streaming)
			protected.GET("/services/:id/logs/stream", h.StreamServiceLogsWS)
			protected.GET("/services/:id/logs/history", h.GetLogsHistory)
			protected.POST("/services/:id/logs/search", h.SearchLogs)
			protected.GET("/deployments/:id/logs/stream", h.StreamLogsWS)
			protected.GET("/services/:id/builds/:build_id/logs", h.GetBuildLogs)
			protected.GET("/services/:id/builds/:build_id/logs/stream", h.StreamBuildLogsWS)

			// Topology
			protected.GET("/topology", h.GetTopology)
			protected.GET("/topology/services/:id/dependencies", h.GetServiceDependencies)
			protected.GET("/topology/services/:id/impact", h.GetServiceImpact)
			protected.GET("/topology/path", h.FindDependencyPath)

			// Networking & Custom Domains
			protected.GET("/services/:id/networking", h.GetServiceNetworking)
			protected.POST("/services/:id/domains", h.auth.RequireRole(string(types.RoleDeveloper)), h.AddServiceDomain)
			protected.GET("/services/:id/domains", h.ListCustomDomains)
			protected.GET("/services/:id/domains/:domain_id", h.GetCustomDomain)
			protected.PATCH("/services/:id/domains/:domain_id", h.auth.RequireRole(string(types.RoleDeveloper)), h.UpdateCustomDomain)
			protected.DELETE("/services/:id/domains/:domain_id", h.auth.RequireRole(string(types.RoleDeveloper)), h.DeleteCustomDomain)
			protected.POST("/services/:id/domains/:domain_id/verify", h.auth.RequireRole(string(types.RoleDeveloper)), h.VerifyCustomDomain)
			protected.PUT("/domains/:domain_id/protection", h.auth.RequireRole(string(types.RoleDeveloper)), h.ToggleZeroTrust)

			// Environments
			protected.GET("/environments", h.GetEnvironments)

			// Integrations (GitHub via Janua OAuth tokens)
			protected.GET("/integrations/github/status", h.GetGitHubStatus)
			protected.GET("/integrations/github/repos", h.ListGitHubRepos)
			protected.POST("/integrations/github/link", h.LinkGitHub)
			protected.GET("/integrations/github/repos/:owner/:repo/branches", h.GetRepositoryBranches)
			protected.POST("/integrations/github/repos/:owner/:repo/analyze", h.AnalyzeRepository)

			// Deployment Groups (coordinated multi-service deployments)
			protected.POST("/projects/:slug/environments/:env_name/deployment-groups", h.auth.RequireRole(string(types.RoleDeveloper)), h.CreateDeploymentGroup)
			protected.GET("/projects/:slug/deployment-groups", h.ListDeploymentGroups)
			protected.GET("/projects/:slug/deployment-groups/:group_id", h.GetDeploymentGroup)
			protected.POST("/projects/:slug/deployment-groups/:group_id/execute", h.auth.RequireRole(string(types.RoleDeveloper)), h.ExecuteDeploymentGroup)
			protected.POST("/projects/:slug/deployment-groups/:group_id/rollback", h.auth.RequireRole(string(types.RoleDeveloper)), h.RollbackDeploymentGroup)

			// Service Dependencies
			protected.POST("/services/:id/dependencies", h.auth.RequireRole(string(types.RoleDeveloper)), h.AddServiceDependency)
			protected.GET("/services/:id/dependencies", h.ListServiceDependencies)
			protected.GET("/services/:id/dependents", h.ListServiceDependents)
			protected.DELETE("/services/:id/dependencies/:depends_on_id", h.auth.RequireRole(string(types.RoleDeveloper)), h.RemoveServiceDependency)

			// Environment Variables
			protected.GET("/services/:id/env-vars", h.ListEnvVars)
			protected.POST("/services/:id/env-vars", h.auth.RequireRole(string(types.RoleDeveloper)), h.CreateEnvVar)
			protected.GET("/services/:id/env-vars/:var_id", h.GetEnvVar)
			protected.PUT("/services/:id/env-vars/:var_id", h.auth.RequireRole(string(types.RoleDeveloper)), h.UpdateEnvVar)
			protected.DELETE("/services/:id/env-vars/:var_id", h.auth.RequireRole(string(types.RoleDeveloper)), h.DeleteEnvVar)
			protected.POST("/services/:id/env-vars/bulk", h.auth.RequireRole(string(types.RoleDeveloper)), h.BulkUpsertEnvVars)
			protected.POST("/services/:id/env-vars/:var_id/reveal", h.auth.RequireRole(string(types.RoleDeveloper)), h.RevealEnvVar)

			// Preview Environments (PR-based ephemeral deployments)
			protected.GET("/services/:id/previews", h.ListPreviews)
			protected.GET("/projects/:slug/previews", h.ListProjectPreviews)
			protected.GET("/previews/:id", h.GetPreview)
			protected.POST("/previews", h.auth.RequireRole(string(types.RoleDeveloper)), h.CreatePreview)
			protected.POST("/previews/:id/close", h.auth.RequireRole(string(types.RoleDeveloper)), h.ClosePreview)
			protected.POST("/previews/:id/wake", h.auth.RequireRole(string(types.RoleDeveloper)), h.WakePreview)
			protected.DELETE("/previews/:id", h.auth.RequireRole(string(types.RoleAdmin)), h.DeletePreview)
			protected.POST("/previews/:id/access", h.RecordPreviewAccess)

			// Preview Comments (collaborative feedback)
			protected.GET("/previews/:id/comments", h.ListPreviewComments)
			protected.POST("/previews/:id/comments", h.CreatePreviewComment)
			protected.POST("/previews/:id/comments/:comment_id/resolve", h.ResolvePreviewComment)

			// Teams (Railway/Vercel-style team management)
			protected.POST("/teams", h.CreateTeam)
			protected.GET("/teams", h.ListTeams)
			protected.GET("/teams/:slug", h.GetTeam)
			protected.PATCH("/teams/:slug", h.UpdateTeam)
			protected.DELETE("/teams/:slug", h.DeleteTeam)

			// Team Members
			protected.GET("/teams/:slug/members", h.ListTeamMembers)
			protected.PATCH("/teams/:slug/members/:member_id", h.UpdateMemberRole)
			protected.DELETE("/teams/:slug/members/:member_id", h.RemoveTeamMember)

			// Team Invitations (team admin operations)
			protected.POST("/teams/:slug/invitations", h.InviteTeamMember)
			protected.GET("/teams/:slug/invitations", h.ListTeamInvitations)
			protected.DELETE("/teams/:slug/invitations/:invitation_id", h.CancelTeamInvitation)

			// User Invitations (personal invitation management)
			protected.GET("/invitations", h.ListMyInvitations)
			protected.GET("/invitations/:token", h.GetInvitationByToken)
			protected.POST("/invitations/:token/accept", h.AcceptInvitation)
			protected.POST("/invitations/:token/decline", h.DeclineInvitation)

			// Usage & Billing
			protected.GET("/usage", h.GetUsageSummary)
			protected.GET("/usage/costs", h.GetCostBreakdown)

			// Global Domains (cross-service domain management)
			protected.GET("/domains", h.GetAllDomains)
			protected.GET("/domains/stats", h.GetDomainStats)

			// Activity (Audit Logs)
			protected.GET("/activity", h.GetActivity)
			protected.GET("/activity/actions", h.GetActivityActions)
			protected.GET("/activity/resource-types", h.GetActivityResourceTypes)

			// Observability (Metrics & Monitoring)
			protected.GET("/observability/metrics", h.GetMetricsSnapshot)
			protected.GET("/observability/metrics/history", h.GetMetricsHistory)
			protected.GET("/observability/health", h.GetServiceHealth)
			protected.GET("/observability/errors", h.GetRecentErrors)
			protected.GET("/observability/alerts", h.GetActiveAlerts)
		}
	}
}
