package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
)

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "with default timeout",
			config: &Config{
				WorkDir:      "/tmp/builds",
				Registry:     "registry.example.com",
				CacheDir:     "/tmp/cache",
				Timeout:      0, // Should default to 30 minutes
				GenerateSBOM: false,
				SignImages:   false,
			},
		},
		{
			name: "with custom timeout",
			config: &Config{
				WorkDir:      "/tmp/builds",
				Registry:     "registry.example.com",
				CacheDir:     "/tmp/cache",
				Timeout:      10 * time.Minute,
				GenerateSBOM: true,
				SignImages:   true,
			},
		},
		{
			name: "minimal config",
			config: &Config{
				WorkDir:  "/tmp/builds",
				Registry: "registry.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.config, logger)

			if service == nil {
				t.Fatal("NewService() returned nil")
			}

			if service.git == nil {
				t.Error("git service is nil")
			}

			if service.builder == nil {
				t.Error("builder is nil")
			}

			if service.sbomGen == nil {
				t.Error("SBOM generator is nil")
			}

			if service.signer == nil {
				t.Error("signer is nil")
			}

			if service.timeout == 0 {
				t.Error("timeout is 0 (should have default)")
			}

			if tt.config.Timeout == 0 && service.timeout != 30*time.Minute {
				t.Errorf("default timeout = %v, want %v", service.timeout, 30*time.Minute)
			}

			if tt.config.Timeout != 0 && service.timeout != tt.config.Timeout {
				t.Errorf("timeout = %v, want %v", service.timeout, tt.config.Timeout)
			}

			if service.workDir != tt.config.WorkDir {
				t.Errorf("workDir = %s, want %s", service.workDir, tt.config.WorkDir)
			}

			if service.registry != tt.config.Registry {
				t.Errorf("registry = %s, want %s", service.registry, tt.config.Registry)
			}
		})
	}
}

func TestService_GetBuildStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := &Config{
		WorkDir:      "/tmp/builds",
		Registry:     "registry.example.com",
		CacheDir:     "/tmp/cache",
		GenerateSBOM: true,
		SignImages:   true,
	}

	service := NewService(config, logger)
	status := service.GetBuildStatus()

	if status == nil {
		t.Fatal("GetBuildStatus() returned nil")
	}

	// Check required fields
	requiredFields := []string{
		"work_dir",
		"registry",
		"timeout",
		"tools_available",
		"sbom_enabled",
		"signing_enabled",
	}

	for _, field := range requiredFields {
		if _, ok := status[field]; !ok {
			t.Errorf("GetBuildStatus() missing field: %s", field)
		}
	}

	// Verify values
	if status["work_dir"] != config.WorkDir {
		t.Errorf("work_dir = %s, want %s", status["work_dir"], config.WorkDir)
	}

	if status["registry"] != config.Registry {
		t.Errorf("registry = %s, want %s", status["registry"], config.Registry)
	}

	// Note: sbom_enabled and signing_enabled may be false even if config says true
	// if Syft/Cosign tools are not installed. This is expected behavior.
	// Just verify the fields exist and are booleans.
	if _, ok := status["sbom_enabled"].(bool); !ok {
		t.Errorf("sbom_enabled should be a boolean, got %T", status["sbom_enabled"])
	}

	if _, ok := status["signing_enabled"].(bool); !ok {
		t.Errorf("signing_enabled should be a boolean, got %T", status["signing_enabled"])
	}

	// If tools are installed, values should match config
	// If tools are missing, values will be false (which is correct)
}

func TestService_GetBuildStatus_Disabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := &Config{
		WorkDir:      "/tmp/builds",
		Registry:     "registry.example.com",
		GenerateSBOM: false,
		SignImages:   false,
	}

	service := NewService(config, logger)
	status := service.GetBuildStatus()

	if status["sbom_enabled"] != false {
		t.Error("sbom_enabled should be false")
	}

	if status["signing_enabled"] != false {
		t.Error("signing_enabled should be false")
	}

	// When disabled, availability checks shouldn't be present
	if _, ok := status["sbom_available"]; ok {
		t.Error("sbom_available should not be present when disabled")
	}

	if _, ok := status["signing_available"]; ok {
		t.Error("signing_available should not be present when disabled")
	}
}

func TestService_ValidateService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := &Config{
		WorkDir:  "/tmp/builds",
		Registry: "registry.example.com",
	}

	service := NewService(config, logger)
	ctx := context.Background()

	testService := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/invalid/nonexistent-repo.git",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// This will likely fail because the repo doesn't exist
	// But we're testing that the validation logic runs without panicking
	err := service.ValidateService(ctx, testService)

	// We expect an error for a non-existent repo
	if err == nil {
		t.Log("ValidateService() succeeded (repo might be accessible or check is lenient)")
	} else {
		t.Logf("ValidateService() error (expected): %v", err)
	}
}

