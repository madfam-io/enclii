package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// GitHubWorkflowRunEvent represents a GitHub workflow_run webhook payload
type GitHubWorkflowRunEvent struct {
	Action      string `json:"action"` // requested, in_progress, completed
	WorkflowRun struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		WorkflowID   int64  `json:"workflow_id"`
		RunNumber    int    `json:"run_number"`
		Status       string `json:"status"`     // queued, in_progress, completed
		Conclusion   string `json:"conclusion"` // success, failure, cancelled, skipped, etc.
		HTMLURL      string `json:"html_url"`
		Event        string `json:"event"` // push, pull_request, etc.
		HeadBranch   string `json:"head_branch"`
		HeadSHA      string `json:"head_sha"`
		RunStartedAt string `json:"run_started_at"`
		UpdatedAt    string `json:"updated_at"`
		Actor        struct {
			Login string `json:"login"`
		} `json:"actor"`
	} `json:"workflow_run"`
	Workflow struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"workflow"`
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

// handleGitHubWorkflowRun processes workflow_run events for CI status tracking
func (h *Handler) handleGitHubWorkflowRun(c *gin.Context, ctx context.Context, body []byte) {
	var event GitHubWorkflowRunEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error(ctx, "Failed to parse workflow_run event", logging.Error("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow_run event payload"})
		return
	}

	h.logger.Info(ctx, "Processing workflow_run event",
		logging.String("action", event.Action),
		logging.String("workflow_name", event.WorkflowRun.Name),
		logging.String("run_id", fmt.Sprintf("%d", event.WorkflowRun.ID)),
		logging.String("status", event.WorkflowRun.Status),
		logging.String("conclusion", event.WorkflowRun.Conclusion),
		logging.String("repo", event.Repository.FullName),
		logging.String("commit_sha", event.WorkflowRun.HeadSHA))

	// Find ALL services matching this git repo
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
		h.logger.Info(ctx, "No services found for repository (workflow_run)",
			logging.String("repo", event.Repository.FullName))
		c.JSON(http.StatusOK, gin.H{
			"message": "No services registered for this repository",
			"repo":    event.Repository.FullName,
		})
		return
	}

	// Map GitHub status/action to our CIRunStatus
	var status types.CIRunStatus
	switch event.Action {
	case "requested":
		status = types.CIRunStatusQueued
	case "in_progress":
		status = types.CIRunStatusInProgress
	case "completed":
		status = types.CIRunStatusCompleted
	default:
		// Acknowledge but don't process unknown actions
		c.JSON(http.StatusOK, gin.H{
			"message": "Workflow run action not handled",
			"action":  event.Action,
		})
		return
	}

	// Map GitHub conclusion to our CIRunConclusion
	var conclusion *types.CIRunConclusion
	if event.WorkflowRun.Conclusion != "" {
		c := types.CIRunConclusion(event.WorkflowRun.Conclusion)
		conclusion = &c
	}

	// Parse timestamps
	var startedAt, completedAt *time.Time
	if event.WorkflowRun.RunStartedAt != "" {
		if t, err := time.Parse(time.RFC3339, event.WorkflowRun.RunStartedAt); err == nil {
			startedAt = &t
		}
	}
	if status == types.CIRunStatusCompleted && event.WorkflowRun.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, event.WorkflowRun.UpdatedAt); err == nil {
			completedAt = &t
		}
	}

	// Create or update CI run record for each matching service
	var results []map[string]any
	for _, service := range services {
		ciRun := &types.CIRun{
			ServiceID:    service.ID,
			CommitSHA:    event.WorkflowRun.HeadSHA,
			WorkflowName: event.WorkflowRun.Name,
			WorkflowID:   event.WorkflowRun.WorkflowID,
			RunID:        event.WorkflowRun.ID,
			RunNumber:    event.WorkflowRun.RunNumber,
			Status:       status,
			Conclusion:   conclusion,
			HTMLURL:      event.WorkflowRun.HTMLURL,
			Branch:       event.WorkflowRun.HeadBranch,
			EventType:    event.WorkflowRun.Event,
			Actor:        event.WorkflowRun.Actor.Login,
			StartedAt:    startedAt,
			CompletedAt:  completedAt,
		}

		if err := h.repos.CIRuns.Upsert(ctx, ciRun); err != nil {
			h.logger.Error(ctx, "Failed to upsert CI run",
				logging.String("service", service.Name),
				logging.String("run_id", fmt.Sprintf("%d", event.WorkflowRun.ID)),
				logging.Error("db_error", err))
			results = append(results, map[string]any{
				"service": service.Name,
				"status":  "failed: " + err.Error(),
			})
			continue
		}

		h.logger.Info(ctx, "CI run status updated",
			logging.String("service", service.Name),
			logging.String("run_id", fmt.Sprintf("%d", event.WorkflowRun.ID)),
			logging.String("status", string(status)))

		results = append(results, map[string]any{
			"service":  service.Name,
			"run_id":   event.WorkflowRun.ID,
			"status":   string(status),
			"workflow": event.WorkflowRun.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("Workflow run status processed for %d services", len(results)),
		"action":        event.Action,
		"workflow_name": event.WorkflowRun.Name,
		"run_id":        event.WorkflowRun.ID,
		"status":        string(status),
		"conclusion":    event.WorkflowRun.Conclusion,
		"services":      results,
	})
}
