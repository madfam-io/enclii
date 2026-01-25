package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/queue"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/switchyard"
	"go.uber.org/zap"
)

// GitHubHandler handles GitHub webhook events
type GitHubHandler struct {
	secret           string
	logger           *zap.Logger
	switchyardClient *switchyard.Client
	previewsEnabled  bool
}

// GitHubHandlerConfig contains configuration for the GitHub webhook handler
type GitHubHandlerConfig struct {
	Secret           string
	SwitchyardURL    string
	SwitchyardAPIKey string
	PreviewsEnabled  bool
}

// NewGitHubHandler creates a new GitHub webhook handler
func NewGitHubHandler(secret string, logger *zap.Logger) *GitHubHandler {
	return &GitHubHandler{
		secret:          secret,
		logger:          logger,
		previewsEnabled: false,
	}
}

// NewGitHubHandlerWithConfig creates a new GitHub webhook handler with full configuration
func NewGitHubHandlerWithConfig(cfg *GitHubHandlerConfig, logger *zap.Logger) *GitHubHandler {
	h := &GitHubHandler{
		secret:          cfg.Secret,
		logger:          logger,
		previewsEnabled: cfg.PreviewsEnabled,
	}

	// Initialize Switchyard client for preview environment integration
	if cfg.SwitchyardURL != "" && cfg.PreviewsEnabled {
		h.switchyardClient = switchyard.NewClient(cfg.SwitchyardURL, cfg.SwitchyardAPIKey, logger)
		logger.Info("Preview environment integration enabled",
			zap.String("switchyard_url", cfg.SwitchyardURL))
	}

	return h
}

// GitHubPushPayload represents a GitHub push event
type GitHubPushPayload struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Repository struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
		Private  bool   `json:"private"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	HeadCommit struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"commits"`
}

