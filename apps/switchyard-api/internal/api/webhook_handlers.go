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
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
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
		ID        string   `json:"id"`
		Message   string   `json:"message"`
		Timestamp string   `json:"timestamp"`
		Added     []string `json:"added"`
		Modified  []string `json:"modified"`
		Removed   []string `json:"removed"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
	Commits []struct {
		ID       string   `json:"id"`
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// handleGitHubPush processes push events and triggers builds for ALL matching services
// Supports monorepos where multiple services share the same git repository
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

	gitSHA := event.After
	if len(gitSHA) < 7 {
		h.logger.Error(ctx, "Invalid git SHA in push event",
			logging.String("sha", gitSHA))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid git SHA"})
		return
	}

	// Find ALL services matching this git repo (monorepo support)
	// Try multiple URL formats that GitHub might send
	var services []*types.Service
	var err error

	for _, repoURL := range []string{
		event.Repository.CloneURL,
		event.Repository.HTMLURL,
		event.Repository.SSHURL,
	} {
		services, err = h.repos.Services.ListByGitRepo(repoURL)
		if err == nil && len(services) > 0 {
			break
		}
	}

	if len(services) == 0 {
		h.logger.Info(ctx, "No services found for repository",
			logging.String("repo", event.Repository.FullName),
			logging.String("clone_url", event.Repository.CloneURL))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No services registered for this repository",
			"repo":    event.Repository.FullName,
			"message": "Register a service with this git_repo URL to enable auto-deploy",
		})
		return
	}

	// Extract all changed files from the push event for monorepo path filtering
	changedFiles := extractChangedFiles(&event)

	h.logger.Info(ctx, "Triggering builds from GitHub webhook (monorepo)",
		logging.Int("service_count", len(services)),
		logging.Int("changed_files", len(changedFiles)),
		logging.String("repo", event.Repository.FullName),
		logging.String("git_sha", gitSHA),
		logging.String("branch", branch),
		logging.String("pusher", event.Pusher.Name),
		logging.String("commit_message", truncateString(event.HeadCommit.Message, 100)))

	// Trigger builds for matching services (filtered by watch paths if configured)
	type buildResult struct {
		Service   string `json:"service"`
		ReleaseID string `json:"release_id"`
		Status    string `json:"status"`
		Skipped   bool   `json:"skipped,omitempty"`
		Reason    string `json:"reason,omitempty"`
	}
	var results []buildResult
	var skippedCount int

	for _, service := range services {
		// Check if service should be rebuilt based on changed files and WatchPaths
		if len(service.WatchPaths) > 0 && !shouldRebuildService(service.WatchPaths, changedFiles) {
			h.logger.Info(ctx, "Skipping build for service - no relevant file changes",
				logging.String("service", service.Name),
				logging.String("watch_paths", strings.Join(service.WatchPaths, ", ")))
			results = append(results, buildResult{
				Service: service.Name,
				Status:  "skipped",
				Skipped: true,
				Reason:  "No files changed in watched paths",
			})
			skippedCount++
			continue
		}
		// Create release record for this service
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
			h.logger.Error(ctx, "Failed to create release for service",
				logging.String("service", service.Name),
				logging.Error("db_error", err))
			results = append(results, buildResult{
				Service: service.Name,
				Status:  "failed: " + err.Error(),
			})
			continue
		}

		// Trigger async build (routes to Roundhouse or in-process based on config)
		h.triggerBuildAsync(service, release, gitSHA, branch)

		h.logger.Info(ctx, "Build triggered for service",
			logging.String("service_id", service.ID.String()),
			logging.String("service_name", service.Name),
			logging.String("release_id", release.ID.String()))

		// Log webhook event to Activity feed for dashboard visibility
		h.repos.AuditLogs.Log(ctx, &types.AuditLog{
			ActorID:      nil, // System action (webhook)
			ActorEmail:   "github-webhook@system.enclii.dev",
			ActorRole:    types.RoleSystem,
			Action:       "webhook.build_triggered",
			ResourceType: "service",
			ResourceID:   service.ID.String(),
			ResourceName: service.Name,
			ProjectID:    &service.ProjectID,
			Outcome:      "success",
			Context: map[string]interface{}{
				"event_type":  "push",
				"commit_sha":  gitSHA,
				"branch":      branch,
				"repository":  event.Repository.FullName,
				"release_id":  release.ID.String(),
				"pusher":      event.Pusher.Name,
				"trigger":     "github_push",
			},
		})

		results = append(results, buildResult{
			Service:   service.Name,
			ReleaseID: release.ID.String(),
			Status:    "building",
		})
	}

	triggeredCount := len(results) - skippedCount
	c.JSON(http.StatusOK, gin.H{
		"message":         fmt.Sprintf("Builds triggered for %d services (%d skipped)", triggeredCount, skippedCount),
		"repo":            event.Repository.FullName,
		"git_sha":         gitSHA,
		"branch":          branch,
		"builds":          results,
		"service_count":   len(results),
		"triggered_count": triggeredCount,
		"skipped_count":   skippedCount,
		"changed_files":   len(changedFiles),
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

// extractChangedFiles extracts all changed file paths from a push event
// Combines added, modified, and removed files from all commits
func extractChangedFiles(event *GitHubPushEvent) []string {
	seen := make(map[string]bool)
	var files []string

	// Add files from head commit
	for _, f := range event.HeadCommit.Added {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	for _, f := range event.HeadCommit.Modified {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	for _, f := range event.HeadCommit.Removed {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}

	// Add files from all commits (for push with multiple commits)
	for _, commit := range event.Commits {
		for _, f := range commit.Added {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
		for _, f := range commit.Modified {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
		for _, f := range commit.Removed {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}

	return files
}

// shouldRebuildService checks if any changed file matches the service's watch paths
// Uses glob patterns and prefix matching for flexible path filtering
func shouldRebuildService(watchPaths []string, changedFiles []string) bool {
	for _, changed := range changedFiles {
		for _, watchPath := range watchPaths {
			if matchWatchPath(changed, watchPath) {
				return true
			}
		}
	}
	return false
}

// matchWatchPath checks if a file path matches a watch path pattern
// Supports:
// - Exact file matches: "package.json"
// - Directory prefixes: "apps/api/" (matches any file in that directory)
// - Glob patterns: "*.go", "apps/*/src/**"
func matchWatchPath(filePath, watchPath string) bool {
	// Handle glob patterns
	if strings.Contains(watchPath, "*") {
		matched, _ := filepath.Match(watchPath, filePath)
		if matched {
			return true
		}
		// Try matching just the filename for patterns like "*.go"
		matched, _ = filepath.Match(watchPath, filepath.Base(filePath))
		if matched {
			return true
		}
		// Try recursive glob matching for ** patterns
		if strings.Contains(watchPath, "**") {
			// Convert ** to single * for basic matching and check prefix
			prefix := strings.Split(watchPath, "**")[0]
			if strings.HasPrefix(filePath, prefix) {
				return true
			}
		}
		return false
	}

	// Handle directory prefix matching (e.g., "apps/api/")
	if strings.HasSuffix(watchPath, "/") {
		return strings.HasPrefix(filePath, watchPath)
	}

	// Exact match or directory prefix without trailing slash
	return filePath == watchPath || strings.HasPrefix(filePath, watchPath+"/")
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
// Uses a semaphore to serialize builds and prevent OOM from concurrent operations
func (h *Handler) triggerPreviewBuild(service *types.Service, preview *types.PreviewEnvironment, gitSHA string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Acquire build semaphore (blocks if another build is running)
	h.logger.Info(ctx, "Preview build waiting for build slot",
		logging.String("preview_id", preview.ID.String()))

	select {
	case h.buildSemaphore <- struct{}{}:
		// Acquired semaphore, ensure we release it when done
		defer func() { <-h.buildSemaphore }()
	case <-ctx.Done():
		h.logger.Error(ctx, "Preview build timed out waiting for semaphore",
			logging.String("preview_id", preview.ID.String()))
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Build timed out waiting for slot")
		return
	}

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

	h.logger.Info(ctx, "Created release for preview",
		logging.String("release_id", release.ID.String()),
		logging.String("preview_id", preview.ID.String()))

	// Update preview status
	h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusBuilding, "Building image from commit "+gitSHA[:7])

	// Execute the build synchronously within this goroutine
	buildResult := h.builder.BuildFromGit(ctx, service, gitSHA)

	if !buildResult.Success {
		h.logger.Error(ctx, "Preview build failed",
			logging.String("preview_id", preview.ID.String()),
			logging.Error("build_error", buildResult.Error))

		h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Build failed: "+buildResult.Error.Error())
		return
	}

	// Update release with image URI and mark as ready
	release.ImageURI = buildResult.ImageURI
	if err := h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusReady); err != nil {
		h.logger.Error(ctx, "Failed to update preview release status", logging.Error("db_error", err))
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Failed to update release")
		return
	}

	h.logger.Info(ctx, "Preview build completed successfully",
		logging.String("preview_id", preview.ID.String()),
		logging.String("image_uri", buildResult.ImageURI))

	// Update preview status to deploying
	h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusDeploying, "Deploying to Kubernetes")

	// Create preview-specific environment/namespace
	previewNamespace := "enclii-preview-" + preview.PreviewSubdomain

	// Create deployment record for preview
	deployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     release.ID,
		EnvironmentID: uuid.Nil, // Preview environments don't use standard environments
		Replicas:      1,
		Status:        types.DeploymentStatusPending,
		Health:        types.HealthStatusUnknown,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.repos.Deployments.Create(deployment); err != nil {
		h.logger.Error(ctx, "Failed to create preview deployment",
			logging.Error("db_error", err),
			logging.String("preview_id", preview.ID.String()))
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Failed to create deployment")
		return
	}

	// Update preview with deployment ID
	if err := h.repos.PreviewEnvironments.UpdateDeployment(ctx, preview.ID, deployment.ID); err != nil {
		h.logger.Warn(ctx, "Failed to link deployment to preview", logging.Error("error", err))
	}

	// Generate preview-specific Ingress with the preview subdomain
	previewDomains := []types.CustomDomain{
		{
			Domain:     preview.PreviewSubdomain + ".preview.enclii.app",
			TLSEnabled: true,
			TLSIssuer:  "letsencrypt-prod",
		},
	}

	// Reconcile preview deployment using the service reconciler
	reconcileReq := &previewReconcileRequest{
		Service:       service,
		Release:       release,
		Deployment:    deployment,
		CustomDomains: previewDomains,
		Namespace:     previewNamespace,
	}

	if result := h.reconcilePreviewDeployment(ctx, reconcileReq); !result.Success {
		h.logger.Error(ctx, "Failed to reconcile preview deployment",
			logging.String("preview_id", preview.ID.String()),
			logging.String("error", result.Message))
		h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusFailed, "Deploy failed: "+result.Message)
		h.repos.Deployments.UpdateStatus(deployment.ID, types.DeploymentStatusFailed, types.HealthStatusUnhealthy)
		return
	}

	// Update statuses to active
	h.repos.Deployments.UpdateStatus(deployment.ID, types.DeploymentStatusRunning, types.HealthStatusHealthy)
	h.repos.PreviewEnvironments.UpdateStatus(ctx, preview.ID, types.PreviewStatusActive, "Preview deployed successfully")

	h.logger.Info(ctx, "Preview environment deployed successfully",
		logging.String("preview_id", preview.ID.String()),
		logging.String("preview_url", preview.PreviewURL),
		logging.Int("pr_number", preview.PRNumber))

	// Post GitHub PR comment with preview URL (async)
	go h.postGitHubPRComment(service, preview)
}

// previewReconcileRequest holds data needed to reconcile a preview deployment
type previewReconcileRequest struct {
	Service       *types.Service
	Release       *types.Release
	Deployment    *types.Deployment
	CustomDomains []types.CustomDomain
	Namespace     string
}

// previewReconcileResult represents the result of a preview reconciliation
type previewReconcileResult struct {
	Success bool
	Message string
}

// reconcilePreviewDeployment deploys a preview to Kubernetes
func (h *Handler) reconcilePreviewDeployment(ctx context.Context, req *previewReconcileRequest) *previewReconcileResult {
	// Use the service reconciler to deploy
	reconcileReq := &struct {
		Service       *types.Service
		Release       *types.Release
		Deployment    *types.Deployment
		CustomDomains []types.CustomDomain
		Routes        []types.Route
		EnvVars       map[string]string
	}{
		Service:       req.Service,
		Release:       req.Release,
		Deployment:    req.Deployment,
		CustomDomains: req.CustomDomains,
		Routes:        nil,
		EnvVars:       map[string]string{},
	}

	// Get user env vars for this service (previews inherit from parent)
	envVarsList, err := h.repos.EnvVars.List(ctx, req.Service.ID, nil)
	if err != nil {
		h.logger.Warn(ctx, "Failed to get env vars for preview", logging.Error("error", err))
	} else {
		// Convert env vars list to map
		for _, ev := range envVarsList {
			reconcileReq.EnvVars[ev.Key] = ev.Value
		}
	}

	// Add preview-specific env vars
	reconcileReq.EnvVars["ENCLII_PREVIEW_URL"] = "https://" + req.CustomDomains[0].Domain
	reconcileReq.EnvVars["ENCLII_IS_PREVIEW"] = "true"

	// Schedule reconciliation
	if err := h.reconciler.ScheduleReconciliation(req.Deployment.ID.String(), 1); err != nil {
		h.logger.Warn(context.Background(), "Reconciler queue full, work queued for retry",
			logging.String("deployment_id", req.Deployment.ID.String()),
			logging.Error("queue_error", err))
	}

	return &previewReconcileResult{
		Success: true,
		Message: "Preview deployment scheduled",
	}
}

// postGitHubPRComment posts a comment on the GitHub PR with the preview URL
func (h *Handler) postGitHubPRComment(service *types.Service, preview *types.PreviewEnvironment) {
	ctx := context.Background()

	// Only post comment if GitHub token is configured
	if h.config.GitHubToken == "" {
		h.logger.Debug(ctx, "Skipping PR comment - no GitHub token configured")
		return
	}

	// Parse owner/repo from git_repo URL
	owner, repo := parseGitHubRepo(service.GitRepo)
	if owner == "" || repo == "" {
		h.logger.Warn(ctx, "Could not parse GitHub owner/repo from service git_repo",
			logging.String("git_repo", service.GitRepo))
		return
	}

	h.logger.Info(ctx, "Posting preview URL to GitHub PR",
		logging.String("owner", owner),
		logging.String("repo", repo),
		logging.Int("pr_number", preview.PRNumber),
		logging.String("preview_url", preview.PreviewURL))

	// Build the comment body with useful information
	commentMarker := "<!-- enclii-preview-comment -->"
	commentBody := fmt.Sprintf(`%s
