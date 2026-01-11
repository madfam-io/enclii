package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/builder"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/config"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/queue"
	"go.uber.org/zap"
)

// Processor handles build job processing
type Processor struct {
	workerID   string
	queue      *queue.RedisQueue
	executor   *builder.Executor
	cfg        *config.Config
	logger     *zap.Logger
	httpClient *http.Client

	// Concurrency control
	semaphore chan struct{}
	wg        sync.WaitGroup
	shutdown  chan struct{}
}

// NewProcessor creates a new job processor
func NewProcessor(cfg *config.Config, q *queue.RedisQueue, logger *zap.Logger) *Processor {
	workerID := cfg.WorkerID
	if workerID == "" {
		hostname, _ := os.Hostname()
		workerID = fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])
	}

	p := &Processor{
		workerID:   workerID,
		queue:      q,
		cfg:        cfg,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		semaphore:  make(chan struct{}, cfg.MaxConcurrentBuilds),
		shutdown:   make(chan struct{}),
	}

	// Create executor with log callback
	p.executor = builder.NewExecutor(&builder.ExecutorConfig{
		WorkDir:      cfg.BuildWorkDir,
		Registry:     cfg.Registry,
		RegistryUser: cfg.RegistryUser,
		RegistryPass: cfg.RegistryPassword,
		GenerateSBOM: cfg.GenerateSBOM,
		SignImages:   cfg.SignImages,
		CosignKey:    cfg.CosignKey,
		Timeout:      cfg.BuildTimeout,
	}, logger, func(jobID uuid.UUID, line string) {
		// Append log to Redis stream
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.AppendLog(ctx, jobID, line)
	})

	return p
}

// Start begins processing jobs
func (p *Processor) Start(ctx context.Context) error {
	p.logger.Info("worker starting",
		zap.String("worker_id", p.workerID),
		zap.Int("max_concurrent", p.cfg.MaxConcurrentBuilds),
	)

	// Register worker
	if err := p.queue.RegisterWorker(ctx, p.workerID); err != nil {
		p.logger.Warn("failed to register worker", zap.Error(err))
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		p.logger.Info("shutdown signal received, waiting for builds to complete...")
		close(p.shutdown)
	}()

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			return p.gracefulShutdown()
		case <-p.shutdown:
			return p.gracefulShutdown()
		default:
			// Try to acquire semaphore (non-blocking check for shutdown)
			select {
			case p.semaphore <- struct{}{}:
				// Got a slot, try to get a job
				job, err := p.queue.Dequeue(ctx, p.cfg.PollInterval)
				if err != nil {
					p.logger.Error("failed to dequeue", zap.Error(err))
					<-p.semaphore // Release slot
					time.Sleep(time.Second)
					continue
				}

				if job == nil {
					// No job available
					<-p.semaphore
					continue
				}

				// Process job in goroutine
				p.wg.Add(1)
				go func(j *queue.BuildJob) {
					defer p.wg.Done()
					defer func() { <-p.semaphore }()
					p.processJob(ctx, j)
				}(job)

			case <-p.shutdown:
				return p.gracefulShutdown()
			}
		}
	}
}

func (p *Processor) processJob(ctx context.Context, job *queue.BuildJob) {
	logger := p.logger.With(
		zap.String("job_id", job.ID.String()),
		zap.String("service_id", job.ServiceID.String()),
		zap.String("git_sha", job.GitSHA[:8]),
	)

	logger.Info("processing job")

	// Update status to building
	if err := p.queue.UpdateStatus(ctx, job.ID, queue.StatusBuilding, p.workerID); err != nil {
		logger.Error("failed to update status", zap.Error(err))
	}

	// Create build context with timeout
	buildCtx, cancel := context.WithTimeout(ctx, p.cfg.BuildTimeout)
	defer cancel()

	// Execute build
	result, err := p.executor.Execute(buildCtx, job)

	// Update final status
	var finalStatus queue.JobStatus
	if err != nil || !result.Success {
		finalStatus = queue.StatusFailed
		logger.Error("build failed",
			zap.String("error", result.ErrorMessage),
			zap.Float64("duration_secs", result.DurationSecs),
		)
	} else {
		finalStatus = queue.StatusCompleted
		logger.Info("build completed",
			zap.String("image_uri", result.ImageURI),
			zap.Float64("duration_secs", result.DurationSecs),
		)
	}

	// Store result
	if err := p.queue.SetResult(ctx, job.ID, result); err != nil {
		logger.Error("failed to store result", zap.Error(err))
	}

	// Update status
	if err := p.queue.UpdateStatus(ctx, job.ID, finalStatus, p.workerID); err != nil {
		logger.Error("failed to update final status", zap.Error(err))
	}

	// Send callback to Switchyard
	if job.CallbackURL != "" {
		if err := p.sendCallback(ctx, job.CallbackURL, result); err != nil {
			logger.Error("failed to send callback", zap.Error(err))
		}
	}
}

func (p *Processor) sendCallback(ctx context.Context, url string, result *queue.BuildResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.cfg.SwitchyardAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.SwitchyardAPIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("callback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}

	p.logger.Info("callback sent successfully",
		zap.String("url", url),
		zap.String("job_id", result.JobID.String()),
	)

	return nil
}

func (p *Processor) gracefulShutdown() error {
	p.logger.Info("waiting for active builds to complete...")

	// Wait for all active jobs with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("all builds completed")
	case <-time.After(5 * time.Minute):
		p.logger.Warn("shutdown timeout, some builds may be interrupted")
	}

	// Unregister worker
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.queue.UnregisterWorker(ctx, p.workerID); err != nil {
		p.logger.Warn("failed to unregister worker", zap.Error(err))
	}

	return nil
}

// Stats returns current worker statistics
func (p *Processor) Stats() map[string]interface{} {
	return map[string]interface{}{
		"worker_id":       p.workerID,
		"max_concurrent":  p.cfg.MaxConcurrentBuilds,
		"active_builds":   len(p.semaphore),
		"available_slots": p.cfg.MaxConcurrentBuilds - len(p.semaphore),
	}
}
