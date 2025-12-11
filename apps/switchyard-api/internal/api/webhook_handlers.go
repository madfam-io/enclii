package api

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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
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
	case "ping":
		// GitHub sends ping event when webhook is first configured
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	default:
		// Acknowledge but ignore unsupported events
		c.JSON(http.StatusOK, gin.H{"message": "Event type not handled", "event": eventType})
	}
}

// GitHubPushEvent represents a GitHub push webhook payload
type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Created    bool   `json:"created"`
	Deleted    bool   `json:"deleted"`
	Forced     bool   `json:"forced"`
	Repository struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
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
}

// handleGitHubPush processes push events and triggers builds for matching services
func (h *Handler) handleGitHubPush(c *gin.Context, ctx context.Context, body []byte) {
	var event GitHubPushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error(ctx, "Failed to parse push event", logging.Error("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid push event payload"})
		return
	}

	// Only trigger builds for pushes to main/master branch
	branch := extractBranchName(event.Ref)
	if branch != "main" && branch != "master" {
		h.logger.Info(ctx, "Ignoring push to non-main branch",
			logging.String("branch", branch),
			logging.String("repo", event.Repository.FullName))
		c.JSON(http.StatusOK, gin.H{
			"message": "Push to non-main branch ignored",
			"branch":  branch,
		})
		return
	}

	// Skip if this is a branch deletion
	if event.Deleted {
		h.logger.Info(ctx, "Ignoring branch deletion event",
			logging.String("branch", branch))
		c.JSON(http.StatusOK, gin.H{"message": "Branch deletion ignored"})
		return
	}

	// Find service by git repo URL
	repoURL := event.Repository.CloneURL
	service, err := h.repos.Services.GetByGitRepo(repoURL)
	if err != nil {
		// Try with HTTPS URL
		repoURL = event.Repository.HTMLURL
		service, err = h.repos.Services.GetByGitRepo(repoURL)
		if err != nil {
			// Try with SSH URL
			repoURL = event.Repository.SSHURL
			service, err = h.repos.Services.GetByGitRepo(repoURL)
			if err != nil {
				h.logger.Info(ctx, "No service found for repository",
					logging.String("repo", event.Repository.FullName),
					logging.String("clone_url", event.Repository.CloneURL))
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "No service registered for this repository",
					"repo":    event.Repository.FullName,
					"message": "Register a service with this git_repo URL to enable auto-deploy",
				})
				return
			}
		}
	}

	gitSHA := event.After
	if len(gitSHA) < 7 {
		h.logger.Error(ctx, "Invalid git SHA in push event",
			logging.String("sha", gitSHA))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid git SHA"})
		return
	}

	h.logger.Info(ctx, "Triggering build from GitHub webhook",
		logging.String("service_id", service.ID.String()),
		logging.String("service_name", service.Name),
		logging.String("git_sha", gitSHA),
		logging.String("branch", branch),
		logging.String("pusher", event.Pusher.Name),
		logging.String("commit_message", truncateString(event.HeadCommit.Message, 100)))

	// Create release record
	release := &types.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		Version:   "v" + time.Now().Format("20060102-150405") + "-" + gitSHA[:7],
		ImageURI:  h.config.Registry + "/" + service.Name + ":" + gitSHA[:7],
		GitSHA:    gitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repos.Releases.Create(release); err != nil {
		h.logger.Error(ctx, "Failed to create release from webhook",
			logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create release"})
		return
	}

	// Trigger async build
	go h.triggerBuild(service, release, gitSHA)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Build triggered",
		"service":    service.Name,
		"release_id": release.ID.String(),
		"git_sha":    gitSHA,
		"branch":     branch,
	})
}

// verifyGitHubSignature verifies the HMAC SHA-256 signature from GitHub
func verifyGitHubSignature(payload []byte, signature string, secret string) bool {
	// GitHub sends signature in format: sha256=<hex digest>
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	signatureHex := strings.TrimPrefix(signature, "sha256=")

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expectedSig), []byte(signatureHex))
}

