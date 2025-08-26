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
	"github.com/spf13/viper"

	"github.com/madfam/enclii/apps/switchyard-api/internal/api"
	"github.com/madfam/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
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

	// Connect to database
	database, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logrus.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	// Run database migrations
	if err := db.Migrate(database, cfg.DatabaseURL); err != nil {
		logrus.Fatal("Failed to run database migrations:", err)
	}

	// Initialize repositories
	repos := db.NewRepositories(database)

	// Setup HTTP server
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Setup API routes
	apiHandler := api.NewHandler(repos, cfg)
	api.SetupRoutes(router, apiHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		logrus.Infof("🚂 Switchyard API starting on port %s", cfg.Port)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown:", err)
	}

	logrus.Info("Server exiting")
}