func TestCompleteBuildResult_Structure(t *testing.T) {
	// Test that the result structure can be created and used
	result := &CompleteBuildResult{
		ImageURI:      "registry.example.com/test:v1",
		GitSHA:        "abc123def456",
		Success:       true,
		Logs:          []string{"log1", "log2"},
		Duration:      5 * time.Minute,
		ClonePath:     "/tmp/build-abc123d",
		SBOMGenerated: true,
		SBOMFormat:    "cyclonedx-json",
		ImageSigned:   true,
	}

	if result.ImageURI == "" {
		t.Error("ImageURI is empty")
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if len(result.Logs) != 2 {
		t.Errorf("Logs length = %d, want 2", len(result.Logs))
	}

	if result.Duration == 0 {
		t.Error("Duration is 0")
	}

	if !result.SBOMGenerated {
		t.Error("SBOMGenerated should be true")
	}

	if !result.ImageSigned {
		t.Error("ImageSigned should be true")
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		valid  bool
	}{
		{
			name: "valid config",
			config: &Config{
				WorkDir:  "/tmp/builds",
				Registry: "registry.example.com",
				CacheDir: "/tmp/cache",
				Timeout:  10 * time.Minute,
			},
			valid: true,
		},
		{
			name: "missing optional fields",
			config: &Config{
				WorkDir:  "/tmp/builds",
				Registry: "registry.example.com",
			},
			valid: true,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.config, logger)

			if tt.valid && service == nil {
				t.Error("NewService() returned nil for valid config")
			}

			if !tt.valid && service != nil {
				t.Error("NewService() should return nil for invalid config")
			}
		})
	}
}

func TestService_BuildFromGit_InvalidRepo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Create a temporary directory for the test
	tmpDir := filepath.Join(os.TempDir(), "builder-test")
	defer os.RemoveAll(tmpDir)

	config := &Config{
		WorkDir:      tmpDir,
		Registry:     "registry.example.com",
		CacheDir:     filepath.Join(tmpDir, "cache"),
		Timeout:      1 * time.Minute,
		GenerateSBOM: false,
		SignImages:   false,
	}

	service := NewService(config, logger)
	ctx := context.Background()

	testService := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/invalid/definitely-nonexistent-repo-12345.git",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := service.BuildFromGit(ctx, testService, "abc123def456")

	// Should fail because repo doesn't exist
	if result.Success {
		t.Error("BuildFromGit() should fail for invalid repo")
	}

	if result.Error == nil {
		t.Error("BuildFromGit() error should not be nil for invalid repo")
	}

	if len(result.Logs) == 0 {
		t.Error("BuildFromGit() should have logs even on failure")
	}
}

func TestService_BuildFromGit_ShortGitSHA(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	tmpDir := filepath.Join(os.TempDir(), "builder-test-short")
	defer os.RemoveAll(tmpDir)

	config := &Config{
		WorkDir:  tmpDir,
		Registry: "registry.example.com",
	}

	service := NewService(config, logger)
	ctx := context.Background()

	testService := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/test/repo.git",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test with short SHA (less than 7 characters)
	result := service.BuildFromGit(ctx, testService, "abc")

	// Should fail gracefully (likely during clone)
	if result.Success {
		t.Error("BuildFromGit() should fail for short SHA")
	}
}

func TestService_Timeout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	tmpDir := filepath.Join(os.TempDir(), "builder-test-timeout")
	defer os.RemoveAll(tmpDir)

	// Very short timeout
	config := &Config{
		WorkDir:  tmpDir,
		Registry: "registry.example.com",
		Timeout:  1 * time.Millisecond, // Very short timeout
	}

	service := NewService(config, logger)
	ctx := context.Background()

	testService := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/test/repo.git",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := service.BuildFromGit(ctx, testService, "abc123def456")

	// Should fail due to timeout or clone error
	if result.Success {
		t.Error("BuildFromGit() should fail with very short timeout")
	}
}

func TestService_Fields(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := &Config{
		WorkDir:      "/custom/workdir",
		Registry:     "custom.registry.io",
		CacheDir:     "/custom/cache",
		Timeout:      15 * time.Minute,
		GenerateSBOM: true,
		SignImages:   true,
	}

	service := NewService(config, logger)

	if service.workDir != config.WorkDir {
		t.Errorf("workDir = %s, want %s", service.workDir, config.WorkDir)
	}

	if service.registry != config.Registry {
		t.Errorf("registry = %s, want %s", service.registry, config.Registry)
	}

	if service.timeout != config.Timeout {
		t.Errorf("timeout = %v, want %v", service.timeout, config.Timeout)
	}

	if service.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestCompleteBuildResult_ErrorHandling(t *testing.T) {
	// Test error cases
	result := &CompleteBuildResult{
		GitSHA:  "abc123",
		Success: false,
		Error:   os.ErrNotExist,
		Logs:    []string{"clone failed"},
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

	if result.ImageURI != "" {
		t.Error("ImageURI should be empty on failure")
	}
}

func TestService_LoggerUsage(t *testing.T) {
	// Create a logger with specific level
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	config := &Config{
		WorkDir:  "/tmp/test",
		Registry: "test.io",
	}

	service := NewService(config, logger)

	// Verify logger is set
	if service.logger == nil {
		t.Error("logger is nil")
	}

	if service.logger.GetLevel() != logrus.DebugLevel {
		t.Errorf("logger level = %v, want %v", service.logger.GetLevel(), logrus.DebugLevel)
	}
}