// extractBranchName extracts the branch name from a git ref
// e.g., "refs/heads/main" -> "main"
func extractBranchName(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	return ref
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	// Replace newlines with spaces for log readability
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GitHubPullRequestEvent represents a GitHub pull_request webhook payload
type GitHubPullRequestEvent struct {
	Action      string `json:"action"` // opened, synchronize, closed, reopened
	Number      int    `json:"number"`
	PullRequest struct {
		ID      int64  `json:"id"`
		Number  int    `json:"number"`
		Title   string `json:"title"`
		State   string `json:"state"` // open, closed
		HTMLURL string `json:"html_url"`
		User    struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"` // Branch name
			SHA string `json:"sha"` // Commit SHA
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"` // Base branch (usually main/master)
		} `json:"base"`
		Merged   bool `json:"merged"`
		MergedAt any  `json:"merged_at"` // null or timestamp
	} `json:"pull_request"`
	Repository struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

// handleGitHubPullRequest processes pull_request events for preview environments
func (h *Handler) handleGitHubPullRequest(c *gin.Context, ctx context.Context, body []byte) {
	var event GitHubPullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error(ctx, "Failed to parse pull_request event", logging.Error("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pull_request event payload"})
		return
	}

	h.logger.Info(ctx, "Processing pull_request event",
		logging.String("action", event.Action),
		logging.Int("pr_number", event.Number),
		logging.String("repo", event.Repository.FullName),
		logging.String("branch", event.PullRequest.Head.Ref),
		logging.String("sha", event.PullRequest.Head.SHA))

	// Find service by git repo URL
	service, err := h.findServiceByRepo(ctx, event.Repository.CloneURL, event.Repository.HTMLURL, event.Repository.SSHURL)
	if err != nil {
		h.logger.Info(ctx, "No service found for repository",
			logging.String("repo", event.Repository.FullName))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No service registered for this repository",
			"repo":    event.Repository.FullName,
			"message": "Register a service with this git_repo URL to enable preview environments",
		})
		return
	}

	switch event.Action {
	case "opened", "reopened":
		h.createPreviewEnvironment(c, ctx, service, &event)
	case "synchronize":
		h.updatePreviewEnvironment(c, ctx, service, &event)
	case "closed":
		h.closePreviewEnvironment(c, ctx, service, &event)
	default:
		c.JSON(http.StatusOK, gin.H{
			"message": "PR action not handled",
			"action":  event.Action,
		})
	}
}

// findServiceByRepo attempts to find a service by trying different repo URL formats
func (h *Handler) findServiceByRepo(ctx context.Context, cloneURL, htmlURL, sshURL string) (*types.Service, error) {
	service, err := h.repos.Services.GetByGitRepo(cloneURL)
	if err == nil {
		return service, nil
	}

	service, err = h.repos.Services.GetByGitRepo(htmlURL)
	if err == nil {
		return service, nil
	}

	service, err = h.repos.Services.GetByGitRepo(sshURL)
	if err == nil {
		return service, nil
	}

	// Try owner/repo format
	parts := strings.Split(htmlURL, "/")
	if len(parts) >= 2 {
		ownerRepo := parts[len(parts)-2] + "/" + parts[len(parts)-1]
		service, err = h.repos.Services.GetByGitRepo(ownerRepo)
		if err == nil {
			return service, nil
		}
	}

	return nil, err
}

