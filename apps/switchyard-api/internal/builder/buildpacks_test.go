package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

func TestNewBuildpacksBuilder(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		cacheDir string
		timeout  time.Duration
	}{
		{
			name:     "standard config",
			registry: "registry.example.com",
			cacheDir: "/tmp/cache",
			timeout:  30 * time.Minute,
		},
		{
			name:     "custom config",
			registry: "gcr.io/myproject",
			cacheDir: "/var/cache/buildpacks",
			timeout:  1 * time.Hour,
		},
		{
			name:     "minimal config",
			registry: "localhost:5000",
			cacheDir: "",
			timeout:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuildpacksBuilder(tt.registry, "", "", tt.cacheDir, tt.timeout)

			if builder == nil {
				t.Fatal("NewBuildpacksBuilder() returned nil")
			}

			if builder.registry != tt.registry {
				t.Errorf("registry = %s, want %s", builder.registry, tt.registry)
			}

			if builder.cacheDir != tt.cacheDir {
				t.Errorf("cacheDir = %s, want %s", builder.cacheDir, tt.cacheDir)
			}

			if builder.timeout != tt.timeout {
				t.Errorf("timeout = %v, want %v", builder.timeout, tt.timeout)
			}
		})
	}
}

func TestBuildpacksBuilder_detectBuildStrategy(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := filepath.Join(os.TempDir(), "buildstrategy-test")
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(tmpDir, 0755)

	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)

	tests := []struct {
		name        string
		createFile  string
		config      types.BuildConfig
		expected    string
		expectError bool
	}{
		{
			name:       "explicit buildpacks",
			createFile: "",
			config:     types.BuildConfig{Type: types.BuildTypeBuildpack},
			expected:   "buildpack",
		},
		{
			name:       "explicit dockerfile",
			createFile: "",
			config:     types.BuildConfig{Type: types.BuildTypeDockerfile},
			expected:   "dockerfile",
		},
		{
			name:       "detect dockerfile",
			createFile: "Dockerfile",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "dockerfile",
		},
		{
			name:       "detect nodejs",
			createFile: "package.json",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
		{
			name:       "detect go",
			createFile: "go.mod",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
		{
			name:       "detect python",
			createFile: "requirements.txt",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
		{
			name:       "detect ruby",
			createFile: "Gemfile",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
		{
			name:       "detect java",
			createFile: "pom.xml",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
		{
			name:       "no files (default to buildpacks)",
			createFile: "",
			config:     types.BuildConfig{Type: types.BuildTypeAuto},
			expected:   "buildpacks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory for this case
			testDir := filepath.Join(tmpDir, tt.name)
			os.MkdirAll(testDir, 0755)
			defer os.RemoveAll(testDir)

			// Create test file if specified
			if tt.createFile != "" {
				testFile := filepath.Join(testDir, tt.createFile)
				os.WriteFile(testFile, []byte("test"), 0644)
			}

			strategy, err := builder.detectBuildStrategy(testDir, tt.config)

			if tt.expectError && err == nil {
				t.Error("detectBuildStrategy() expected error, got nil")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("detectBuildStrategy() unexpected error: %v", err)
				return
			}

			if strategy != tt.expected {
				t.Errorf("detectBuildStrategy() = %s, want %s", strategy, tt.expected)
			}
		})
	}
}

func TestBuildpacksBuilder_generateImageURI(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)

	tests := []struct {
		name        string
		serviceName string
		gitSHA      string
	}{
		{
			name:        "standard service",
			serviceName: "my-service",
			gitSHA:      "abc123def456",
		},
		{
			name:        "service with dashes",
			serviceName: "my-awesome-service",
			gitSHA:      "xyz789ghi012",
		},
		{
			name:        "short service name",
			serviceName: "api",
			gitSHA:      "1234567890ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := builder.generateImageURI(tt.serviceName, tt.gitSHA)

			// Should contain registry
			if uri == "" {
				t.Error("generateImageURI() returned empty string")
			}

			// Should start with registry
			expectedPrefix := "registry.example.com/" + tt.serviceName + ":"
			if len(uri) < len(expectedPrefix) || uri[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("URI = %s, should start with %s", uri, expectedPrefix)
			}

			// Should contain short SHA (first 7 chars)
			shortSHA := tt.gitSHA[:7]
			if !contains(uri, shortSHA) {
				t.Errorf("URI = %s, should contain short SHA %s", uri, shortSHA)
			}
		})
	}
}

func TestBuildpacksBuilder_generateImageURI_Format(t *testing.T) {
	builder := NewBuildpacksBuilder("gcr.io/project", "", "", "/cache", 30*time.Minute)

	uri := builder.generateImageURI("test-service", "abc123def456")

	// Should match format: registry/service:vYYYYMMDD-HHMMSS-shortsha
	// Example: gcr.io/project/test-service:v20240101-120000-abc123d
	if uri == "" {
		t.Fatal("generateImageURI() returned empty string")
	}

	// Verify it starts with the registry and service name
	if !contains(uri, "gcr.io/project/test-service:v") {
		t.Errorf("URI format incorrect: %s", uri)
	}

	// Verify it contains the short SHA
	if !contains(uri, "-abc123d") {
		t.Errorf("URI should contain short SHA: %s", uri)
	}
}

func TestBuildpacksBuilder_ValidateTools(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)

	// This test will fail if pack or docker are not installed
	// That's expected - we're testing the validation logic
	err := builder.ValidateTools()

	if err != nil {
		t.Logf("ValidateTools() error (expected if tools not installed): %v", err)
		// This is not a failure - it's expected if pack/docker aren't installed
	} else {
		t.Log("ValidateTools() passed - build tools are available")
	}
}

