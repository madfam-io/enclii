package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

type APIClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

func (e APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %d: %s (%s)", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Projects
func (c *APIClient) CreateProject(ctx context.Context, name, slug string) (*types.Project, error) {
	payload := map[string]string{
		"name": name,
		"slug": slug,
	}

	var project types.Project
	if err := c.post(ctx, "/v1/projects", payload, &project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return &project, nil
}

func (c *APIClient) GetProject(ctx context.Context, slug string) (*types.Project, error) {
	var project types.Project
	if err := c.get(ctx, fmt.Sprintf("/v1/projects/%s", slug), &project); err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &project, nil
}

func (c *APIClient) ListProjects(ctx context.Context) ([]*types.Project, error) {
	var response struct {
		Projects []*types.Project `json:"projects"`
	}

	if err := c.get(ctx, "/v1/projects", &response); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	return response.Projects, nil
}

// Services
func (c *APIClient) CreateService(ctx context.Context, projectSlug string, service *types.Service) (*types.Service, error) {
	payload := map[string]interface{}{
		"name":         service.Name,
		"git_repo":     service.GitRepo,
		"build_config": service.BuildConfig,
	}

	var createdService types.Service
	if err := c.post(ctx, fmt.Sprintf("/v1/projects/%s/services", projectSlug), payload, &createdService); err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return &createdService, nil
}

func (c *APIClient) GetService(ctx context.Context, serviceID string) (*types.Service, error) {
	var service types.Service
	if err := c.get(ctx, fmt.Sprintf("/v1/services/%s", serviceID), &service); err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	return &service, nil
}

func (c *APIClient) ListServices(ctx context.Context, projectSlug string) ([]*types.Service, error) {
	var response struct {
		Services []*types.Service `json:"services"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/projects/%s/services", projectSlug), &response); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	return response.Services, nil
}

// Build & Deploy
func (c *APIClient) BuildService(ctx context.Context, serviceID, gitSHA string) (*types.Release, error) {
	payload := map[string]string{
		"git_sha": gitSHA,
	}

	var release types.Release
	if err := c.post(ctx, fmt.Sprintf("/v1/services/%s/build", serviceID), payload, &release); err != nil {
		return nil, fmt.Errorf("failed to build service: %w", err)
	}

	return &release, nil
}

func (c *APIClient) DeployService(ctx context.Context, serviceID string, req DeployRequest) (*types.Deployment, error) {
	var deployment types.Deployment
	if err := c.post(ctx, fmt.Sprintf("/v1/services/%s/deploy", serviceID), req, &deployment); err != nil {
		return nil, fmt.Errorf("failed to deploy service: %w", err)
	}

	return &deployment, nil
}

func (c *APIClient) GetServiceStatus(ctx context.Context, serviceID string) (*ServiceStatus, error) {
	var status ServiceStatus
	if err := c.get(ctx, fmt.Sprintf("/v1/services/%s/status", serviceID), &status); err != nil {
		return nil, fmt.Errorf("failed to get service status: %w", err)
	}

	return &status, nil
}

func (c *APIClient) ListReleases(ctx context.Context, serviceID string) ([]*types.Release, error) {
	var response struct {
		Releases []*types.Release `json:"releases"`
	}

	if err := c.get(ctx, fmt.Sprintf("/v1/services/%s/releases", serviceID), &response); err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	return response.Releases, nil
}

// Logs
func (c *APIClient) GetLogs(ctx context.Context, deploymentID string, opts LogOptions) ([]LogLine, error) {
	params := url.Values{}
	if opts.Follow {
		params.Set("follow", "true")
	}
	if opts.Lines > 0 {
		params.Set("lines", fmt.Sprintf("%d", opts.Lines))
	}
	if opts.Since != nil {
		params.Set("since", opts.Since.Format(time.RFC3339))
	}

	var response struct {
		Logs []LogLine `json:"logs"`
	}

	endpoint := fmt.Sprintf("/v1/deployments/%s/logs", deploymentID)
	if params.Encode() != "" {
		endpoint += "?" + params.Encode()
	}

	if err := c.get(ctx, endpoint, &response); err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	return response.Logs, nil
}

// Rollback
func (c *APIClient) RollbackDeployment(ctx context.Context, deploymentID string, req RollbackRequest) error {
	if err := c.post(ctx, fmt.Sprintf("/v1/deployments/%s/rollback", deploymentID), req, nil); err != nil {
		return fmt.Errorf("failed to rollback deployment: %w", err)
	}

	return nil
}

// Health check
func (c *APIClient) Health(ctx context.Context) (*HealthResponse, error) {
	var health HealthResponse
	if err := c.get(ctx, "/health", &health); err != nil {
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	return &health, nil
}

// HTTP helpers
func (c *APIClient) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	return c.doRequest(req, result)
}

func (c *APIClient) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doRequest(req, result)
}

func (c *APIClient) doRequest(req *http.Request, result interface{}) error {
	// Add authentication if token is available
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	// Add common headers
	req.Header.Set("User-Agent", "enclii-cli/1.0.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			// Fallback to generic error
			return APIError{
				StatusCode: resp.StatusCode,
				Message:    string(body),
			}
		}
		apiErr.StatusCode = resp.StatusCode
		return apiErr
	}

	// Parse success response
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Request/Response types
type DeployRequest struct {
	Environment string `json:"environment"`
	ReleaseID   string `json:"release_id,omitempty"`
	Wait        bool   `json:"wait"`
}

type RollbackRequest struct {
	ToRelease string `json:"to_release,omitempty"`
}

type LogOptions struct {
	Follow bool
	Lines  int
	Since  *time.Time
}

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Pod       string    `json:"pod"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"`
}

type ServiceStatus struct {
	ServiceID   string                   `json:"service_id"`
	Environment string                   `json:"environment"`
	Status      types.DeploymentStatus   `json:"status"`
	Health      types.HealthStatus       `json:"health"`
	Replicas    int                      `json:"replicas"`
	Version     string                   `json:"version"`
	Uptime      time.Duration            `json:"uptime"`
	LastDeploy  time.Time                `json:"last_deploy"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}