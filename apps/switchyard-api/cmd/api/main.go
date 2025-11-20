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

	"github.com/madfam/enclii/apps/switchyard-api/internal/api"
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
	"github.com/madfam/enclii/apps/switchyard-api/internal/topology"
	"github.com/madfam/enclii/apps/switchyard-api/internal/validation"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatal("Failed to load configuration:", err)
	}

	// Setup logging
	logrus.SetLevel(cfg.LogLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logger := logging.NewLogrusLogger(logrus.StandardLogger())

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

	// Initialize authentication with session revocation support
	authManager, err := auth.NewJWTManager(
		15*time.Minute, // Access token duration
		7*24*time.Hour, // Refresh token duration (7 days)
		repos,          // Database repositories for authorization
		cacheService,   // Cache for session revocation (can be nil)
	)
	if err != nil {
		logrus.Fatal("Failed to initialize auth manager:", err)
	}

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.KubeConfig, cfg.KubeContext)
	if err != nil {
		logrus.Fatal("Failed to initialize Kubernetes client:", err)
	}

	// Initialize builder service
	builderService := builder.NewService(&builder.Config{
		WorkDir:      cfg.BuildWorkDir,
		Registry:     cfg.Registry,
		CacheDir:     cfg.BuildCacheDir,
		Timeout:      time.Duration(cfg.BuildTimeout) * time.Second,
		GenerateSBOM: true, // Enable SBOM generation with Syft
		SignImages:   true, // Enable image signing with Cosign
	}, logrus.StandardLogger())

	// Ensure build directories exist
	if err := os.MkdirAll(cfg.BuildWorkDir, 0755); err != nil {
		logrus.Fatal("Failed to create build work directory:", err)
	}
	if err := os.MkdirAll(cfg.BuildCacheDir, 0755); err != nil {
		logrus.Warnf("Failed to create build cache directory (non-fatal): %v", err)
	}

	// Initialize reconciler
	reconcilerController := reconciler.NewController(k8sClient, repos, logger)

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

	// Setup HTTP server
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Setup API routes with all dependencies
	apiHandler := api.NewHandler(
		repos,
		cfg,
		authManager,
		cacheService,
		builderService,
		k8sClient,
		reconcilerController,
		metricsCollector,
		logger,
		validatorInstance,
		provenanceChecker,
		complianceExporter,
		topologyBuilder,
	)
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
	if cacheService != nil {
		if err := cacheService.Close(); err != nil {
			logrus.Warnf("Error closing cache connection: %v", err)
		} else {
			logrus.Info("Cache connection closed")
		}
	}

	logrus.Info("Server exiting")
}
