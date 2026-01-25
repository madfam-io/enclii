package reconciler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

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