// GitHubPRPayload represents a GitHub pull request event
type GitHubPRPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		ID     int64  `json:"id"`
		Number int    `json:"number"`
		State  string `json:"state"`
		Title  string `json:"title"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
		User    struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

// Handle processes incoming GitHub webhooks
func (h *GitHubHandler) Handle(c *gin.Context) {
	// SECURITY: Require webhook secret to be configured
	// Without a secret, anyone could send fake webhooks to trigger builds
	if h.secret == "" {
		h.logger.Error("webhook secret not configured - rejecting request for security")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "webhook secret not configured",
			"hint":  "set GITHUB_WEBHOOK_SECRET environment variable",
		})
		return
	}

	// Validate signature - always required when secret is configured
	signature := c.GetHeader("X-Hub-Signature-256")
	if signature == "" {
		h.logger.Warn("webhook received without signature header")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature header"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	if !h.validateSignature(body, signature) {
		h.logger.Warn("webhook signature validation failed",
			zap.String("signature", signature[:20]+"..."),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Determine event type
	eventType := c.GetHeader("X-GitHub-Event")

	var payload *queue.WebhookPayload

	switch eventType {
	case "push":
		payload, err = h.handlePush(body)
	case "pull_request":
		payload, err = h.handlePullRequest(body)
	case "ping":
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
		return
	default:
		h.logger.Debug("ignoring event", zap.String("event", eventType))
		c.JSON(http.StatusOK, gin.H{"message": "event ignored"})
		return
	}

	if err != nil {
		h.logger.Error("failed to parse webhook", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Store payload in context for downstream processing
	c.Set("webhook_payload", payload)

	// Log webhook details (handle empty commit SHA for closed events)
	commitLog := "n/a"
	if len(payload.CommitSHA) >= 8 {
		commitLog = payload.CommitSHA[:8]
	}

	h.logger.Info("webhook received",
		zap.String("provider", payload.Provider),
		zap.String("event", payload.Event),
		zap.String("repository", payload.Repository),
		zap.String("branch", payload.Branch),
		zap.String("commit", commitLog),
	)

	// Process preview environment for PR events (async)
	isPREvent := payload.Event == "pull_request" || payload.Event == "pull_request_closed"
	if isPREvent && h.previewsEnabled && h.switchyardClient != nil {
		go h.processPreviewEnvironment(context.Background(), payload, body)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "webhook received",
		"repository": payload.Repository,
		"branch":     payload.Branch,
		"commit":     payload.CommitSHA,
	})
}

// processPreviewEnvironment handles preview environment creation/update/close
func (h *GitHubHandler) processPreviewEnvironment(ctx context.Context, payload *queue.WebhookPayload, rawBody []byte) {
	// Re-parse the PR payload to get full details
	var pr GitHubPRPayload
	if err := json.Unmarshal(rawBody, &pr); err != nil {
		h.logger.Error("Failed to parse PR payload for preview processing", zap.Error(err))
		return
	}

	logger := h.logger.With(
		zap.Int("pr_number", pr.PullRequest.Number),
		zap.String("action", pr.Action),
		zap.String("repo", pr.Repository.FullName),
	)

	// Find services that use this repository
	services, err := h.switchyardClient.GetServicesByRepo(ctx, pr.Repository.CloneURL)
	if err != nil {
		logger.Error("Failed to find services for repository", zap.Error(err))
		return
	}

	if len(services.Services) == 0 {
		logger.Debug("No services found for repository, skipping preview environment")
		return
	}

	// Handle based on PR action
	switch pr.Action {
	case "opened", "reopened", "synchronize":
		h.createOrUpdatePreviews(ctx, logger, services, &pr)
	case "closed":
		h.closePreviews(ctx, logger, services, &pr)
	default:
		logger.Debug("Ignoring PR action for preview environments", zap.String("action", pr.Action))
	}
}

// createOrUpdatePreviews creates or updates preview environments for all matching services
func (h *GitHubHandler) createOrUpdatePreviews(ctx context.Context, logger *zap.Logger, services *switchyard.ServiceByRepoResponse, pr *GitHubPRPayload) {
	for _, svc := range services.Services {
		svcLogger := logger.With(
			zap.String("service_id", svc.ID),
			zap.String("service_name", svc.Name),
		)

		req := &switchyard.CreatePreviewRequest{
			ServiceID:    svc.ID,
			PRNumber:     pr.PullRequest.Number,
			PRTitle:      pr.PullRequest.Title,
			PRURL:        pr.PullRequest.HTMLURL,
			PRAuthor:     pr.PullRequest.User.Login,
			PRBranch:     pr.PullRequest.Head.Ref,
			PRBaseBranch: pr.PullRequest.Base.Ref,
			CommitSHA:    pr.PullRequest.Head.SHA,
		}

		resp, err := h.switchyardClient.CreatePreview(ctx, req)
		if err != nil {
			svcLogger.Error("Failed to create/update preview environment", zap.Error(err))
			continue
		}

		svcLogger.Info("Preview environment processed",
			zap.String("preview_id", resp.Preview.ID),
			zap.String("preview_url", resp.Preview.PreviewURL),
			zap.String("action", resp.Action),
		)
	}
}

// closePreviews closes preview environments for all matching services
func (h *GitHubHandler) closePreviews(ctx context.Context, logger *zap.Logger, services *switchyard.ServiceByRepoResponse, pr *GitHubPRPayload) {
	for _, svc := range services.Services {
		svcLogger := logger.With(
			zap.String("service_id", svc.ID),
			zap.String("service_name", svc.Name),
		)

		if err := h.switchyardClient.ClosePreviewByPR(ctx, svc.ID, pr.PullRequest.Number); err != nil {
			svcLogger.Error("Failed to close preview environment", zap.Error(err))
			continue
		}

		svcLogger.Info("Preview environment closed")
	}
}

func (h *GitHubHandler) handlePush(body []byte) (*queue.WebhookPayload, error) {
	var push GitHubPushPayload
	if err := json.Unmarshal(body, &push); err != nil {
		return nil, fmt.Errorf("failed to parse push payload: %w", err)
	}

	// Extract branch from ref (refs/heads/main -> main)
	branch := strings.TrimPrefix(push.Ref, "refs/heads/")

	// Skip if this is a branch deletion
	if push.After == "0000000000000000000000000000000000000000" {
		return nil, fmt.Errorf("branch deletion, skipping")
	}

	return &queue.WebhookPayload{
		Provider:   "github",
		Event:      "push",
		Repository: push.Repository.CloneURL,
		Branch:     branch,
		CommitSHA:  push.After,
		Author:     push.Pusher.Name,
		Message:    push.HeadCommit.Message,
	}, nil
}

func (h *GitHubHandler) handlePullRequest(body []byte) (*queue.WebhookPayload, error) {
	var pr GitHubPRPayload
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR payload: %w", err)
	}

	// Actions that trigger builds
	buildActions := map[string]bool{
		"opened":      true,
		"synchronize": true,
		"reopened":    true,
	}

	// Actions that we process (includes closed for preview environments)
	validActions := map[string]bool{
		"opened":      true,
		"synchronize": true,
		"reopened":    true,
		"closed":      true,
	}

	if !validActions[pr.Action] {
		return nil, fmt.Errorf("ignoring PR action: %s", pr.Action)
	}

	// Determine event type based on action
	eventType := "pull_request"
	if pr.Action == "closed" {
		eventType = "pull_request_closed"
	}

	// Only include commit SHA for build actions
	commitSHA := ""
	if buildActions[pr.Action] {
		commitSHA = pr.PullRequest.Head.SHA
	}

	return &queue.WebhookPayload{
		Provider:   "github",
		Event:      eventType,
		Repository: pr.Repository.CloneURL,
		Branch:     pr.PullRequest.Head.Ref,
		CommitSHA:  commitSHA,
		Author:     pr.PullRequest.User.Login,
		Message:    pr.PullRequest.Title,
		PRURL:      pr.PullRequest.HTMLURL,
		PRNumber:   pr.PullRequest.Number,
	}, nil
}

func (h *GitHubHandler) validateSignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSig), []byte(signature))
}
