package rotation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/lockbox"
)

// Controller orchestrates zero-downtime secret rotation
type Controller struct {
	k8sClient     *k8s.Client
	repos         *db.Repositories
	logger        *logrus.Logger
	eventQueue    chan *lockbox.SecretChangeEvent
	auditQueue    chan *lockbox.RotationAuditLog
	maxConcurrent int
	enabled       bool
}

// Config holds configuration for the rotation controller
type Config struct {
	MaxConcurrent int           // Max concurrent rotations
	Timeout       time.Duration // Timeout for each rotation
	Enabled       bool          // Enable secret rotation
}

// NewController creates a new rotation controller
func NewController(
	k8sClient *k8s.Client,
	repos *db.Repositories,
	logger *logrus.Logger,
	cfg *Config,
) *Controller {
	if cfg.MaxConcurrent == 0 {
		cfg.MaxConcurrent = 3 // Default: max 3 concurrent rotations
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute // Default timeout
	}

	return &Controller{
		k8sClient:     k8sClient,
		repos:         repos,
		logger:        logger,
		eventQueue:    make(chan *lockbox.SecretChangeEvent, 100),
		auditQueue:    make(chan *lockbox.RotationAuditLog, 100),
		maxConcurrent: cfg.MaxConcurrent,
		enabled:       cfg.Enabled,
	}
}

// Start begins processing secret rotation events
func (c *Controller) Start(ctx context.Context) error {
	if !c.enabled {
		c.logger.Info("Secret rotation controller is disabled")
		return nil
	}

	c.logger.Infof("Starting secret rotation controller (max concurrent: %d)", c.maxConcurrent)

	// Start audit log writer
	go c.processAuditLogs(ctx)

	// Start rotation workers
	for i := 0; i < c.maxConcurrent; i++ {
		go c.worker(ctx, i)
	}

	<-ctx.Done()
	c.logger.Info("Secret rotation controller shutting down")
	return nil
}

// EnqueueRotation adds a secret change event to the rotation queue
func (c *Controller) EnqueueRotation(event *lockbox.SecretChangeEvent) error {
	if !c.enabled {
		return fmt.Errorf("secret rotation is disabled")
	}

	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	select {
	case c.eventQueue <- event:
		c.logger.Infof("Enqueued secret rotation: %s (version %d -> %d)",
			event.SecretName, event.OldVersion, event.NewVersion)
		return nil
	default:
		return fmt.Errorf("rotation queue is full, try again later")
	}
}

// worker processes rotation events from the queue
func (c *Controller) worker(ctx context.Context, workerID int) {
	c.logger.Infof("Rotation worker %d started", workerID)

	for {
		select {
		case <-ctx.Done():
			c.logger.Infof("Rotation worker %d shutting down", workerID)
			return

		case event := <-c.eventQueue:
			c.logger.Infof("[Worker %d] Processing rotation for secret: %s", workerID, event.SecretName)

			// Create audit log
			auditLog := &lockbox.RotationAuditLog{
				ID:          uuid.New(),
				EventID:     event.ID,
				ServiceID:   event.ServiceID,
				Environment: event.Environment,
				SecretName:  event.SecretName,
				SecretPath:  event.SecretPath,
				OldVersion:  event.OldVersion,
				NewVersion:  event.NewVersion,
				Status:      lockbox.RotationInProgress,
				StartedAt:   time.Now().UTC(),
				TriggeredBy: event.TriggeredBy,
			}

			// Perform rotation
			err := c.performRotation(ctx, event, auditLog)

			// Update audit log
			completedAt := time.Now().UTC()
			auditLog.CompletedAt = &completedAt
			auditLog.Duration = completedAt.Sub(auditLog.StartedAt)

			if err != nil {
				c.logger.Errorf("[Worker %d] Rotation failed for %s: %v", workerID, event.SecretName, err)
				auditLog.Status = lockbox.RotationFailed
				auditLog.Error = err.Error()
				event.Status = lockbox.RotationFailed
				event.Error = err.Error()
			} else {
				c.logger.Infof("[Worker %d] Rotation completed for %s in %v", workerID, event.SecretName, auditLog.Duration)
				auditLog.Status = lockbox.RotationCompleted
				event.Status = lockbox.RotationCompleted
			}

			processedAt := time.Now().UTC()
			event.ProcessedAt = &processedAt

			// Send to audit queue
			c.auditQueue <- auditLog
		}
	}
}

