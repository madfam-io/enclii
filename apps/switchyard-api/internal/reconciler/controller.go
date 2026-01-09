package reconciler

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Controller manages the reconciliation loop for all deployments
type Controller struct {
	db                *sql.DB
	repositories      *db.Repositories
	serviceReconciler *ServiceReconciler
	logger            *logrus.Logger

	// Control channels
	stopCh   chan struct{}
	workCh   chan *ReconcileWork
	resultCh chan *ReconcileWorkResult

	// Worker management
	workers int
	wg      sync.WaitGroup
	started bool
	mu      sync.RWMutex
}

type ReconcileWork struct {
	DeploymentID string
	Priority     int
	Attempt      int
	ScheduledAt  time.Time
}

type ReconcileWorkResult struct {
	Work   *ReconcileWork
	Result *ReconcileResult
}

func NewController(database *sql.DB, repositories *db.Repositories, k8sClient *k8s.Client, logger *logrus.Logger) *Controller {
	return &Controller{
		db:                database,
		repositories:      repositories,
		serviceReconciler: NewServiceReconciler(k8sClient, logger),
		logger:            logger,
		stopCh:            make(chan struct{}),
		workCh:            make(chan *ReconcileWork, 100),
		resultCh:          make(chan *ReconcileWorkResult, 100),
		workers:           5, // Number of concurrent reconcilers
	}
}

// Start begins the reconciliation controller
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return fmt.Errorf("controller already started")
	}

	c.started = true
	c.logger.Info("Starting reconciliation controller")

	// Start worker goroutines
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}

	// Start result processor
	c.wg.Add(1)
	go c.resultProcessor(ctx)

	// Start work scheduler
	c.wg.Add(1)
	go c.workScheduler(ctx)

	c.logger.WithField("workers", c.workers).Info("Reconciliation controller started")
	return nil
}

// Stop gracefully shuts down the controller
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return
	}

	c.logger.Info("Stopping reconciliation controller")
	close(c.stopCh)
	c.wg.Wait()
	c.started = false
	c.logger.Info("Reconciliation controller stopped")
}

// ScheduleReconciliation adds a deployment to the reconciliation queue
func (c *Controller) ScheduleReconciliation(deploymentID string, priority int) {
	work := &ReconcileWork{
		DeploymentID: deploymentID,
		Priority:     priority,
		Attempt:      1,
		ScheduledAt:  time.Now(),
	}

	select {
	case c.workCh <- work:
		c.logger.WithFields(logrus.Fields{
			"deployment": deploymentID,
			"priority":   priority,
		}).Debug("Scheduled reconciliation work")
	default:
		c.logger.WithField("deployment", deploymentID).Warn("Work queue full, dropping reconciliation request")
	}
}

