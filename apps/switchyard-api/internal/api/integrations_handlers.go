// Package api provides HTTP handlers for the Switchyard API.
// This file contains handlers for third-party integrations (GitHub, etc.)
// that use OAuth tokens from Janua.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
)

// JanuaIntegrationToken represents the response from Janua's integrations API
type JanuaIntegrationToken struct {
	Provider        string     `json:"provider"`
	AccessToken     string     `json:"access_token"`
	RefreshToken    *string    `json:"refresh_token,omitempty"`
	TokenExpiresAt  *time.Time `json:"token_expires_at,omitempty"`
	ProviderUserID  *string    `json:"provider_user_id,omitempty"`
	ProviderEmail   *string    `json:"provider_email,omitempty"`
	LinkedAt        time.Time  `json:"linked_at"`
}

// JanuaIntegrationStatus represents the status response from Janua
type JanuaIntegrationStatus struct {
	Provider       string     `json:"provider"`
	Linked         bool       `json:"linked"`
	ProviderEmail  *string    `json:"provider_email,omitempty"`
	LinkedAt       *time.Time `json:"linked_at,omitempty"`
	CanAccessRepos bool       `json:"can_access_repos"`
}

// GitHubRepository represents a GitHub repository from the API
type GitHubRepository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	CloneURL      string    `json:"clone_url"`
	HTMLURL       string    `json:"html_url"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	UpdatedAt     time.Time `json:"updated_at"`
	Owner         struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

// GitHubReposResponse is the API response for listing repositories
type GitHubReposResponse struct {
	Repositories []GitHubRepository `json:"repositories"`
	TotalCount   int                `json:"total_count"`
}

// IntegrationStatusResponse is the API response for checking integration status
type IntegrationStatusResponse struct {
	Provider       string  `json:"provider"`
	Linked         bool    `json:"linked"`
	ProviderEmail  *string `json:"provider_email,omitempty"`
	CanAccessRepos bool    `json:"can_access_repos"`
	Message        string  `json:"message,omitempty"`
}

// getJanuaToken retrieves the user's OAuth token for a provider from Janua
func (h *Handler) getJanuaToken(ctx context.Context, provider, jwtToken string) (*JanuaIntegrationToken, error) {
	if h.config.JanuaAPIURL == "" {
		return nil, fmt.Errorf("Janua API URL not configured")
	}

	url := fmt.Sprintf("%s/api/v1/integrations/%s/token", h.config.JanuaAPIURL, provider)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Janua API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GitHub account not linked. Please connect your GitHub account first")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Janua API error: %d - %s", resp.StatusCode, string(body))
	}

	var tokenResp JanuaIntegrationToken
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// getJanuaIntegrationStatus checks if a provider is linked in Janua
func (h *Handler) getJanuaIntegrationStatus(ctx context.Context, provider, jwtToken string) (*JanuaIntegrationStatus, error) {
	if h.config.JanuaAPIURL == "" {
		return nil, fmt.Errorf("Janua API URL not configured")
	}

	url := fmt.Sprintf("%s/api/v1/integrations/%s/status", h.config.JanuaAPIURL, provider)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Janua API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Janua API error: %d - %s", resp.StatusCode, string(body))
	}

	var statusResp JanuaIntegrationStatus
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &statusResp, nil
}

// listGitHubRepos fetches repositories from GitHub using the provided access token
func (h *Handler) listGitHubRepos(ctx context.Context, accessToken string) ([]GitHubRepository, error) {
	// Fetch user repos with pagination (up to 100 per page, we'll fetch first page)
	url := "https://api.github.com/user/repos?visibility=all&affiliation=owner,collaborator,organization_member&sort=updated&per_page=100"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var repos []GitHubRepository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	return repos, nil
}

// GetGitHubStatus returns the GitHub integration status for the current user
// GET /v1/integrations/github/status
func (h *Handler) GetGitHubStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Get the JWT token from the Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}

	jwtToken := strings.TrimPrefix(authHeader, "Bearer ")

	status, err := h.getJanuaIntegrationStatus(ctx, "github", jwtToken)
	if err != nil {
		h.logger.Error(ctx, "Failed to get GitHub integration status", logging.Error("error", err))

		c.JSON(http.StatusOK, IntegrationStatusResponse{
			Provider:       "github",
			Linked:         false,
			CanAccessRepos: false,
			Message:        "Unable to check GitHub status. Please try again.",
		})
		return
	}

	message := ""
	if !status.Linked {
		message = "GitHub account is not connected. Please link your GitHub account to import repositories."
	} else if !status.CanAccessRepos {
		message = "GitHub is connected but lacks repository access. Please re-authorize with repo permissions."
	}

	c.JSON(http.StatusOK, IntegrationStatusResponse{
		Provider:       "github",
		Linked:         status.Linked,
		ProviderEmail:  status.ProviderEmail,
		CanAccessRepos: status.CanAccessRepos,
		Message:        message,
	})
}

// ListGitHubRepos returns the user's GitHub repositories
// GET /v1/integrations/github/repos
func (h *Handler) ListGitHubRepos(c *gin.Context) {
	ctx := c.Request.Context()

	// Get the JWT token from the Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}

	jwtToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Step 1: Get the user's GitHub access token from Janua
	tokenResp, err := h.getJanuaToken(ctx, "github", jwtToken)
	if err != nil {
		h.logger.Error(ctx, "Failed to get GitHub token from Janua", logging.Error("error", err))

		// Return helpful error message based on the error type
		if strings.Contains(err.Error(), "not linked") || strings.Contains(err.Error(), "not connected") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "GitHub account not linked",
				"message": "Please connect your GitHub account first via Settings > Integrations",
				"code":    "GITHUB_NOT_LINKED",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve GitHub credentials",
			"message": "Unable to access your GitHub account. Please try re-linking.",
		})
		return
	}

	// Step 2: Use the GitHub access token to list repositories
	repos, err := h.listGitHubRepos(ctx, tokenResp.AccessToken)
	if err != nil {
		h.logger.Error(ctx, "Failed to list GitHub repositories", logging.Error("error", err))

		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "GitHub access denied",
				"message": "Your GitHub token may have expired. Please re-link your GitHub account.",
				"code":    "GITHUB_TOKEN_EXPIRED",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list repositories",
			"message": "Unable to fetch repositories from GitHub. Please try again.",
		})
		return
	}

	h.logger.Info(ctx, "Listed GitHub repositories", logging.Int("repo_count", len(repos)))

	c.JSON(http.StatusOK, GitHubReposResponse{
		Repositories: repos,
		TotalCount:   len(repos),
	})
}