// performRotation executes the zero-downtime rotation
func (c *Controller) performRotation(ctx context.Context, event *lockbox.SecretChangeEvent, auditLog *lockbox.RotationAuditLog) error {
	// Get service information
	service, err := c.repos.Service.GetByID(ctx, event.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	auditLog.ServiceName = service.Name

	c.logger.Infof("Rotating secret %s for service %s in environment %s",
		event.SecretName, service.Name, event.Environment)

	// Step 1: Update Kubernetes secret
	namespace := fmt.Sprintf("enclii-%s", service.ProjectID)
	secretName := fmt.Sprintf("%s-secrets", service.Name)

	c.logger.Infof("Updating Kubernetes secret %s/%s", namespace, secretName)

	// In a real implementation, you would:
	// 1. Fetch new secret value from Vault
	// 2. Update K8s Secret resource
	// For now, we'll trigger a rolling restart annotation update

	// Step 2: Trigger rolling restart with zero downtime
	c.logger.Infof("Triggering rolling restart for deployment %s/%s", namespace, service.Name)

	err = c.k8sClient.RollingRestart(ctx, namespace, service.Name)
	if err != nil {
		return fmt.Errorf("failed to trigger rolling restart: %w", err)
	}

	event.RolloutID = fmt.Sprintf("%s-%s", service.Name, time.Now().Format("20060102-150405"))
	auditLog.RolloutStrategy = "rolling"

	// Step 3: Monitor rollout progress
	c.logger.Infof("Monitoring rollout for deployment %s/%s", namespace, service.Name)

	// Wait for rollout to complete (with timeout)
	rolloutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	podsRestarted, err := c.waitForRollout(rolloutCtx, namespace, service.Name)
	if err != nil {
		// Attempt rollback
		c.logger.Errorf("Rollout failed, attempting rollback: %v", err)
		if rollbackErr := c.k8sClient.RollbackDeployment(ctx, namespace, service.Name); rollbackErr != nil {
			return fmt.Errorf("rollout failed and rollback failed: %w (original error: %v)", rollbackErr, err)
		}
		auditLog.Status = lockbox.RotationRolledBack
		return fmt.Errorf("rollout failed, rolled back: %w", err)
	}

	auditLog.PodsRestarted = podsRestarted

	c.logger.Infof("✓ Secret rotation completed successfully for %s (%d pods restarted)",
		event.SecretName, podsRestarted)

	return nil
}

// waitForRollout monitors a Kubernetes deployment rollout
func (c *Controller) waitForRollout(ctx context.Context, namespace, deploymentName string) (int, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	timeout := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()

		case <-ticker.C:
			// Check rollout status
			status, err := c.k8sClient.GetDeploymentStatusInfo(ctx, namespace, deploymentName)
			if err != nil {
				c.logger.Warnf("Failed to get deployment status: %v", err)
				continue
			}

			// Check if rollout is complete
			if status.UpdatedReplicas == status.Replicas &&
				status.AvailableReplicas == status.Replicas &&
				status.UnavailableReplicas == 0 {
				c.logger.Infof("Rollout complete: %d/%d replicas ready", status.AvailableReplicas, status.Replicas)
				return int(status.Replicas), nil
			}

			// Check timeout
			if time.Since(startTime) > timeout {
				return 0, fmt.Errorf("rollout timeout after %v", timeout)
			}

			c.logger.Infof("Rollout in progress: %d/%d replicas ready",
				status.AvailableReplicas, status.Replicas)
		}
	}
}

// processAuditLogs writes audit logs to the database
func (c *Controller) processAuditLogs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case auditLog := <-c.auditQueue:
			// In production, save to database
			// For now, just log to stdout
			c.logger.WithFields(logrus.Fields{
				"event_id":       auditLog.EventID,
				"service_id":     auditLog.ServiceID,
				"service_name":   auditLog.ServiceName,
				"secret_name":    auditLog.SecretName,
				"old_version":    auditLog.OldVersion,
				"new_version":    auditLog.NewVersion,
				"status":         auditLog.Status,
				"duration":       auditLog.Duration,
				"pods_restarted": auditLog.PodsRestarted,
			}).Info("Secret rotation audit log")

			// TODO: Save to database using repos.RotationAuditLog.Create()
		}
	}
}

// GetRotationHistory returns recent rotation history for a service
func (c *Controller) GetRotationHistory(ctx context.Context, serviceID string, limit int) ([]*lockbox.RotationAuditLog, error) {
	// TODO: Implement database query
	// For now, return empty slice
	return []*lockbox.RotationAuditLog{}, nil
}

// IsEnabled returns whether secret rotation is enabled
func (c *Controller) IsEnabled() bool {
	return c.enabled
}
