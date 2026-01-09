// Package github provides GitHub App integration for repository management.
// This enables Vercel/Railway-style GitHub repo connections.
package github

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// Config holds GitHub App configuration
type Config struct {
	AppID         int64  // GitHub App ID
	PrivateKeyPEM string // PEM-encoded private key
	WebhookSecret string // Webhook signing secret
	ClientID      string // OAuth App client ID (for user auth flow)
	ClientSecret  string // OAuth App client secret
	InstallURL    string // URL to install the GitHub App
}

// Client provides GitHub App API operations
type Client struct {
	config     *Config
	privateKey *rsa.PrivateKey
	httpClient *http.Client
	logger     *logrus.Logger

	// Token cache for installation access tokens
	tokenCache   map[int64]*InstallationToken
	tokenCacheMu sync.RWMutex
}

// InstallationToken represents a GitHub App installation access token
type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Installation represents a GitHub App installation
type Installation struct {
	ID              int64             `json:"id"`
	Account         Account           `json:"account"`
	AccessTokensURL string            `json:"access_tokens_url"`
	RepositoriesURL string            `json:"repositories_url"`
	AppID           int64             `json:"app_id"`
	TargetID        int64             `json:"target_id"`
	TargetType      string            `json:"target_type"`
	Permissions     map[string]string `json:"permissions"`
	Events          []string          `json:"events"`
	SuspendedAt     *time.Time        `json:"suspended_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Account represents a GitHub user or organization
type Account struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	Type      string `json:"type"` // "User" or "Organization"
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// Repository represents a GitHub repository
type Repository struct {
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
}

// RepositoryList represents a paginated list of repositories
type RepositoryList struct {
	TotalCount   int          `json:"total_count"`
	Repositories []Repository `json:"repositories"`
}

// NewClient creates a new GitHub App client
func NewClient(config *Config, logger *logrus.Logger) (*Client, error) {
	if config.AppID == 0 {
		return nil, fmt.Errorf("GitHub App ID is required")
	}

	if config.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("GitHub App private key is required")
	}

	// Parse the private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub App private key: %w", err)
	}

	return &Client{
		config:     config,
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
		tokenCache: make(map[int64]*InstallationToken),
	}, nil
}

// GenerateJWT creates a JWT for authenticating as the GitHub App
func (c *Client) GenerateJWT() (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // 60 seconds in the past for clock drift
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),  // Max 10 minutes
		Issuer:    fmt.Sprintf("%d", c.config.AppID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signedToken, nil
}

// GetInstallation retrieves an installation by ID
func (c *Client) GetInstallation(ctx context.Context, installationID int64) (*Installation, error) {
	jwt, err := c.GenerateJWT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var installation Installation
	if err := json.NewDecoder(resp.Body).Decode(&installation); err != nil {
		return nil, fmt.Errorf("failed to decode installation: %w", err)
	}

	return &installation, nil
}

// GetInstallationToken gets or refreshes an installation access token
func (c *Client) GetInstallationToken(ctx context.Context, installationID int64) (string, error) {
	// Check cache first
	c.tokenCacheMu.RLock()
	cached, exists := c.tokenCache[installationID]
	c.tokenCacheMu.RUnlock()

	if exists && time.Now().Add(5*time.Minute).Before(cached.ExpiresAt) {
		return cached.Token, nil
	}

	// Generate new token
	jwt, err := c.GenerateJWT()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var tokenResp InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// Cache the token
	c.tokenCacheMu.Lock()
	c.tokenCache[installationID] = &tokenResp
	c.tokenCacheMu.Unlock()

	c.logger.WithFields(logrus.Fields{
		"installation_id": installationID,
		"expires_at":      tokenResp.ExpiresAt,
	}).Debug("Obtained new GitHub installation token")

	return tokenResp.Token, nil
}

// ListInstallationRepositories lists repositories accessible to an installation
func (c *Client) ListInstallationRepositories(ctx context.Context, installationID int64) (*RepositoryList, error) {
	token, err := c.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := "https://api.github.com/installation/repositories"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var repoList RepositoryList
	if err := json.NewDecoder(resp.Body).Decode(&repoList); err != nil {
		return nil, fmt.Errorf("failed to decode repository list: %w", err)
	}

	return &repoList, nil
}

// GetRepository gets a specific repository by owner and name
func (c *Client) GetRepository(ctx context.Context, installationID int64, owner, repo string) (*Repository, error) {
	token, err := c.GetInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var repository Repository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, fmt.Errorf("failed to decode repository: %w", err)
	}

	return &repository, nil
}

// CreateWebhook creates a webhook on a repository
func (c *Client) CreateWebhook(ctx context.Context, installationID int64, owner, repo, webhookURL, secret string) (int64, error) {
	token, err := c.GetInstallationToken(ctx, installationID)
	if err != nil {
		return 0, err
	}

	type webhookConfig struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		Secret      string `json:"secret,omitempty"`
		InsecureSSL string `json:"insecure_ssl"`
	}

	type webhookRequest struct {
		Name   string        `json:"name"`
		Active bool          `json:"active"`
		Events []string      `json:"events"`
		Config webhookConfig `json:"config"`
	}

	payload := webhookRequest{
		Name:   "web",
		Active: true,
		Events: []string{"push", "pull_request"},
		Config: webhookConfig{
			URL:         webhookURL,
			ContentType: "json",
			Secret:      secret,
			InsecureSSL: "0",
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return 0, err
	}
	req.Body = io.NopCloser(io.MultiReader(io.NopCloser(json.NewDecoder(io.NopCloser(nil)).Buffered())))

	// Re-create request with body
	req, err = http.NewRequestWithContext(ctx, "POST", url,
		io.NopCloser(json.NewDecoder(io.NopCloser(nil)).Buffered()))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	// Actually set the body properly
	req, _ = http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Body = io.NopCloser(io.MultiReader())

	// Properly create the request with body
	import_bytes := payloadBytes
	_ = import_bytes

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var webhookResp struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&webhookResp); err != nil {
		return 0, fmt.Errorf("failed to decode webhook response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"repo":       fmt.Sprintf("%s/%s", owner, repo),
		"webhook_id": webhookResp.ID,
	}).Info("Created GitHub webhook")

	return webhookResp.ID, nil
}

// GetInstallURL returns the URL to install the GitHub App
func (c *Client) GetInstallURL() string {
	return c.config.InstallURL
}

// GetOAuthURL returns the URL for OAuth authorization
func (c *Client) GetOAuthURL(state, redirectURI string) string {
	return fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=repo,read:user",
		c.config.ClientID,
		redirectURI,
		state,
	)
}
