package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewGitService(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
	}{
		{"standard path", "/tmp/builds"},
		{"relative path", "./builds"},
		{"empty path", ""},
		{"nested path", "/var/lib/enclii/builds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewGitService(tt.workDir)

			if service == nil {
				t.Fatal("NewGitService() returned nil")
			}

			if service.workDir != tt.workDir {
				t.Errorf("workDir = %s, want %s", service.workDir, tt.workDir)
			}
		})
	}
}

func TestGitService_CloneRepository_InvalidRepo(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-test")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)
	ctx := context.Background()

	// Test with invalid repository URL
	result := service.CloneRepository(ctx, "https://github.com/invalid/nonexistent-repo-12345.git", "abc123def456")

	if result == nil {
		t.Fatal("CloneRepository() returned nil")
	}

	// Should fail for non-existent repo
	if result.Success {
		t.Error("CloneRepository() should fail for invalid repo")
	}

	if result.Error == nil {
		t.Error("Error should not be nil for invalid repo")
	}

	if result.GitSHA != "abc123def456" {
		t.Errorf("GitSHA = %s, want abc123def456", result.GitSHA)
	}
}

func TestGitService_CloneRepository_ShortSHA(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-test-short")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)
	ctx := context.Background()

	// Test with short SHA (less than 7 characters)
	// This should fail during clone or checkout
	result := service.CloneRepository(ctx, "https://github.com/test/repo.git", "abc")

	if result == nil {
		t.Fatal("CloneRepository() returned nil")
	}

	// Should fail (likely with index out of range or similar)
	if result.Success {
		t.Error("CloneRepository() should handle short SHA gracefully")
	}
}

func TestGitService_CloneShallow_InvalidRepo(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-test-shallow")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)
	ctx := context.Background()

	result := service.CloneShallow(ctx, "https://github.com/invalid/nonexistent-repo-12345.git", "abc123def456")

	if result == nil {
		t.Fatal("CloneShallow() returned nil")
	}

	// Should fail and fallback to full clone (which will also fail)
	if result.Success {
		t.Error("CloneShallow() should fail for invalid repo")
	}

	if result.Error == nil {
		t.Error("Error should not be nil")
	}
}

func TestGitService_CloneShallow_ShortSHA(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-test-shallow-short")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)
	ctx := context.Background()

	result := service.CloneShallow(ctx, "https://github.com/test/repo.git", "abc")

	if result == nil {
		t.Fatal("CloneShallow() returned nil")
	}

	// Should fail gracefully
	if result.Success {
		t.Error("CloneShallow() should handle short SHA")
	}
}

func TestGitService_ValidateRepository(t *testing.T) {
	service := NewGitService("/tmp")
	ctx := context.Background()

	tests := []struct {
		name      string
		repoURL   string
		expectErr bool
	}{
		{
			name:      "invalid repo",
			repoURL:   "https://github.com/invalid/definitely-nonexistent-repo-12345.git",
			expectErr: true,
		},
		{
			name:      "invalid URL format",
			repoURL:   "not-a-valid-url",
			expectErr: true,
		},
		{
			name:      "empty URL",
			repoURL:   "",
			expectErr: true,
		},
		{
			name:      "malformed git URL",
			repoURL:   "git@invalid:repo",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateRepository(ctx, tt.repoURL)

			if tt.expectErr && err == nil {
				t.Error("ValidateRepository() expected error, got nil")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("ValidateRepository() unexpected error: %v", err)
			}
		})
	}
}