// createPreviewEnvironment creates a new preview environment for a PR
func (h *Handler) createPreviewEnvironment(c *gin.Context, ctx context.Context, service *types.Service, event *GitHubPullRequestEvent) {
	// Check if preview already exists (reopen case)
	existing, _ := h.repos.PreviewEnvironments.GetByServiceAndPR(ctx, service.ID, event.Number)
	if existing != nil {
		// Reopen existing preview
		if existing.Status == types.PreviewStatusClosed {
			existing.Status = types.PreviewStatusPending
			existing.CommitSHA = event.PullRequest.Head.SHA
			existing.ClosedAt = nil
			if err := h.repos.PreviewEnvironments.UpdateStatus(ctx, existing.ID, types.PreviewStatusPending, "PR reopened, rebuilding preview"); err != nil {
				h.logger.Error(ctx, "Failed to reopen preview environment", logging.Error("error", err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reopen preview environment"})
				return
			}
			h.logger.Info(ctx, "Reopened preview environment",
				logging.String("preview_id", existing.ID.String()),
				logging.Int("pr_number", event.Number))

			// Trigger new build
			go h.triggerPreviewBuild(service, existing, event.PullRequest.Head.SHA)

			c.JSON(http.StatusOK, gin.H{
				"message":     "Preview environment reopened",
				"preview_id":  existing.ID.String(),
				"preview_url": existing.PreviewURL,
				"pr_number":   event.Number,
			})
			return
		}

		// Already exists and active
		c.JSON(http.StatusOK, gin.H{
			"message":     "Preview environment already exists",
			"preview_id":  existing.ID.String(),
			"preview_url": existing.PreviewURL,
			"pr_number":   event.Number,
		})
		return
	}

	// Generate preview subdomain: pr-{number}-{service-slug}.preview.enclii.app
	serviceSlug := strings.ToLower(strings.ReplaceAll(service.Name, " ", "-"))
	serviceSlug = strings.ToLower(strings.ReplaceAll(serviceSlug, "_", "-"))
	subdomain := "pr-" + itoa(event.Number) + "-" + serviceSlug
	previewURL := "https://" + subdomain + ".preview.enclii.app"

	preview := &types.PreviewEnvironment{
		ID:               uuid.New(),
		ProjectID:        service.ProjectID,
		ServiceID:        service.ID,
		PRNumber:         event.Number,
		PRTitle:          event.PullRequest.Title,
		PRURL:            event.PullRequest.HTMLURL,
		PRAuthor:         event.PullRequest.User.Login,
		PRBranch:         event.PullRequest.Head.Ref,
		PRBaseBranch:     event.PullRequest.Base.Ref,
		CommitSHA:        event.PullRequest.Head.SHA,
		PreviewSubdomain: subdomain,
		PreviewURL:       previewURL,
		Status:           types.PreviewStatusPending,
		StatusMessage:    "Preview environment created, starting build",
		AutoSleepAfter:   30, // 30 minutes default
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := h.repos.PreviewEnvironments.Create(ctx, preview); err != nil {
		h.logger.Error(ctx, "Failed to create preview environment", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create preview environment"})
		return
	}

	h.logger.Info(ctx, "Created preview environment",
		logging.String("preview_id", preview.ID.String()),
		logging.String("preview_url", previewURL),
		logging.Int("pr_number", event.Number),
		logging.String("service", service.Name))

	// Trigger async build for preview
	go h.triggerPreviewBuild(service, preview, event.PullRequest.Head.SHA)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Preview environment created",
		"preview_id":  preview.ID.String(),
		"preview_url": previewURL,
		"pr_number":   event.Number,
		"subdomain":   subdomain,
	})
}

// updatePreviewEnvironment updates an existing preview environment with new commits
func (h *Handler) updatePreviewEnvironment(c *gin.Context, ctx context.Context, service *types.Service, event *GitHubPullRequestEvent) {
	preview, err := h.repos.PreviewEnvironments.GetByServiceAndPR(ctx, service.ID, event.Number)
	if err != nil {
		// No preview exists, create one
		h.createPreviewEnvironment(c, ctx, service, event)
		return
	}

	// If preview is closed, reopen it
	if preview.Status == types.PreviewStatusClosed {
		h.createPreviewEnvironment(c, ctx, service, event)
		return
	}

	// Update commit SHA and trigger rebuild
	if err := h.repos.PreviewEnvironments.UpdateCommit(ctx, preview.ID, event.PullRequest.Head.SHA); err != nil {
		h.logger.Error(ctx, "Failed to update preview commit", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preview environment"})
		return
	}

	// Update status to building
	if err := h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusBuilding, "New commits pushed, rebuilding"); err != nil {
		h.logger.Error(ctx, "Failed to update preview status", logging.Error("error", err))
	}

	h.logger.Info(ctx, "Updating preview environment with new commit",
		logging.String("preview_id", preview.ID.String()),
		logging.String("old_sha", preview.CommitSHA),
		logging.String("new_sha", event.PullRequest.Head.SHA),
		logging.Int("pr_number", event.Number))

	// Trigger rebuild
	go h.triggerPreviewBuild(service, preview, event.PullRequest.Head.SHA)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Preview environment updating",
		"preview_id":  preview.ID.String(),
		"preview_url": preview.PreviewURL,
		"pr_number":   event.Number,
		"commit_sha":  event.PullRequest.Head.SHA,
	})
}

