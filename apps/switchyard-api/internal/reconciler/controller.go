package reconciler

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/notifications"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// Controller manages the reconciliation loop for all deployments
type Controller struct {
	db                *sql.DB
	repositories      *db.Repositories
	serviceReconciler *ServiceReconciler
	k8sClient         *k8s.Client
	logger            *logrus.Logger

	// Notification service for webhooks (optional)
	notificationService *notifications.Service

	// Control channels
	stopCh   chan struct{}
	workCh   chan *ReconcileWork
	resultCh chan *ReconcileWorkResult

	// Worker management
	workers int
	wg      sync.WaitGroup
	started bool
	mu      sync.RWMutex

	// Backpressure tracking
	droppedWork int64            // Atomic counter for dropped work items
	retryQueue  []*ReconcileWork // Items that need to be retried when queue has space
	retryMu     sync.Mutex       // Protects retryQueue
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

// QueuePressure tracks work queue metrics
type QueuePressure struct {
	QueueSize     int
	QueueCapacity int
	DroppedWork   int64
	RetryQueue    int
}

// ErrQueueFull is returned when the work queue cannot accept more work
var ErrQueueFull = fmt.Errorf("work queue is full")

func NewController(database *sql.DB, repositories *db.Repositories, k8sClient *k8s.Client, logger *logrus.Logger) *Controller {
	return &Controller{
		db:                database,
		repositories:      repositories,
		serviceReconciler: NewServiceReconciler(k8sClient, logger),
		k8sClient:         k8sClient,
		logger:            logger,
		stopCh:            make(chan struct{}),
		workCh:            make(chan *ReconcileWork, 100),
		resultCh:          make(chan *ReconcileWorkResult, 100),
		workers:           5, // Number of concurrent reconcilers
	}
}

// SetNotificationService sets the notification service for sending webhook events
func (c *Controller) SetNotificationService(svc *notifications.Service) {
	c.notificationService = svc
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

	// Start K8s→DB sync job (runs every 60 seconds)
	c.wg.Add(1)
	go c.k8sSyncScheduler(ctx)

	// Start retry queue processor (drains retry queue when work queue has space)
	c.wg.Add(1)
	go c.retryQueueProcessor(ctx)

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

// ScheduleReconciliation adds a deployment to the reconciliation queue.
// Returns ErrQueueFull if the queue cannot accept the work.
func (c *Controller) ScheduleReconciliation(deploymentID string, priority int) error {
	work := &ReconcileWork{
		DeploymentID: deploymentID,
		Priority:     priority,
		Attempt:      1,
		ScheduledAt:  time.Now(),
	}

	return c.enqueueWork(work)
}

// enqueueWork attempts to add work to the queue, with retry queue fallback
func (c *Controller) enqueueWork(work *ReconcileWork) error {
	select {
	case c.workCh <- work:
		c.logger.WithFields(logrus.Fields{
			"deployment": work.DeploymentID,
			"priority":   work.Priority,
			"attempt":    work.Attempt,
		}).Debug("Scheduled reconciliation work")
		return nil
	default:
		// Queue is full - add to retry queue with backpressure tracking
		atomic.AddInt64(&c.droppedWork, 1)

		c.retryMu.Lock()
		c.retryQueue = append(c.retryQueue, work)
		retryLen := len(c.retryQueue)
		c.retryMu.Unlock()

		c.logger.WithFields(logrus.Fields{
			"deployment":       work.DeploymentID,
			"retry_queue_size": retryLen,
			"dropped_total":    atomic.LoadInt64(&c.droppedWork),
		}).Warn("Work queue full, added to retry queue")

		return ErrQueueFull
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
	// Use GetDecryptedWithMeta to properly separate secrets from regular env vars
	var envVars map[string]string
	var envVarsWithMeta []EnvVarWithMeta
	if c.repositories.EnvVars != nil {
		// Try the new metadata-aware method first
		dbEnvVars, err := c.repositories.EnvVars.GetDecryptedWithMeta(ctx, service.ID, deployment.EnvironmentID)
		if err != nil {
			logger.WithError(err).Warn("Failed to get environment variables with metadata, falling back to legacy")
			envVars, _ = c.repositories.EnvVars.GetDecrypted(ctx, service.ID, deployment.EnvironmentID)
		} else {
			// Convert db.EnvVarWithMeta to reconciler.EnvVarWithMeta
			envVarsWithMeta = make([]EnvVarWithMeta, len(dbEnvVars))
			envVars = make(map[string]string) // Also populate legacy map for backwards compatibility
			for i, ev := range dbEnvVars {
				envVarsWithMeta[i] = EnvVarWithMeta{
					Key:      ev.Key,
					Value:    ev.Value,
					IsSecret: ev.IsSecret,
				}
				envVars[ev.Key] = ev.Value
			}
		}
	} else {
		envVars = make(map[string]string)
	}

	// Get database addon bindings for this service
	var addonBindings []AddonBinding
	if c.repositories.DatabaseAddons != nil {
		bindings, err := c.repositories.DatabaseAddons.GetBindingsByService(ctx, service.ID)
		if err != nil {
			logger.WithError(err).Warn("Failed to get addon bindings, continuing without them")
		} else {
			for _, binding := range bindings {
				// Get the addon details
				addon, err := c.repositories.DatabaseAddons.GetByID(ctx, binding.AddonID)
				if err != nil {
					logger.WithError(err).WithField("addon_id", binding.AddonID).Warn("Failed to get addon for binding")
					continue
				}
				// Only include ready addons
				if addon.Status != types.DatabaseAddonStatusReady {
					logger.WithFields(logrus.Fields{
						"addon_id": addon.ID,
						"status":   addon.Status,
					}).Debug("Skipping non-ready addon binding")
					continue
				}
				addonBindings = append(addonBindings, AddonBinding{
					EnvVarName:       binding.EnvVarName,
					AddonType:        addon.Type,
					K8sNamespace:     addon.K8sNamespace,
					K8sResourceName:  addon.K8sResourceName,
					ConnectionSecret: addon.ConnectionSecret,
				})
			}
		}
	}

	// Create reconcile request
	req := &ReconcileRequest{
		Service:         service,
		Release:         release,
		Deployment:      deployment,
		Environment:     environment,
		EnvVars:         envVars,
		EnvVarsWithMeta: envVarsWithMeta,
		AddonBindings:   addonBindings,
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

			// Schedule retry with proper backpressure handling
			retryWork := &ReconcileWork{
				DeploymentID: work.DeploymentID,
				Priority:     work.Priority + 1, // Increase priority for retries
				Attempt:      work.Attempt + 1,
				ScheduledAt:  *result.NextCheck,
			}

			go func() {
				time.Sleep(time.Until(*result.NextCheck))
				select {
				case <-c.stopCh:
					return
				default:
				}
				if err := c.enqueueWork(retryWork); err != nil {
					// Work was added to retry queue, will be processed later
					c.logger.WithFields(logrus.Fields{
						"deployment": retryWork.DeploymentID,
						"attempt":    retryWork.Attempt,
					}).Debug("Retry scheduled to retry queue")
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

	// Store error message for failed deployments
	var errorMsg *string
	if status == types.DeploymentStatusFailed && result.Error != nil {
		errStr := result.Error.Error()
		errorMsg = &errStr
	}
	err = c.repositories.Deployments.UpdateStatusWithError(deploymentUUID, status, health, errorMsg)
	if err != nil {
		logger.WithError(err).Error("Failed to update deployment status")
	}

	// Send webhook notifications for final states (success or permanent failure)
	if c.notificationService != nil && (status == types.DeploymentStatusRunning || status == types.DeploymentStatusFailed) {
		go c.sendDeploymentNotification(ctx, deploymentUUID, status, result)
	}
}

// sendDeploymentNotification sends webhook notifications for deployment status changes
func (c *Controller) sendDeploymentNotification(ctx context.Context, deploymentID uuid.UUID, status types.DeploymentStatus, result *ReconcileResult) {
	logger := c.logger.WithFields(logrus.Fields{
		"deployment_id": deploymentID,
		"status":        status,
	})

	// Get deployment details
	deployment, err := c.repositories.Deployments.GetByID(ctx, deploymentID.String())
	if err != nil {
		logger.WithError(err).Error("Failed to get deployment for notification")
		return
	}

	// Get release
	release, err := c.repositories.Releases.GetByID(deployment.ReleaseID)
	if err != nil {
		logger.WithError(err).Error("Failed to get release for notification")
		return
	}

	// Get service
	service, err := c.repositories.Services.GetByID(release.ServiceID)
	if err != nil {
		logger.WithError(err).Error("Failed to get service for notification")
		return
	}

	// Get project
	project, err := c.repositories.Projects.GetByID(ctx, service.ProjectID)
	if err != nil {
		logger.WithError(err).Error("Failed to get project for notification")
		return
	}

	// Get environment
	environment, err := c.repositories.Environments.GetByID(ctx, deployment.EnvironmentID)
	if err != nil {
		logger.WithError(err).Error("Failed to get environment for notification")
		return
	}

	// Determine event type
	var eventType types.WebhookEventType
	if status == types.DeploymentStatusRunning {
		eventType = types.WebhookEventDeploymentSucceeded
	} else {
		eventType = types.WebhookEventDeploymentFailed
	}

	// Build webhook event
	event := &types.WebhookEvent{
		ID:        uuid.New(),
		Type:      eventType,
		Timestamp: time.Now(),
		ProjectID: project.ID,
		Project: types.WebhookProjectInfo{
			ID:   project.ID,
			Name: project.Name,
			Slug: project.Slug,
		},
		Deployment: &types.WebhookDeploymentInfo{
			ID:          deployment.ID,
			ServiceName: service.Name,
			Environment: environment.Name,
			Status:      string(status),
			CommitSHA:   release.GitSHA,
		},
	}

	// Add error message for failed deployments
	if status == types.DeploymentStatusFailed && result != nil && result.Error != nil {
		event.Deployment.Error = result.Error.Error()
	}

	// Send notification
	if err := c.notificationService.SendEvent(ctx, project.ID, event); err != nil {
		logger.WithError(err).Error("Failed to send deployment notification")
	} else {
		logger.Info("Deployment notification sent")
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

		work := &ReconcileWork{
			DeploymentID: deployment.ID.String(),
			Priority:     priority,
			Attempt:      1,
			ScheduledAt:  time.Now(),
		}

		if err := c.enqueueWork(work); err != nil {
			// Added to retry queue, will be processed when queue has space
			logger.WithFields(logrus.Fields{
				"deployment":       deployment.ID,
				"retry_queue_size": len(c.retryQueue),
			}).Debug("Pending deployment added to retry queue")
		} else {
			logger.WithFields(logrus.Fields{
				"deployment": deployment.ID,
				"age":        age,
				"priority":   priority,
			}).Debug("Scheduled pending deployment")
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

	c.retryMu.Lock()
	retryQueueLen := len(c.retryQueue)
	c.retryMu.Unlock()

	return map[string]interface{}{
		"started":            c.started,
		"workers":            c.workers,
		"work_queue":         len(c.workCh),
		"work_queue_cap":     cap(c.workCh),
		"result_queue":       len(c.resultCh),
		"result_queue_cap":   cap(c.resultCh),
		"retry_queue":        retryQueueLen,
		"dropped_work_total": atomic.LoadInt64(&c.droppedWork),
	}
}

// GetQueuePressure returns detailed backpressure metrics
func (c *Controller) GetQueuePressure() QueuePressure {
	c.retryMu.Lock()
	retryQueueLen := len(c.retryQueue)
	c.retryMu.Unlock()

	return QueuePressure{
		QueueSize:     len(c.workCh),
		QueueCapacity: cap(c.workCh),
		DroppedWork:   atomic.LoadInt64(&c.droppedWork),
		RetryQueue:    retryQueueLen,
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

// retryQueueProcessor drains the retry queue when the work queue has space
func (c *Controller) retryQueueProcessor(ctx context.Context) {
	defer c.wg.Done()

	logger := c.logger.WithField("component", "retry-processor")
	logger.Debug("Starting retry queue processor")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			logger.Debug("Retry queue processor stopping")
			return
		case <-ctx.Done():
			logger.Debug("Retry queue processor context cancelled")
			return
		case <-ticker.C:
			c.drainRetryQueue(logger)
		}
	}
}

// drainRetryQueue attempts to move items from retry queue to work queue
func (c *Controller) drainRetryQueue(logger *logrus.Entry) {
	c.retryMu.Lock()
	if len(c.retryQueue) == 0 {
		c.retryMu.Unlock()
		return
	}

	// Check if work queue has space (at least 20% free)
	workQueueFree := cap(c.workCh) - len(c.workCh)
	if workQueueFree < cap(c.workCh)/5 {
		c.retryMu.Unlock()
		return
	}

	// Move items from retry queue to work queue
	toMove := len(c.retryQueue)
	if toMove > workQueueFree {
		toMove = workQueueFree
	}

	moved := 0
	remaining := make([]*ReconcileWork, 0, len(c.retryQueue)-toMove)

	for i, work := range c.retryQueue {
		if i < toMove {
			select {
			case c.workCh <- work:
				moved++
			default:
				// Queue filled up, keep remaining items
				remaining = append(remaining, c.retryQueue[i:]...)
				break
			}
		} else {
			remaining = append(remaining, work)
		}
	}

	c.retryQueue = remaining
	c.retryMu.Unlock()

	if moved > 0 {
		logger.WithFields(logrus.Fields{
			"moved":     moved,
			"remaining": len(remaining),
		}).Debug("Drained retry queue to work queue")
	}
}

// k8sSyncScheduler runs the K8s→DB sync job every 60 seconds
func (c *Controller) k8sSyncScheduler(ctx context.Context) {
	defer c.wg.Done()

	logger := c.logger.WithField("component", "k8s-sync")
	logger.Info("Starting K8s→DB sync scheduler")

	// Run initial sync immediately
	c.runK8sSync(ctx, logger)

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			logger.Debug("K8s sync scheduler stopping")
			return
		case <-ctx.Done():
			logger.Debug("K8s sync scheduler context cancelled")
			return
		case <-ticker.C:
			c.runK8sSync(ctx, logger)
		}
	}
}

// runK8sSync synchronizes K8s deployment state to database
func (c *Controller) runK8sSync(ctx context.Context, logger *logrus.Entry) {
	if c.k8sClient == nil {
		logger.Debug("K8s client not available, skipping sync")
		return
	}

	// Dynamic namespace discovery: Get all unique namespaces from environments table
	// This ensures new environment patterns (e.g., enclii-{project_slug}-{env_name}) are monitored
	envs, err := c.repositories.Environments.ListAll()
	if err != nil {
		logger.WithError(err).Error("Failed to list environments for namespace sync")
		return
	}

	// Build unique namespace set including core namespaces
	namespaceSet := make(map[string]bool)
	// Always include core namespaces
	for _, ns := range []string{"enclii", "janua", "data", "monitoring"} {
		namespaceSet[ns] = true
	}
	// Add all environment namespaces from database
	for _, env := range envs {
		if env.KubeNamespace != "" {
			namespaceSet[env.KubeNamespace] = true
		}
	}
	// Convert set to slice
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	logger.WithField("namespace_count", len(namespaces)).Debug("Syncing K8s deployments from namespaces")

	for _, ns := range namespaces {
		deployments, err := c.k8sClient.ListDeployments(ctx, ns)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"namespace": ns,
				"error":     err,
			}).Warn("Failed to list K8s deployments, skipping namespace")
			continue
		}

		for _, dep := range deployments {
			c.syncDeploymentToDatabase(ctx, ns, dep, logger)
		}
	}
}

// syncDeploymentToDatabase checks if a K8s deployment has corresponding DB records
func (c *Controller) syncDeploymentToDatabase(ctx context.Context, namespace string, k8sDep appsv1.Deployment, logger *logrus.Entry) {
	deploymentName := k8sDep.Name

	// 1. Find matching service by name (skip if not registered)
	service, err := c.repositories.Services.GetByName(deploymentName)
	if err != nil {
		// Service not registered in database, skip silently
		// (This is expected for system deployments like ingress, etc.)
		return
	}

	// 2. Check if deployment record already exists
	existingDep, err := c.repositories.Deployments.GetLatestByService(ctx, service.ID.String())
	if err == nil && existingDep != nil {
		// Deployment record exists, update health status if needed
		c.updateDeploymentHealth(ctx, existingDep, &k8sDep, logger)
		return
	}

	// 3. No deployment record exists - create missing release + deployment records
	if len(k8sDep.Spec.Template.Spec.Containers) == 0 {
		logger.WithField("deployment", deploymentName).Warn("K8s deployment has no containers, skipping")
		return
	}

	imageURI := k8sDep.Spec.Template.Spec.Containers[0].Image
	replicas := int32(1)
	if k8sDep.Spec.Replicas != nil {
		replicas = *k8sDep.Spec.Replicas
	}
	availableReplicas := k8sDep.Status.AvailableReplicas

	c.createMissingRecords(ctx, service, namespace, imageURI, replicas, availableReplicas, logger)
}

// updateDeploymentHealth updates the health status of an existing deployment based on K8s state
func (c *Controller) updateDeploymentHealth(ctx context.Context, deployment *types.Deployment, k8sDep *appsv1.Deployment, logger *logrus.Entry) {
	replicas := int32(1)
	if k8sDep.Spec.Replicas != nil {
		replicas = *k8sDep.Spec.Replicas
	}
	availableReplicas := k8sDep.Status.AvailableReplicas

	// Determine expected health status
	var expectedHealth types.HealthStatus
	if availableReplicas == replicas && replicas > 0 {
		expectedHealth = types.HealthStatusHealthy
	} else if availableReplicas > 0 {
		expectedHealth = types.HealthStatusUnhealthy
	} else {
		expectedHealth = types.HealthStatusUnknown
	}

	// Determine expected deployment status based on K8s state
	// If K8s shows healthy pods but deployment is stuck at pending, transition to running
	newStatus := deployment.Status
	if expectedHealth == types.HealthStatusHealthy {
		if deployment.Status == types.DeploymentStatusPending {
			newStatus = types.DeploymentStatusRunning
			logger.WithFields(logrus.Fields{
				"deployment_id": deployment.ID,
				"old_status":    deployment.Status,
				"new_status":    newStatus,
			}).Info("Transitioning deployment from pending to running based on K8s state")
		}
	}

	// Only update if health or status changed
	if deployment.Health != expectedHealth || deployment.Status != newStatus {
		if err := c.repositories.Deployments.UpdateStatus(deployment.ID, newStatus, expectedHealth); err != nil {
			logger.WithFields(logrus.Fields{
				"deployment_id": deployment.ID,
				"old_health":    deployment.Health,
				"new_health":    expectedHealth,
				"old_status":    deployment.Status,
				"new_status":    newStatus,
				"error":         err,
			}).Warn("Failed to update deployment status")
		} else {
			logger.WithFields(logrus.Fields{
				"deployment_id": deployment.ID,
				"old_health":    deployment.Health,
				"new_health":    expectedHealth,
				"old_status":    deployment.Status,
				"new_status":    newStatus,
			}).Debug("Updated deployment status")
		}
	}
}

// createMissingRecords creates release and deployment records for a K8s deployment
func (c *Controller) createMissingRecords(ctx context.Context, service *types.Service, namespace string, imageURI string, replicas int32, availableReplicas int32, logger *logrus.Entry) {
	// Extract version and git SHA from image URI
	version := extractVersionFromImage(imageURI)
	gitSHA := extractGitSHAFromImage(imageURI)

	// Get environment by namespace
	env, err := c.repositories.Environments.GetByKubeNamespace(namespace)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"namespace": namespace,
			"service":   service.Name,
			"error":     err,
		}).Warn("No environment found for namespace, skipping")
		return
	}

	// Create release record
	release := &types.Release{
		ServiceID: service.ID,
		Version:   version,
		ImageURI:  imageURI,
		GitSHA:    gitSHA,
		Status:    types.ReleaseStatusReady,
	}

	if err := c.repositories.Releases.Create(release); err != nil {
		logger.WithFields(logrus.Fields{
			"service": service.Name,
			"error":   err,
		}).Error("Failed to create release record")
		return
	}

	// Determine health status based on replica counts
	health := types.HealthStatusUnknown
	if availableReplicas == replicas && replicas > 0 {
		health = types.HealthStatusHealthy
	} else if availableReplicas > 0 {
		health = types.HealthStatusUnhealthy
	}

	// Create deployment record
	deployment := &types.Deployment{
		ReleaseID:     release.ID,
		EnvironmentID: env.ID,
		Replicas:      int(replicas),
		Status:        types.DeploymentStatusRunning,
		Health:        health,
	}

	if err := c.repositories.Deployments.Create(deployment); err != nil {
		logger.WithFields(logrus.Fields{
			"service":    service.Name,
			"release_id": release.ID,
			"error":      err,
		}).Error("Failed to create deployment record")
		return
	}

	logger.WithFields(logrus.Fields{
		"service":       service.Name,
		"namespace":     namespace,
		"release_id":    release.ID,
		"deployment_id": deployment.ID,
		"version":       version,
	}).Info("Created missing deployment record from K8s state")
}

