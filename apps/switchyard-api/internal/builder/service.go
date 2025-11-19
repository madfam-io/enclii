package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/madfam/enclii/apps/switchyard-api/internal/sbom"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Service orchestrates the complete build process: clone → build → SBOM → cleanup
type Service struct {
	git          *GitService
	builder      *BuildpacksBuilder
	sbomGen      *sbom.Generator
	logger       *logrus.Logger
	workDir      string
	registry     string
	timeout      time.Duration
	generateSBOM bool // Enable/disable SBOM generation
}

type Config struct {
	WorkDir      string
	Registry     string
	CacheDir     string
	Timeout      time.Duration
	GenerateSBOM bool // Enable SBOM generation (requires Syft)
}

func NewService(cfg *Config, logger *logrus.Logger) *Service {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}

	sbomGenerator := sbom.NewGenerator(5 * time.Minute)

	// Check if Syft is available (non-fatal if missing)
	generateSBOM := cfg.GenerateSBOM
	if generateSBOM {
		if err := sbomGenerator.ValidateSyftInstalled(); err != nil {
			logger.Warn("SBOM generation disabled: Syft not installed")
			logger.Warnf("  %v", err)
			generateSBOM = false
		} else {
			logger.Info("✓ SBOM generation enabled (Syft installed)")
		}
	}

	return &Service{
		git:          NewGitService(cfg.WorkDir),
		builder:      NewBuildpacksBuilder(cfg.Registry, cfg.CacheDir, cfg.Timeout),
		sbomGen:      sbomGenerator,
		logger:       logger,
		workDir:      cfg.WorkDir,
		registry:     cfg.Registry,
		timeout:      cfg.Timeout,
		generateSBOM: generateSBOM,
	}
}

type CompleteBuildResult struct {
	ImageURI     string
	GitSHA       string
	Success      bool
	Error        error
	Logs         []string
	Duration     time.Duration
	ClonePath    string
	SBOM         *sbom.SBOM // Software Bill of Materials
	SBOMFormat   string     // e.g., "cyclonedx-json"
	SBOMGenerated bool      // Whether SBOM was successfully generated
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

	// Step 3: Generate SBOM (if enabled)
	if s.generateSBOM {
		result.Logs = append(result.Logs, "Generating SBOM with Syft...")
		sbomResult, err := s.sbomGen.GenerateFromImage(buildCtx, result.ImageURI, sbom.GetDefaultFormat())

		if err != nil {
			// SBOM generation failure is non-fatal - log warning and continue
			s.logger.Warnf("SBOM generation failed (non-fatal): %v", err)
			result.Logs = append(result.Logs, fmt.Sprintf("WARNING: SBOM generation failed: %v", err))
			result.SBOMGenerated = false
		} else {
			result.SBOM = sbomResult
			result.SBOMFormat = string(sbomResult.Format)
			result.SBOMGenerated = true
			result.Logs = append(result.Logs, fmt.Sprintf("✓ SBOM generated (%d packages found)", sbomResult.PackageCount))
		}
	}

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

	// Check if SBOM generation is enabled and available
	status["sbom_enabled"] = s.generateSBOM
	if s.generateSBOM {
		if err := s.sbomGen.ValidateSyftInstalled(); err != nil {
			status["sbom_available"] = false
			status["sbom_error"] = err.Error()
		} else {
			status["sbom_available"] = true
		}
	}

	return status
}
