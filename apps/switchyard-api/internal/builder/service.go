package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Service orchestrates the complete build process: clone → build → cleanup
type Service struct {
	git      *GitService
	builder  *BuildpacksBuilder
	logger   *logrus.Logger
	workDir  string
	registry string
	timeout  time.Duration
}

type Config struct {
	WorkDir  string
	Registry string
	CacheDir string
	Timeout  time.Duration
}

func NewService(cfg *Config, logger *logrus.Logger) *Service {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}

	return &Service{
		git:      NewGitService(cfg.WorkDir),
		builder:  NewBuildpacksBuilder(cfg.Registry, cfg.CacheDir, cfg.Timeout),
		logger:   logger,
		workDir:  cfg.WorkDir,
		registry: cfg.Registry,
		timeout:  cfg.Timeout,
	}
}

type CompleteBuildResult struct {
	ImageURI  string
	GitSHA    string
	Success   bool
	Error     error
	Logs      []string
	Duration  time.Duration
	ClonePath string
}

// BuildFromGit clones a repository and builds it
func (s *Service) BuildFromGit(ctx context.Context, service *types.Service, gitSHA string) *CompleteBuildResult {
	start := time.Now()
	result := &CompleteBuildResult{
		GitSHA: gitSHA,
		Logs:   []string{},
	}

	// Create a timeout context
	buildCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Step 1: Clone the repository
	result.Logs = append(result.Logs, fmt.Sprintf("Cloning repository: %s", service.GitRepo))
	cloneResult := s.git.CloneRepository(buildCtx, service.GitRepo, gitSHA)

	if !cloneResult.Success {
		result.Error = fmt.Errorf("clone failed: %w", cloneResult.Error)
		result.Logs = append(result.Logs, fmt.Sprintf("ERROR: %v", cloneResult.Error))
		return result
	}

	result.ClonePath = cloneResult.Path
	result.Logs = append(result.Logs, fmt.Sprintf("Successfully cloned to: %s", cloneResult.Path))

	// Ensure cleanup happens
	if cloneResult.CleanupFn != nil {
		defer func() {
			if cleanupErr := cloneResult.CleanupFn(); cleanupErr != nil {
				s.logger.Errorf("Failed to cleanup clone directory: %v", cleanupErr)
			}
		}()
	}

	// Step 2: Build the service
	result.Logs = append(result.Logs, fmt.Sprintf("Starting build for service: %s", service.Name))
	buildResult, err := s.builder.BuildService(buildCtx, service, gitSHA, cloneResult.Path)

	if err != nil {
		result.Error = fmt.Errorf("build failed: %w", err)
		result.Logs = append(result.Logs, buildResult.Logs...)
		result.Logs = append(result.Logs, fmt.Sprintf("ERROR: %v", err))
		return result
	}

	// Success!
	result.ImageURI = buildResult.ImageURI
	result.Success = true
	result.Logs = append(result.Logs, buildResult.Logs...)
	result.Duration = time.Since(start)

	result.Logs = append(result.Logs, fmt.Sprintf("Build completed successfully in %v", result.Duration))
	result.Logs = append(result.Logs, fmt.Sprintf("Image: %s", result.ImageURI))

	return result
}

// ValidateService checks if a service can be built
func (s *Service) ValidateService(ctx context.Context, service *types.Service) error {
	// Validate git repository
	if err := s.git.ValidateRepository(ctx, service.GitRepo); err != nil {
		return fmt.Errorf("invalid git repository: %w", err)
	}

	// Validate build tools
	if err := s.builder.ValidateTools(); err != nil {
		return fmt.Errorf("build tools not available: %w", err)
	}

	return nil
}

// GetBuildStatus returns information about the build environment
func (s *Service) GetBuildStatus() map[string]interface{} {
	status := map[string]interface{}{
		"work_dir": s.workDir,
		"registry": s.registry,
		"timeout":  s.timeout.String(),
	}

	// Check if tools are available
	if err := s.builder.ValidateTools(); err != nil {
		status["tools_available"] = false
		status["tools_error"] = err.Error()
	} else {
		status["tools_available"] = true
	}

	return status
}