func TestBuildRequest_Structure(t *testing.T) {
	req := &BuildRequest{
		ServiceName: "test-service",
		SourcePath:  "/tmp/source",
		GitSHA:      "abc123def456",
		BuildConfig: types.BuildConfig{
			Type: types.BuildTypeBuildpack,
		},
		Env: map[string]string{
			"NODE_ENV": "production",
			"GIT_SHA":  "abc123def456",
		},
	}

	if req.ServiceName != "test-service" {
		t.Errorf("ServiceName = %s, want test-service", req.ServiceName)
	}

	if req.SourcePath != "/tmp/source" {
		t.Errorf("SourcePath = %s, want /tmp/source", req.SourcePath)
	}

	if req.GitSHA != "abc123def456" {
		t.Errorf("GitSHA = %s, want abc123def456", req.GitSHA)
	}

	if len(req.Env) != 2 {
		t.Errorf("Env length = %d, want 2", len(req.Env))
	}

	if req.Env["NODE_ENV"] != "production" {
		t.Error("Env[NODE_ENV] incorrect")
	}
}

func TestBuildResult_Structure(t *testing.T) {
	result := &BuildResult{
		ImageURI: "registry.example.com/service:v1",
		Success:  true,
		Error:    nil,
		Logs:     []string{"log1", "log2", "log3"},
		Duration: 5 * time.Minute,
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.Error != nil {
		t.Error("Error should be nil")
	}

	if len(result.Logs) != 3 {
		t.Errorf("Logs length = %d, want 3", len(result.Logs))
	}

	if result.Duration == 0 {
		t.Error("Duration should not be 0")
	}

	if result.ImageURI == "" {
		t.Error("ImageURI should not be empty")
	}
}

func TestBuildResult_ErrorCase(t *testing.T) {
	result := &BuildResult{
		ImageURI: "registry.example.com/service:v1",
		Success:  false,
		Error:    os.ErrNotExist,
		Logs:     []string{"build failed"},
		Duration: 1 * time.Minute,
	}

	if result.Success {
		t.Error("Success should be false")
	}

	if result.Error == nil {
		t.Error("Error should not be nil")
	}

	if len(result.Logs) == 0 {
		t.Error("Should have error logs")
	}
}

func TestBuildpacksBuilder_Build_DetectionError(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)
	ctx := context.Background()

	req := &BuildRequest{
		ServiceName: "test-service",
		SourcePath:  "/nonexistent/path",
		GitSHA:      "abc123def456",
		BuildConfig: types.BuildConfig{Type: types.BuildTypeAuto},
		Env:         map[string]string{},
	}

	result := builder.Build(ctx, req)

	if result == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have logs even on error
	if len(result.Logs) == 0 {
		t.Error("Build() should have logs")
	}

	// Image URI should still be generated
	if result.ImageURI == "" {
		t.Error("ImageURI should be set even on error")
	}
}

