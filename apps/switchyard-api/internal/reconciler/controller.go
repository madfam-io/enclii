package reconciler

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// Controller manages the reconciliation loop for all deployments
type Controller struct {
	db                *sql.DB
	repositories      *db.Repositories
	serviceReconciler *ServiceReconciler
	k8sClient         *k8s.Client
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
		k8sClient:         k8sClient,
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

	// Start K8s→DB sync job (runs every 60 seconds)
	c.wg.Add(1)
	go c.k8sSyncScheduler(ctx)

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
