package webhook

import (
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
	"go.uber.org/zap"
)

// GitHubHandler handles GitHub webhook events
type GitHubHandler struct {
	secret string
	logger *zap.Logger
}

// NewGitHubHandler creates a new GitHub webhook handler
func NewGitHubHandler(secret string, logger *zap.Logger) *GitHubHandler {
	return &GitHubHandler{
		secret: secret,
		logger: logger,
	}
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
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

// Handle processes incoming GitHub webhooks
func (h *GitHubHandler) Handle(c *gin.Context) {
	// Validate signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if h.secret != "" && signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	if h.secret != "" {
		if !h.validateSignature(body, signature) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
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

	h.logger.Info("webhook received",
		zap.String("provider", payload.Provider),
		zap.String("event", payload.Event),
		zap.String("repository", payload.Repository),
		zap.String("branch", payload.Branch),
		zap.String("commit", payload.CommitSHA[:8]),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "webhook received",
		"repository": payload.Repository,
		"branch":     payload.Branch,
		"commit":     payload.CommitSHA,
	})
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

	// Only build on specific actions
	validActions := map[string]bool{
		"opened":      true,
		"synchronize": true,
		"reopened":    true,
	}

	if !validActions[pr.Action] {
		return nil, fmt.Errorf("ignoring PR action: %s", pr.Action)
	}

	return &queue.WebhookPayload{
		Provider:   "github",
		Event:      "pull_request",
		Repository: pr.Repository.CloneURL,
		Branch:     pr.PullRequest.Head.Ref,
		CommitSHA:  pr.PullRequest.Head.SHA,
		Author:     "", // Would need to fetch from API
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
