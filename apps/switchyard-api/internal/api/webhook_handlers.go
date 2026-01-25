package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
)

// GitHubWebhook handles incoming GitHub webhook events
// This endpoint is used for automated deployments triggered by git pushes
func (h *Handler) GitHubWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if webhook secret is configured
	if h.config.GitHubWebhookSecret == "" {
		h.logger.Warn(ctx, "GitHub webhook received but no secret configured - rejecting for security")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook not configured"})
		return
	}

	// Read the raw body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error(ctx, "Failed to read webhook body", logging.Error("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Verify GitHub signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if signature == "" {
		h.logger.Warn(ctx, "GitHub webhook missing signature header")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing signature"})
		return
	}

	if !verifyGitHubSignature(body, signature, h.config.GitHubWebhookSecret) {
		h.logger.Warn(ctx, "GitHub webhook signature verification failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Check event type
	eventType := c.GetHeader("X-GitHub-Event")
	h.logger.Info(ctx, "Received GitHub webhook",
		logging.String("event_type", eventType),
		logging.String("delivery_id", c.GetHeader("X-GitHub-Delivery")))

	switch eventType {
	case "push":
		h.handleGitHubPush(c, ctx, body)
	case "pull_request":
		h.handleGitHubPullRequest(c, ctx, body)
	case "workflow_run":
		h.handleGitHubWorkflowRun(c, ctx, body)
	case "ping":
		// GitHub sends ping event when webhook is first configured
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	default:
		// Acknowledge but ignore unsupported events
		c.JSON(http.StatusOK, gin.H{"message": "Event type not handled", "event": eventType})
	}
}