## 🚀 Preview Deployment

| Environment | Status |
|------------|--------|
| **Preview URL** | [%s](%s) |
| **Branch** | %s |
| **Commit** | %s |

---

<details>
<summary>📋 Preview Details</summary>

- **Service**: %s
- **Created**: %s
- **Auto-sleep**: %d minutes after inactivity

</details>

*Deployed with [Enclii](https://enclii.dev)*`,
		commentMarker,
		preview.PreviewURL,
		preview.PreviewURL,
		preview.PRBranch,
		preview.CommitSHA[:7],
		service.Name,
		preview.CreatedAt.Format("2006-01-02 15:04 UTC"),
		preview.AutoSleepAfter,
	)

	// Check if we already have a comment on this PR (update it instead of creating new)
	existingComment, err := h.findExistingPreviewComment(ctx, owner, repo, preview.PRNumber, commentMarker)
	if err != nil {
		h.logger.Warn(ctx, "Failed to check for existing comment",
			logging.Error("error", err))
	}

	if existingComment != nil {
		// Update existing comment
		if err := h.updateGitHubComment(ctx, owner, repo, existingComment.ID, commentBody); err != nil {
			h.logger.Error(ctx, "Failed to update GitHub PR comment",
				logging.Int("pr_number", preview.PRNumber),
				logging.Error("error", err))
			return
		}
		h.logger.Info(ctx, "Updated existing GitHub PR comment",
			logging.Int("pr_number", preview.PRNumber),
			logging.String("comment_id", fmt.Sprintf("%d", existingComment.ID)))
	} else {
		// Create new comment
		commentID, err := h.createGitHubComment(ctx, owner, repo, preview.PRNumber, commentBody)
		if err != nil {
			h.logger.Error(ctx, "Failed to create GitHub PR comment",
				logging.Int("pr_number", preview.PRNumber),
				logging.Error("error", err))
			return
		}
		h.logger.Info(ctx, "Created GitHub PR comment",
			logging.Int("pr_number", preview.PRNumber),
			logging.String("comment_id", fmt.Sprintf("%d", commentID)))
	}
}