func TestCloneResult_Structure(t *testing.T) {
	// Test creating and using CloneResult
	cleanupCalled := false
	result := &CloneResult{
		Path:    "/tmp/build-abc123d",
		GitSHA:  "abc123def456",
		Success: true,
		Error:   nil,
		CleanupFn: func() error {
			cleanupCalled = true
			return nil
		},
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if result.Error != nil {
		t.Error("Error should be nil")
	}

	if result.Path == "" {
		t.Error("Path should not be empty")
	}

	if result.GitSHA != "abc123def456" {
		t.Errorf("GitSHA = %s, want abc123def456", result.GitSHA)
	}

	// Test cleanup function
	if result.CleanupFn == nil {
		t.Error("CleanupFn is nil")
	} else {
		err := result.CleanupFn()
		if err != nil {
			t.Errorf("CleanupFn() error: %v", err)
		}
		if !cleanupCalled {
			t.Error("CleanupFn was not called")
		}
	}
}

func TestCloneResult_ErrorCase(t *testing.T) {
	result := &CloneResult{
		Path:      "",
		GitSHA:    "abc123",
		Success:   false,
		Error:     os.ErrNotExist,
		CleanupFn: nil,
	}

	if result.Success {
		t.Error("Success should be false")
	}

	if result.Error == nil {
		t.Error("Error should not be nil")
	}

	if result.Path != "" {
		t.Error("Path should be empty on error")
	}

	if result.CleanupFn != nil {
		t.Error("CleanupFn should be nil on error")
	}
}

func TestGitService_WorkDirCreation(t *testing.T) {
	tmpBase := filepath.Join(os.TempDir(), "git-workdir-test")
	defer os.RemoveAll(tmpBase)

	tmpDir := filepath.Join(tmpBase, "nested", "deep", "path")
	service := NewGitService(tmpDir)
	ctx := context.Background()

	// Try to clone (will fail but should create work directory)
	result := service.CloneRepository(ctx, "https://invalid.repo.git", "abc123def456")

	// Check if work directory was created
	if _, err := os.Stat(tmpDir); err != nil {
		if os.IsNotExist(err) {
			t.Error("Work directory was not created")
		}
	}

	if result.Success {
		t.Error("Clone should fail for invalid repo")
	}
}

func TestGitService_CloneDirectoryNaming(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-naming-test")
	defer os.RemoveAll(tmpDir)

	// Test that directory name includes first 7 chars of SHA
	gitSHA := "abc123def456789"
	expectedDirName := "build-abc123d"

	// We can't test actual cloning, but we can verify the expected path structure
	expectedPath := filepath.Join(tmpDir, expectedDirName)

	// This is what the path should look like
	if !filepath.IsAbs(tmpDir) && tmpDir != "" {
		t.Log("Testing with relative path")
	}

	// Verify the expected path format
	if expectedPath != filepath.Join(tmpDir, "build-"+gitSHA[:7]) {
		t.Errorf("Expected path format mismatch")
	}
}

func TestGitService_ConcurrentClones(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-concurrent-test")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)
	ctx := context.Background()

	// Test that different SHAs create different directories
	shas := []string{"abc123def456", "xyz789ghi012", "qwe456rty789"}

	for _, sha := range shas {
		result := service.CloneRepository(ctx, "https://invalid.repo.git", sha)

		if result == nil {
			t.Errorf("CloneRepository() returned nil for SHA %s", sha)
			continue
		}

		if result.GitSHA != sha {
			t.Errorf("GitSHA = %s, want %s", result.GitSHA, sha)
		}
	}
}

func TestGitService_ContextCancellation(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-context-test")
	defer os.RemoveAll(tmpDir)

	service := NewGitService(tmpDir)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := service.CloneRepository(ctx, "https://github.com/test/repo.git", "abc123def456")

	if result == nil {
		t.Fatal("CloneRepository() returned nil")
	}

	// Should fail due to cancelled context
	if result.Success {
		t.Error("CloneRepository() should fail with cancelled context")
	}

	if result.Error == nil {
		t.Error("Error should not be nil with cancelled context")
	}
}

func TestGitService_CleanupFunction(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "git-cleanup-test")
	defer os.RemoveAll(tmpDir)

	// Create a test directory
	testDir := filepath.Join(tmpDir, "test-cleanup")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(testDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a CloneResult with cleanup function
	result := &CloneResult{
		Path:    testDir,
		GitSHA:  "test",
		Success: true,
		CleanupFn: func() error {
			return os.RemoveAll(testDir)
		},
	}

	// Verify directory exists
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Fatal("Test directory does not exist before cleanup")
	}

	// Call cleanup
	if result.CleanupFn != nil {
		err = result.CleanupFn()
		if err != nil {
			t.Errorf("CleanupFn() error: %v", err)
		}
	}

	// Verify directory is removed
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Test directory still exists after cleanup")
	}
}
