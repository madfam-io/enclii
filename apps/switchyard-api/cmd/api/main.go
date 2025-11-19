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
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/apps/switchyard-api/internal/monitoring"
	"github.com/madfam/enclii/apps/switchyard-api/internal/reconciler"
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

	// Initialize authentication
	authManager, err := auth.NewJWTManager(
		15*time.Minute, // Access token duration
		7*24*time.Hour, // Refresh token duration (7 days)
		repos,          // Database repositories for authorization
	)
	if err != nil {
		logrus.Fatal("Failed to initialize auth manager:", err)
	}

	// Initialize cache service
	cacheService, err := cache.NewRedisCache(&cache.RedisConfig{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err != nil {
		logrus.Warnf("Failed to initialize Redis cache, using in-memory cache: %v", err)
		cacheService = cache.NewInMemoryCache()
	}

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(cfg.KubeConfig, cfg.KubeContext)
	if err != nil {
		logrus.Fatal("Failed to initialize Kubernetes client:", err)
	}

	// Initialize builder service
	builderService := builder.NewService(&builder.Config{
		WorkDir:  cfg.BuildWorkDir,
		Registry: cfg.Registry,
		CacheDir: cfg.BuildCacheDir,
		Timeout:  time.Duration(cfg.BuildTimeout) * time.Second,
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

	logrus.Info("Server exiting")
}