// extractVersionFromImage extracts version string from an image URI
// e.g., "ghcr.io/madfam-org/enclii/waybill:1ead1b30fdb4" -> "1ead1b30"
func extractVersionFromImage(imageURI string) string {
	// Find the tag after the last ":"
	lastColon := -1
	for i := len(imageURI) - 1; i >= 0; i-- {
		if imageURI[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon == -1 || lastColon == len(imageURI)-1 {
		return "unknown"
	}

	tag := imageURI[lastColon+1:]
	// If tag is longer than 12 chars, truncate for version display
	if len(tag) > 12 {
		return tag[:12]
	}
	return tag
}

// extractGitSHAFromImage extracts git SHA from an image URI
// e.g., "ghcr.io/madfam-org/enclii/waybill:1ead1b30fdb4" -> "1ead1b30fdb4"
func extractGitSHAFromImage(imageURI string) string {
	// Find the tag after the last ":"
	lastColon := -1
	for i := len(imageURI) - 1; i >= 0; i-- {
		if imageURI[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon == -1 || lastColon == len(imageURI)-1 {
		return ""
	}

	return imageURI[lastColon+1:]
}

// GetPodEnvVars retrieves environment variables from a running pod (delegates to ServiceReconciler)
func (c *Controller) GetPodEnvVars(ctx context.Context, namespace, podName string) (map[string]string, error) {
	return c.serviceReconciler.GetPodEnvVars(ctx, namespace, podName)
}