// closePreviewEnvironment closes a preview environment when PR is closed/merged
func (h *Handler) closePreviewEnvironment(c *gin.Context, ctx context.Context, service *types.Service, event *GitHubPullRequestEvent) {
	preview, err := h.repos.PreviewEnvironments.GetByServiceAndPR(ctx, service.ID, event.Number)
	if err != nil {
		h.logger.Info(ctx, "No preview environment found for closed PR",
			logging.Int("pr_number", event.Number),
			logging.String("service", service.Name))
		c.JSON(http.StatusOK, gin.H{
			"message":   "No preview environment found for this PR",
			"pr_number": event.Number,
		})
		return
	}

	statusMessage := "PR closed"
	if event.PullRequest.Merged {
		statusMessage = "PR merged"
	}

	if err := h.repos.PreviewEnvironments.Close(ctx, preview.ID); err != nil {
		h.logger.Error(ctx, "Failed to close preview environment", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close preview environment"})
		return
	}

	// Update status message
	if err := h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusClosed, statusMessage); err != nil {
		h.logger.Error(ctx, "Failed to update preview status message", logging.Error("error", err))
	}

	h.logger.Info(ctx, "Closed preview environment",
		logging.String("preview_id", preview.ID.String()),
		logging.Int("pr_number", event.Number),
		logging.String("reason", statusMessage))

	// TODO: Trigger cleanup of preview resources (deployment, ingress, etc.)
	go h.cleanupPreviewResources(preview)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Preview environment closed",
		"preview_id": preview.ID.String(),
		"pr_number":  event.Number,
		"reason":     statusMessage,
		"merged":     event.PullRequest.Merged,
	})
}

// triggerPreviewBuild triggers an async build for a preview environment
func (h *Handler) triggerPreviewBuild(service *types.Service, preview *types.PreviewEnvironment, gitSHA string) {
	ctx := context.Background()

	// Create release for preview
	release := &types.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		Version:   "preview-pr-" + itoa(preview.PRNumber) + "-" + gitSHA[:7],
		ImageURI:  h.config.Registry + "/" + service.Name + ":pr-" + itoa(preview.PRNumber) + "-" + gitSHA[:7],
		GitSHA:    gitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repos.Releases.Create(release); err != nil {
		h.logger.Error(ctx, "Failed to create preview release",
			logging.Error("db_error", err),
			logging.String("preview_id", preview.ID.String()))
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Failed to create release: "+err.Error())
		return
	}

	// Note: The deployment_id will be linked once the build completes and a deployment is created
	h.logger.Info(ctx, "Created release for preview",
		logging.String("release_id", release.ID.String()),
		logging.String("preview_id", preview.ID.String()))

	// Update preview status
	h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusBuilding, "Building image from commit "+gitSHA[:7])

	// Trigger actual build (reuse existing triggerBuild logic)
	h.triggerBuild(service, release, gitSHA)

	// After build completes, update preview status
	// Note: In production, this would be event-driven, but for now we poll
	h.logger.Info(ctx, "Preview build triggered",
		logging.String("preview_id", preview.ID.String()),
		logging.String("release_id", release.ID.String()),
		logging.String("git_sha", gitSHA))
}

// cleanupPreviewResources cleans up resources for a closed preview environment
func (h *Handler) cleanupPreviewResources(preview *types.PreviewEnvironment) {
	ctx := context.Background()
	h.logger.Info(ctx, "Starting cleanup for preview environment",
		logging.String("preview_id", preview.ID.String()),
		logging.String("subdomain", preview.PreviewSubdomain))

	// TODO: Delete Kubernetes deployment
	// TODO: Delete Ingress/Route
	// TODO: Clean up any DNS records
	// TODO: Archive build logs

	h.logger.Info(ctx, "Preview environment cleanup completed",
		logging.String("preview_id", preview.ID.String()))
}

// itoa converts an int to string
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