// worker processes reconciliation work
func (c *Controller) worker(ctx context.Context, workerID int) {
	defer c.wg.Done()

	logger := c.logger.WithField("worker", workerID)
	logger.Debug("Starting reconciliation worker")

	for {
		select {
		case <-c.stopCh:
			logger.Debug("Worker stopping")
			return
		case <-ctx.Done():
			logger.Debug("Worker context cancelled")
			return
		case work := <-c.workCh:
			result := c.processWork(ctx, work, logger)

			select {
			case c.resultCh <- &ReconcileWorkResult{Work: work, Result: result}:
			case <-c.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

// processWork handles a single reconciliation task
func (c *Controller) processWork(ctx context.Context, work *ReconcileWork, logger *logrus.Entry) *ReconcileResult {
	logger = logger.WithFields(logrus.Fields{
		"deployment": work.DeploymentID,
		"attempt":    work.Attempt,
	})

	logger.Info("Processing reconciliation work")
	start := time.Now()

	// Get deployment details from database
	deployment, err := c.repositories.Deployments.GetByID(ctx, work.DeploymentID)
	if err != nil {
		logger.WithError(err).Error("Failed to get deployment")
		return &ReconcileResult{
			Success: false,
			Message: "Failed to retrieve deployment",
			Error:   err,
		}
	}

	// Get associated release first
	release, err := c.repositories.Releases.GetByID(deployment.ReleaseID)
	if err != nil {
		logger.WithError(err).Error("Failed to get release")
		return &ReconcileResult{
			Success: false,
			Message: "Failed to retrieve release",
			Error:   err,
		}
	}

	// Get service from release
	service, err := c.repositories.Services.GetByID(release.ServiceID)
	if err != nil {
		logger.WithError(err).Error("Failed to get service")
		return &ReconcileResult{
			Success: false,
			Message: "Failed to retrieve service",
			Error:   err,
		}
	}

	// Get environment to determine the target Kubernetes namespace
	environment, err := c.repositories.Environments.GetByID(ctx, deployment.EnvironmentID)
	if err != nil {
		logger.WithError(err).Error("Failed to get environment")
		return &ReconcileResult{
			Success: false,
			Message: "Failed to retrieve environment",
			Error:   err,
		}
	}

	// Get environment variables (decrypted) for this service and environment
	var envVars map[string]string
	if c.repositories.EnvVars != nil {
		envVars, err = c.repositories.EnvVars.GetDecrypted(ctx, service.ID, deployment.EnvironmentID)
		if err != nil {
			logger.WithError(err).Warn("Failed to get environment variables, continuing without them")
			envVars = make(map[string]string)
		}
	} else {
		envVars = make(map[string]string)
	}

	// Create reconcile request
	req := &ReconcileRequest{
		Service:     service,
		Release:     release,
		Deployment:  deployment,
		Environment: environment,
		EnvVars:     envVars,
	}

	// Perform reconciliation
	result := c.serviceReconciler.Reconcile(ctx, req)

	duration := time.Since(start)
	logger.WithFields(logrus.Fields{
		"success":  result.Success,
		"duration": duration,
		"message":  result.Message,
	}).Info("Completed reconciliation work")

	return result
}

// resultProcessor handles reconciliation results
func (c *Controller) resultProcessor(ctx context.Context) {
	defer c.wg.Done()

	logger := c.logger.WithField("component", "result-processor")
	logger.Debug("Starting result processor")

	for {
		select {
		case <-c.stopCh:
			logger.Debug("Result processor stopping")
			return
		case <-ctx.Done():
			logger.Debug("Result processor context cancelled")
			return
		case workResult := <-c.resultCh:
			c.handleResult(ctx, workResult, logger)
		}
	}
}

// handleResult processes a reconciliation result
func (c *Controller) handleResult(ctx context.Context, workResult *ReconcileWorkResult, logger *logrus.Entry) {
	work := workResult.Work
	result := workResult.Result

	logger = logger.WithFields(logrus.Fields{
		"deployment": work.DeploymentID,
		"success":    result.Success,
	})

	// Update deployment status in database
	var status types.DeploymentStatus
	var health types.HealthStatus

	if result.Success {
		status = types.DeploymentStatusRunning
		health = types.HealthStatusHealthy
		logger.Info("Deployment reconciled successfully")
	} else {
		if result.NextCheck != nil {
			// Retry later
			status = types.DeploymentStatusPending
			health = types.HealthStatusUnknown

			// Schedule retry
			retryWork := &ReconcileWork{
				DeploymentID: work.DeploymentID,
				Priority:     work.Priority,
				Attempt:      work.Attempt + 1,
				ScheduledAt:  *result.NextCheck,
			}

			go func() {
				time.Sleep(time.Until(*result.NextCheck))
				select {
				case c.workCh <- retryWork:
				case <-c.stopCh:
				}
			}()

			logger.WithField("next_check", result.NextCheck).Info("Scheduled reconciliation retry")
		} else {
			// Failed permanently
			status = types.DeploymentStatusFailed
			health = types.HealthStatusUnhealthy
			logger.WithError(result.Error).Error("Deployment reconciliation failed")
		}
	}

	// Update deployment in database
	deploymentUUID, err := uuid.Parse(work.DeploymentID)
	if err != nil {
		logger.WithError(err).Error("Failed to parse deployment ID")
		return
	}
	err = c.repositories.Deployments.UpdateStatus(deploymentUUID, status, health)
	if err != nil {
		logger.WithError(err).Error("Failed to update deployment status")
	}
}

// workScheduler periodically checks for pending deployments
func (c *Controller) workScheduler(ctx context.Context) {
	defer c.wg.Done()

	logger := c.logger.WithField("component", "work-scheduler")
	logger.Debug("Starting work scheduler")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			logger.Debug("Work scheduler stopping")
			return
		case <-ctx.Done():
			logger.Debug("Work scheduler context cancelled")
			return
		case <-ticker.C:
			c.schedulePendingWork(ctx, logger)
		}
	}
}

// schedulePendingWork finds and schedules pending deployments
func (c *Controller) schedulePendingWork(ctx context.Context, logger *logrus.Entry) {
	// Get pending deployments
	deployments, err := c.repositories.Deployments.GetByStatus(ctx, types.DeploymentStatusPending)
	if err != nil {
		logger.WithError(err).Error("Failed to get pending deployments")
		return
	}

	for _, deployment := range deployments {
		// Calculate priority based on age
		age := time.Since(deployment.CreatedAt)
		priority := int(age.Minutes()) // Older deployments get higher priority

		select {
		case c.workCh <- &ReconcileWork{
			DeploymentID: deployment.ID.String(),
			Priority:     priority,
			Attempt:      1,
			ScheduledAt:  time.Now(),
		}:
			logger.WithFields(logrus.Fields{
				"deployment": deployment.ID,
				"age":        age,
				"priority":   priority,
			}).Debug("Scheduled pending deployment")
		default:
			logger.WithField("deployment", deployment.ID).Debug("Work queue full, skipping deployment")
		}
	}

	if len(deployments) > 0 {
		logger.WithField("count", len(deployments)).Debug("Scheduled pending deployments")
	}
}

// GetStatus returns the current status of the controller
func (c *Controller) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"started":      c.started,
		"workers":      c.workers,
		"work_queue":   len(c.workCh),
		"result_queue": len(c.resultCh),
	}
}

// Health check for the controller
func (c *Controller) HealthCheck() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.started {
		return fmt.Errorf("controller not started")
	}

	// Check if work channels are functional
	if len(c.workCh) == cap(c.workCh) {
		return fmt.Errorf("work queue is full")
	}

	if len(c.resultCh) == cap(c.resultCh) {
		return fmt.Errorf("result queue is full")
	}

	return nil
}
