package main

import (
	"context"
	"log"
	"os"

	"github.com/madfam-org/enclii/apps/roundhouse/internal/config"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/queue"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/worker"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Validate required config
	if cfg.RedisURL == "" {
		logger.Fatal("REDIS_URL is required")
	}
	if cfg.Registry == "" {
		logger.Fatal("REGISTRY is required")
	}

	// Initialize Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL, logger)
	if err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redisQueue.Close()

	logger.Info("connected to Redis")

	// Ensure build work directory exists
	if err := os.MkdirAll(cfg.BuildWorkDir, 0755); err != nil {
		logger.Fatal("failed to create build work directory", zap.Error(err))
	}

	// Create and start processor
	processor := worker.NewProcessor(cfg, redisQueue, logger)

	logger.Info("starting Roundhouse worker",
		zap.String("work_dir", cfg.BuildWorkDir),
		zap.Int("max_concurrent", cfg.MaxConcurrentBuilds),
		zap.Duration("timeout", cfg.BuildTimeout),
	)

	ctx := context.Background()
	if err := processor.Start(ctx); err != nil {
		logger.Fatal("worker failed", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("worker shutdown complete")
}
