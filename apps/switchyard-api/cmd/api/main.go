package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/api"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/auth"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/builder"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cache"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/clients"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cloudflare"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/compliance"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/middleware"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/monitoring"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/provenance"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/reconciler"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/services"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/topology"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/validation"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatal("Failed to load configuration:", err)
	}

	// Set global logrus level based on config
	// This ensures components using logrus.StandardLogger() also respect the log level
	logrusLevel, err := logrus.ParseLevel(cfg.LogLevel.String())
	if err != nil {
		logrusLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logrusLevel)
	logrus.Infof("Log level set to: %s", logrusLevel.String())

	// Setup logging
	logger, err := logging.NewStructuredLogger(&logging.LogConfig{
		Level:       cfg.LogLevel.String(),
		Format:      "json",
		Output:      "stdout",
		ServiceName: "switchyard-api",
		Environment: cfg.Environment,
	})
	if err != nil {
		logrus.Fatal("Failed to initialize logger:", err)
	}

	// Connect to database
	database, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logrus.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Verify database connection
	if err := database.Ping(); err != nil {
		logrus.Fatal("Failed to ping database:", err)
	}

	// Run database migrations
	if err := db.Migrate(database, cfg.DatabaseURL); err != nil {
		logrus.Fatal("Failed to run database migrations:", err)
	}

	// Initialize repositories
	repos := db.NewRepositories(database)

	// Initialize cache service (needed for session revocation)
	cacheService, err := cache.NewRedisCache(&cache.CacheConfig{
		Host:         cfg.RedisHost,
		Port:         cfg.RedisPort,
		Password:     cfg.RedisPassword,
		DB:           0,
		MaxRetries:   3,
		PoolSize:     10,
		IdleTimeout:  5 * time.Minute,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		DefaultTTL:   cache.MediumTTL,
	})
	if err != nil {
		logrus.Warnf("Failed to initialize Redis cache: %v", err)
		logrus.Warn("Session revocation will not be available")
		cacheService = nil
	}

	// Initialize authentication manager
	// This will create either JWTManager (local mode) or OIDCManager (OIDC mode)
	// based on the ENCLII_AUTH_MODE configuration
	ctx := context.Background()
	authManager, err := auth.NewAuthManager(
		ctx,
		cfg,
		repos,
		cacheService, // Cache for session revocation (can be nil)
	)
	if err != nil {
		logrus.Fatal("Failed to initialize auth manager:", err)
	}

	// Log which authentication mode is active
	logrus.WithField("auth_mode", cfg.AuthMode).Info("✓ Authentication manager initialized")

	// Wire up API token validator for CLI/CI/CD authentication
	// This enables "enclii_xxx" tokens in addition to JWT/OIDC tokens
	switch am := authManager.(type) {
	case *auth.JWTManager:
		am.SetAPITokenValidator(repos.APITokens)
		logrus.Info("✓ API token authentication enabled (JWT mode)")
	case *auth.OIDCManager:
		am.SetAPITokenValidator(repos.APITokens)
		logrus.Info("✓ API token authentication enabled (OIDC mode)")
	default:
		logrus.Warn("⚠ API token authentication not configured (unknown auth manager type)")
	}

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.KubeConfig, cfg.KubeContext)
	if err != nil {
		logrus.Fatal("Failed to initialize Kubernetes client:", err)
	}

	// Initialize builder service
	builderService := builder.NewService(&builder.Config{
		WorkDir:          cfg.BuildWorkDir,
		Registry:         cfg.Registry,
		RegistryUsername: cfg.RegistryUsername,
		RegistryPassword: cfg.RegistryPassword,
		CacheDir:         cfg.BuildCacheDir,
		Timeout:          time.Duration(cfg.BuildTimeout) * time.Second,
		GenerateSBOM:     true, // Enable SBOM generation with Syft
		SignImages:       true, // Enable image signing with Cosign
	}, logrus.StandardLogger())

	// Ensure build directories exist
	if err := os.MkdirAll(cfg.BuildWorkDir, 0755); err != nil {
		logrus.Fatal("Failed to create build work directory:", err)
	}
	if err := os.MkdirAll(cfg.BuildCacheDir, 0755); err != nil {
		logrus.Warnf("Failed to create build cache directory (non-fatal): %v", err)
	}

	// Initialize reconciler
	reconcilerController := reconciler.NewController(database, repos, k8sClient, logrus.StandardLogger())

	// Start reconciliation controller (processes pending deployments from database)
	if err := reconcilerController.Start(ctx); err != nil {
		logrus.Fatal("Failed to start reconciler controller:", err)
	}
	logrus.Info("✓ Reconciliation controller started (processing pending deployments)")

	// Initialize service reconciler (also used directly by API handlers)
	serviceReconciler := reconciler.NewServiceReconciler(k8sClient, logrus.StandardLogger())

	// Initialize metrics collector
	metricsCollector := monitoring.NewMetricsCollector()

	// Initialize validator
	validatorInstance := validation.NewValidator()

	// Initialize provenance checker (PR approval verification)
	var provenanceChecker *provenance.Checker
	if cfg.GitHubToken != "" {
		provenanceChecker = provenance.NewChecker(cfg.GitHubToken, nil) // nil = use default policy
		logrus.Info("✓ PR approval checking enabled")
	} else {
		logrus.Warn("⚠ GitHub token not configured - PR approval checking disabled")
		logrus.Warn("   Set ENCLII_GITHUB_TOKEN to enable deployment approval verification")
	}

	// Initialize compliance exporter (Vanta/Drata webhooks)
	complianceExporter := compliance.NewExporter(&compliance.Config{
		Enabled:      cfg.ComplianceWebhooksEnabled,
		VantaWebhook: cfg.VantaWebhookURL,
		DrataWebhook: cfg.DrataWebhookURL,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
	}, logrus.StandardLogger())

	if cfg.ComplianceWebhooksEnabled {
		logrus.Info("✓ Compliance webhooks enabled")
		if cfg.VantaWebhookURL != "" {
			logrus.Info("  → Vanta webhook configured")
		}
		if cfg.DrataWebhookURL != "" {
			logrus.Info("  → Drata webhook configured")
		}
	} else {
		logrus.Info("ℹ Compliance webhooks disabled")
		logrus.Info("   Set ENCLII_COMPLIANCE_WEBHOOKS_ENABLED=true to enable")
	}

	// Initialize topology builder
	topologyBuilder := topology.NewGraphBuilder(repos, k8sClient, logrus.StandardLogger())
	logrus.Info("✓ Topology graph builder initialized")

	// Initialize service layer (business logic)
	// Note: AuthService currently only works with JWT local auth mode
	// TODO: Refactor to support both JWT and OIDC modes
	jwtManager, ok := authManager.(*auth.JWTManager)
	if !ok {
		logrus.Warn("⚠ AuthService not initialized - OIDC mode detected")
		logrus.Warn("   AuthService currently only supports JWT local auth mode")
	}
	authService := services.NewAuthService(
		repos,
		jwtManager, // May be nil in OIDC mode
		logrus.StandardLogger(),
	)
	if jwtManager != nil {
		logrus.Info("✓ AuthService initialized")
	}

	projectService := services.NewProjectService(
		repos,
		logrus.StandardLogger(),
	)
	logrus.Info("✓ ProjectService initialized")

	deploymentService := services.NewDeploymentService(
		repos,
		logrus.StandardLogger(),
	)
	logrus.Info("✓ DeploymentService initialized")

	deploymentGroupService := services.NewDeploymentGroupService(
		repos,
		deploymentService,
		logrus.StandardLogger(),
	)
	logrus.Info("✓ DeploymentGroupService initialized")

	// Initialize Roundhouse client (for async builds)
	var roundhouseClient *clients.RoundhouseClient
	if cfg.BuildMode == "roundhouse" {
		roundhouseClient = clients.NewRoundhouseClient(cfg.RoundhouseURL, cfg.RoundhouseAPIKey)
		logrus.WithFields(logrus.Fields{
			"build_mode":     "roundhouse",
			"roundhouse_url": cfg.RoundhouseURL,
		}).Info("✓ Build mode: roundhouse (async builds via worker)")
	} else {
		logrus.WithField("build_mode", "in-process").Info("✓ Build mode: in-process (sync builds in API)")
	}

	// Initialize Cloudflare client (for domain status sync)
	var cfClient *cloudflare.Client
	var domainSyncService *services.DomainSyncService
	if cfg.CloudflareAPIToken != "" && cfg.CloudflareAccountID != "" && cfg.CloudflareZoneID != "" {
		var cfErr error
		cfClient, cfErr = cloudflare.NewClient(&cloudflare.Config{
			APIToken:  cfg.CloudflareAPIToken,
			AccountID: cfg.CloudflareAccountID,
			ZoneID:    cfg.CloudflareZoneID,
			TunnelID:  cfg.CloudflareTunnelID,
		})
		if cfErr != nil {
			logrus.WithError(cfErr).Warn("⚠ Failed to initialize Cloudflare client")
		} else {
			// Verify token works
			if verifyErr := cfClient.VerifyToken(ctx); verifyErr != nil {
				logrus.WithError(verifyErr).Warn("⚠ Cloudflare API token verification failed")
				cfClient = nil
			} else {
				logrus.Info("✓ Cloudflare client initialized")

				// Create domain sync service
				domainSyncService = services.NewDomainSyncService(cfClient, repos, logrus.StandardLogger())
				logrus.Info("✓ Domain sync service initialized")
			}
		}
	} else {
		logrus.Info("ℹ Cloudflare integration not configured")
		logrus.Info("   Set ENCLII_CLOUDFLARE_API_TOKEN, ENCLII_CLOUDFLARE_ACCOUNT_ID, ENCLII_CLOUDFLARE_ZONE_ID to enable")
	}

	// Setup HTTP server
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Initialize security middleware with CORS support
	securityMiddleware := middleware.NewSecurityMiddleware(nil) // Uses default config with CORS
	router.Use(securityMiddleware.CORSMiddleware())
	logrus.Info("✓ CORS middleware enabled")

	// Setup API routes with all dependencies
	apiHandler := api.NewHandler(
		repos,
		cfg,
		authManager,
		cacheService,
		builderService,
		k8sClient,
		reconcilerController,
		serviceReconciler,
		metricsCollector,
		logger,
		validatorInstance,
		provenanceChecker,
		complianceExporter,
		topologyBuilder,
		// Service layer
		authService,
		projectService,
		deploymentService,
		deploymentGroupService,
		// Optional clients
		roundhouseClient,
	)

	// Wire up optional domain sync service (if Cloudflare is configured)
	if domainSyncService != nil {
		apiHandler.SetDomainSyncService(domainSyncService)
		logrus.Info("✓ Domain sync service wired to API handler")
	}

	api.SetupRoutes(router, apiHandler)

	server := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in goroutine
	go func() {
		logrus.Infof("🚂 Switchyard API starting on port %s", cfg.Port)
		logrus.Infof("   Environment: %s", cfg.Environment)
		logrus.Infof("   Registry: %s", cfg.Registry)
		logrus.Infof("   Build work dir: %s", cfg.BuildWorkDir)
		logrus.Infof("   Build cache dir: %s", cfg.BuildCacheDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatal("Failed to start server:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown:", err)
	}

	// Clean up resources
	// Stop domain sync service if running
	if domainSyncService != nil {
		domainSyncService.StopBackgroundSync()
		logrus.Info("Domain sync service stopped")
	}

	// Stop reconciler controller gracefully
	reconcilerController.Stop()
	logrus.Info("Reconciler controller stopped")

	if cacheService != nil {
		if err := cacheService.Close(); err != nil {
			logrus.Warnf("Error closing cache connection: %v", err)
		} else {
			logrus.Info("Cache connection closed")
		}
	}

	logrus.Info("Server exiting")
}