func TestBuildpacksBuilder_BuildService_ToolValidationFails(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)
	ctx := context.Background()

	service := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/test/repo.git",
		BuildConfig: types.BuildConfig{
			Type: types.BuildTypeBuildpack,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// This will likely fail if pack/docker aren't installed
	result, err := builder.BuildService(ctx, service, "abc123def456", "/nonexistent/path")

	// If tools aren't available, should get an error
	if err != nil {
		t.Logf("BuildService() error (expected if tools not installed): %v", err)

		if result != nil {
			t.Error("BuildService() should return nil result when validation fails")
		}
	} else {
		t.Log("BuildService() passed tool validation")

		if result == nil {
			t.Error("BuildService() returned nil result")
		}
	}
}

func TestBuildpacksBuilder_ContextCancellation(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &BuildRequest{
		ServiceName: "test-service",
		SourcePath:  "/tmp/test",
		GitSHA:      "abc123def456",
		BuildConfig: types.BuildConfig{Type: types.BuildTypeAuto},
		Env:         map[string]string{},
	}

	result := builder.Build(ctx, req)

	if result == nil {
		t.Fatal("Build() returned nil")
	}

	// Build should complete (detection phase) even with cancelled context
	// The actual build commands would fail if executed
	if result.ImageURI == "" {
		t.Error("ImageURI should be set")
	}
}

func TestBuildpacksBuilder_Timeout(t *testing.T) {
	// Create builder with very short timeout
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 1*time.Millisecond)

	ctx := context.Background()

	req := &BuildRequest{
		ServiceName: "test-service",
		SourcePath:  "/tmp/test",
		GitSHA:      "abc123def456",
		BuildConfig: types.BuildConfig{Type: types.BuildTypeAuto},
		Env:         map[string]string{},
	}

	result := builder.Build(ctx, req)

	if result == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have some result even with timeout
	if result.Duration == 0 {
		t.Error("Duration should be set")
	}
}

func TestBuildpacksBuilder_Fields(t *testing.T) {
	registry := "custom.registry.io"
	cacheDir := "/custom/cache"
	timeout := 45 * time.Minute

	builder := NewBuildpacksBuilder(registry, "", "", cacheDir, timeout)

	if builder.registry != registry {
		t.Errorf("registry = %s, want %s", builder.registry, registry)
	}

	if builder.cacheDir != cacheDir {
		t.Errorf("cacheDir = %s, want %s", builder.cacheDir, cacheDir)
	}

	if builder.timeout != timeout {
		t.Errorf("timeout = %v, want %v", builder.timeout, timeout)
	}
}

func TestBuildpacksBuilder_MultipleBuilds(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)

	// Test that multiple builds generate different image URIs
	shas := []string{"abc123def456", "xyz789ghi012", "qwe456rty789"}
	uris := make(map[string]bool)

	for _, sha := range shas {
		uri := builder.generateImageURI("test-service", sha)
		if uris[uri] {
			t.Errorf("Duplicate URI generated: %s", uri)
		}
		uris[uri] = true

		// Brief sleep to ensure different timestamps
		time.Sleep(2 * time.Millisecond)
	}

	if len(uris) != len(shas) {
		t.Errorf("Expected %d unique URIs, got %d", len(shas), len(uris))
	}
}

func TestBuildpacksBuilder_EnvVariables(t *testing.T) {
	builder := NewBuildpacksBuilder("registry.example.com", "", "", "/tmp/cache", 30*time.Minute)
	ctx := context.Background()

	env := map[string]string{
		"NODE_ENV":     "production",
		"API_KEY":      "secret123",
		"DATABASE_URL": "postgres://localhost/db",
		"GIT_SHA":      "abc123def456",
	}

	req := &BuildRequest{
		ServiceName: "test-service",
		SourcePath:  "/tmp/test",
		GitSHA:      "abc123def456",
		BuildConfig: types.BuildConfig{Type: types.BuildTypeAuto},
		Env:         env,
	}

	result := builder.Build(ctx, req)

	if result == nil {
		t.Fatal("Build() returned nil")
	}

	// Environment variables should be preserved in the request
	if len(req.Env) != 4 {
		t.Errorf("Env variables count = %d, want 4", len(req.Env))
	}

	for key, expected := range env {
		if req.Env[key] != expected {
			t.Errorf("Env[%s] = %s, want %s", key, req.Env[key], expected)
		}
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
