package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

func TestAPIClient_CreateProject(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/projects", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "enclii-cli/1.0.0", r.Header.Get("User-Agent"))

		// Parse request body
		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "Test Project", req["name"])
		assert.Equal(t, "test-project", req["slug"])

		// Return response
		project := types.Project{
			ID:          "project-123",
			Name:        req["name"],
			Slug:        req["slug"],
			Description: req["description"],
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(project)
	}))
	defer server.Close()

	// Create client
	client := NewAPIClient(server.URL, "test-token")

	// Test successful creation
	ctx := context.Background()
	project, err := client.CreateProject(ctx, "Test Project", "test-project")
	
	require.NoError(t, err)
	assert.NotNil(t, project)
	assert.Equal(t, "project-123", project.ID)
	assert.Equal(t, "Test Project", project.Name)
	assert.Equal(t, "test-project", project.Slug)
}

func TestAPIClient_GetProject(t *testing.T) {
	project := &types.Project{
		ID:          "project-123",
		Name:        "Test Project",
		Slug:        "test-project",
		Description: "A test project",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/projects/test-project", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(project)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	result, err := client.GetProject(ctx, "test-project")
	
	require.NoError(t, err)
	assert.Equal(t, project.ID, result.ID)
	assert.Equal(t, project.Name, result.Name)
	assert.Equal(t, project.Slug, result.Slug)
}

func TestAPIClient_ListProjects(t *testing.T) {
	projects := []*types.Project{
		{
			ID:        "project-1",
			Name:      "Project 1",
			Slug:      "project-1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "project-2",
			Name:      "Project 2", 
			Slug:      "project-2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/projects", r.URL.Path)

		response := struct {
			Projects []*types.Project `json:"projects"`
		}{
			Projects: projects,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	result, err := client.ListProjects(ctx)
	
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "project-1", result[0].ID)
	assert.Equal(t, "project-2", result[1].ID)
}

func TestAPIClient_BuildService(t *testing.T) {
	release := &types.Release{
		ID:        "release-123",
		ServiceID: "service-123", 
		Version:   "v1.0.0",
		ImageURL:  "registry.example.com/service:v1.0.0",
		GitSHA:    "abc123def456",
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/services/service-123/build", r.URL.Path)

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "abc123def456", req["git_sha"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	result, err := client.BuildService(ctx, "service-123", "abc123def456")
	
	require.NoError(t, err)
	assert.Equal(t, release.ID, result.ID)
	assert.Equal(t, release.Version, result.Version)
	assert.Equal(t, release.Status, result.Status)
}

func TestAPIClient_DeployService(t *testing.T) {
	deployment := &types.Deployment{
		ID:          "deployment-123",
		ServiceID:   "service-123",
		ReleaseID:   "release-123",
		Status:      types.DeploymentStatusPending,
		Environment: map[string]string{"ENV": "production"},
		Replicas:    2,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/services/service-123/deploy", r.URL.Path)

		var req DeployRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "release-123", req.ReleaseID)
		assert.Equal(t, map[string]string{"ENV": "production"}, req.Environment)
		assert.Equal(t, 2, req.Replicas)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(deployment)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	deployReq := DeployRequest{
		ReleaseID:   "release-123",
		Environment: map[string]string{"ENV": "production"},
		Replicas:    2,
	}
	
	result, err := client.DeployService(ctx, "service-123", deployReq)
	
	require.NoError(t, err)
	assert.Equal(t, deployment.ID, result.ID)
	assert.Equal(t, deployment.Status, result.Status)
	assert.Equal(t, deployment.Replicas, result.Replicas)
}

func TestAPIClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Project not found",
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	_, err := client.GetProject(ctx, "nonexistent")
	
	require.Error(t, err)
	apiErr, ok := err.(APIError)
	require.True(t, ok)
	assert.Equal(t, 404, apiErr.StatusCode)
	assert.Equal(t, "Project not found", apiErr.Message)
}

func TestAPIClient_Health(t *testing.T) {
	health := &HealthResponse{
		Status:  "healthy",
		Service: "switchyard-api",
		Version: "1.0.0",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/health", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	
	ctx := context.Background()
	result, err := client.Health(ctx)
	
	require.NoError(t, err)
	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, "switchyard-api", result.Service)
	assert.Equal(t, "1.0.0", result.Version)
}

// Test authentication header handling
func TestAPIClient_Authentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HealthResponse{Status: "healthy"})
	}))
	defer server.Close()

	// Test with valid token
	client := NewAPIClient(server.URL, "secret-token")
	ctx := context.Background()
	
	_, err := client.Health(ctx)
	require.NoError(t, err)

	// Test with invalid token
	client = NewAPIClient(server.URL, "invalid-token")
	_, err = client.Health(ctx)
	
	require.Error(t, err)
	apiErr, ok := err.(APIError)
	require.True(t, ok)
	assert.Equal(t, 401, apiErr.StatusCode)
}

// Benchmark tests
func BenchmarkAPIClient_GetProject(b *testing.B) {
	project := &types.Project{
		ID:   "project-123",
		Name: "Test Project",
		Slug: "test-project",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(project)
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetProject(ctx, "test-project")
		if err != nil {
			b.Fatal(err)
		}
	}
}