// existingGitHubComment represents a GitHub comment for lookup
type existingGitHubComment struct {
	ID int64
}

// findExistingPreviewComment checks if we already posted a preview comment
func (h *Handler) findExistingPreviewComment(ctx context.Context, owner, repo string, prNumber int, marker string) (*existingGitHubComment, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=100", owner, repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+h.config.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, err
	}

	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return &existingGitHubComment{ID: c.ID}, nil
		}
	}

	return nil, nil
}

// createGitHubComment creates a new comment on a GitHub PR
func (h *Handler) createGitHubComment(ctx context.Context, owner, repo string, prNumber int, body string) (int64, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber)

	payload := struct {
		Body string `json:"body"`
	}{Body: body}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+h.config.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.ID, nil
}

// updateGitHubComment updates an existing comment on a GitHub PR
func (h *Handler) updateGitHubComment(ctx context.Context, owner, repo string, commentID int64, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", owner, repo, commentID)

	payload := struct {
		Body string `json:"body"`
	}{Body: body}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.config.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// parseGitHubRepo extracts owner and repo from a GitHub URL
func parseGitHubRepo(gitRepo string) (string, string) {
	// Handle various formats:
	// https://github.com/owner/repo.git
	// https://github.com/owner/repo
	// git@github.com:owner/repo.git
	// owner/repo
	gitRepo = strings.TrimSuffix(gitRepo, ".git")
	gitRepo = strings.TrimPrefix(gitRepo, "https://github.com/")
	gitRepo = strings.TrimPrefix(gitRepo, "git@github.com:")

	parts := strings.Split(gitRepo, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}

// cleanupPreviewResources cleans up resources for a closed preview environment
func (h *Handler) cleanupPreviewResources(preview *types.PreviewEnvironment) {
	ctx := context.Background()
	h.logger.Info(ctx, "Starting cleanup for preview environment",
		logging.String("preview_id", preview.ID.String()),
		logging.String("subdomain", preview.PreviewSubdomain))

	// Get the service for namespace calculation
	service, err := h.repos.Services.GetByID(preview.ServiceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service for preview cleanup",
			logging.String("preview_id", preview.ID.String()),
			logging.Error("error", err))
		return
	}

	// Calculate preview namespace
	previewNamespace := "enclii-preview-" + preview.PreviewSubdomain

	// Delete Kubernetes resources using the service reconciler
	if err := h.serviceReconciler.Delete(ctx, previewNamespace, service.Name); err != nil {
		h.logger.Error(ctx, "Failed to delete K8s resources for preview",
			logging.String("preview_id", preview.ID.String()),
			logging.String("namespace", previewNamespace),
			logging.Error("error", err))
		// Continue cleanup even if this fails
	} else {
		h.logger.Info(ctx, "Deleted K8s deployment and service for preview",
			logging.String("preview_id", preview.ID.String()),
			logging.String("namespace", previewNamespace))
	}

	// Delete the preview namespace itself (if empty)
	if err := h.deletePreviewNamespace(ctx, previewNamespace); err != nil {
		h.logger.Warn(ctx, "Failed to delete preview namespace (may not be empty)",
			logging.String("namespace", previewNamespace),
			logging.Error("error", err))
	}

	// Delete the preview ingress if it exists
	if err := h.deletePreviewIngress(ctx, previewNamespace, service.Name); err != nil {
		h.logger.Warn(ctx, "Failed to delete preview ingress",
			logging.String("namespace", previewNamespace),
			logging.Error("error", err))
	}

	h.logger.Info(ctx, "Preview environment cleanup completed",
		logging.String("preview_id", preview.ID.String()),
		logging.String("namespace", previewNamespace))
}

// deletePreviewNamespace deletes the preview-specific namespace
func (h *Handler) deletePreviewNamespace(ctx context.Context, namespace string) error {
	return h.k8sClient.Clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
}

// deletePreviewIngress deletes the ingress for a preview environment
func (h *Handler) deletePreviewIngress(ctx context.Context, namespace, serviceName string) error {
	return h.k8sClient.Clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
}

// itoa converts an int to string
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
