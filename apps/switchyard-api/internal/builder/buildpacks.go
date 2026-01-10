package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
)

type BuildpacksBuilder struct {
	registry         string
	registryUsername string
	registryPassword string
	cacheDir         string
	timeout          time.Duration
}

func NewBuildpacksBuilder(registry, registryUsername, registryPassword, cacheDir string, timeout time.Duration) *BuildpacksBuilder {
	return &BuildpacksBuilder{
		registry:         registry,
		registryUsername: registryUsername,
		registryPassword: registryPassword,
		cacheDir:         cacheDir,
		timeout:          timeout,
	}
}

type BuildRequest struct {
	ServiceName string
	SourcePath  string
	GitSHA      string
	BuildConfig types.BuildConfig
	Env         map[string]string
}

type BuildResult struct {
	ImageURI string
	Success  bool
	Error    error
	Logs     []string
	Duration time.Duration
}

func (b *BuildpacksBuilder) Build(ctx context.Context, req *BuildRequest) *BuildResult {
	start := time.Now()
	result := &BuildResult{
		ImageURI: b.generateImageURI(req.ServiceName, req.GitSHA),
		Logs:     []string{},
	}

	logrus.Infof("Starting build for service %s (SHA: %s)", req.ServiceName, req.GitSHA[:7])

	// Detect build strategy
	buildStrategy, err := b.detectBuildStrategy(req.SourcePath, req.BuildConfig)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect build strategy: %w", err)
		return result
	}

	result.Logs = append(result.Logs, fmt.Sprintf("Detected build strategy: %s", buildStrategy))

	// Execute build
	switch buildStrategy {
	case "buildpacks":
		err = b.buildWithBuildpacks(ctx, req, result)
	case "dockerfile":
		err = b.buildWithDockerfile(ctx, req, result)
	default:
		err = fmt.Errorf("unsupported build strategy: %s", buildStrategy)
	}

	result.Duration = time.Since(start)
	result.Success = err == nil
	result.Error = err

	if result.Success {
		logrus.Infof("Build completed successfully in %v: %s", result.Duration, result.ImageURI)
	} else {
		logrus.Errorf("Build failed after %v: %v", result.Duration, err)
	}

	return result
}

func (b *BuildpacksBuilder) detectBuildStrategy(sourcePath string, config types.BuildConfig) (string, error) {
	// If explicitly configured, use that
	if config.Type != "" && config.Type != types.BuildTypeAuto {
		return string(config.Type), nil
	}

	// Auto-detect based on files present
	files := []string{
		"Dockerfile",
		"package.json",
		"go.mod",
		"requirements.txt",
		"Gemfile",
		"pom.xml",
	}

	for _, file := range files {
		if _, err := os.Stat(filepath.Join(sourcePath, file)); err == nil {
			switch file {
			case "Dockerfile":
				return "dockerfile", nil
			case "package.json":
				return "buildpacks", nil
			case "go.mod":
				return "buildpacks", nil
			case "requirements.txt":
				return "buildpacks", nil
			case "Gemfile":
				return "buildpacks", nil
			case "pom.xml":
				return "buildpacks", nil
			}
		}
	}

	// Default to buildpacks
	return "buildpacks", nil
}

func (b *BuildpacksBuilder) buildWithBuildpacks(ctx context.Context, req *BuildRequest, result *BuildResult) error {
	result.Logs = append(result.Logs, "Building with Cloud Native Buildpacks...")

	// Create build command
	// Note: pack CLI uses Docker volumes for caching by default
	// The --cache-dir flag doesn't exist; use --cache-image for remote caching
	args := []string{
		"build", result.ImageURI,
		"--path", req.SourcePath,
		"--builder", "paketocommunity/builder-ubi-base:latest",
		"--publish",
	}

	// Add environment variables
	for key, value := range req.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.CommandContext(ctx, "pack", args...)
	cmd.Dir = req.SourcePath

	// Capture output
	output, err := cmd.CombinedOutput()
	result.Logs = append(result.Logs, string(output))

	if err != nil {
		return fmt.Errorf("pack build failed: %w", err)
	}

	return nil
}

func (b *BuildpacksBuilder) buildWithDockerfile(ctx context.Context, req *BuildRequest, result *BuildResult) error {
	result.Logs = append(result.Logs, "Building with Dockerfile...")

	// Find Dockerfile
	dockerfilePath := "Dockerfile"
	if req.BuildConfig.Dockerfile != "" {
		dockerfilePath = req.BuildConfig.Dockerfile
	}

	// Create build command
	args := []string{
		"build",
		"-t", result.ImageURI,
		"-f", dockerfilePath,
		".",
	}

	// Add build args
	for key, value := range req.Env {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = req.SourcePath

	// Capture output
	output, err := cmd.CombinedOutput()
	result.Logs = append(result.Logs, string(output))

	if err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	// Push to registry
	pushCmd := exec.CommandContext(ctx, "docker", "push", result.ImageURI)
	pushOutput, err := pushCmd.CombinedOutput()
	result.Logs = append(result.Logs, string(pushOutput))

	if err != nil {
		return fmt.Errorf("docker push failed: %w", err)
	}

	return nil
}

func (b *BuildpacksBuilder) generateImageURI(serviceName, gitSHA string) string {
	timestamp := time.Now().Format("20060102-150405")
	tag := fmt.Sprintf("v%s-%s", timestamp, gitSHA[:7])
	return fmt.Sprintf("%s/%s:%s", b.registry, serviceName, tag)
}

func (b *BuildpacksBuilder) ValidateTools() error {
	tools := []string{"pack", "docker"}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s not found in PATH: %w", tool, err)
		}
	}

	// Check Docker is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running: %w", err)
	}

	return nil
}

// DockerLogin authenticates with the container registry
func (b *BuildpacksBuilder) DockerLogin(ctx context.Context) error {
	if b.registryUsername == "" || b.registryPassword == "" {
		logrus.Warn("Registry credentials not configured, skipping docker login")
		return nil
	}

	// Extract registry hostname (e.g., ghcr.io from ghcr.io/madfam-org)
	registryHost := b.registry
	if idx := len(b.registry); idx > 0 {
		// Find first slash after protocol
		for i, c := range b.registry {
			if c == '/' {
				registryHost = b.registry[:i]
				break
			}
		}
	}

	logrus.Infof("Logging in to container registry: %s", registryHost)

	// Use docker login with password from stdin for security
	cmd := exec.CommandContext(ctx, "docker", "login", registryHost, "-u", b.registryUsername, "--password-stdin")
	cmd.Stdin = strings.NewReader(b.registryPassword)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login failed: %w, output: %s", err, string(output))
	}

	logrus.Info("Successfully authenticated with container registry")
	return nil
}

// BuildService is a high-level function that orchestrates the build process
func (b *BuildpacksBuilder) BuildService(ctx context.Context, service *types.Service, gitSHA string, sourcePath string) (*BuildResult, error) {
	// Validate tools are available
	if err := b.ValidateTools(); err != nil {
		return nil, fmt.Errorf("build tools validation failed: %w", err)
	}

	// Authenticate with container registry before build
	if err := b.DockerLogin(ctx); err != nil {
		return nil, fmt.Errorf("registry authentication failed: %w", err)
	}

	// Create build request
	req := &BuildRequest{
		ServiceName: service.Name,
		SourcePath:  sourcePath,
		GitSHA:      gitSHA,
		BuildConfig: service.BuildConfig,
		Env: map[string]string{
			"GIT_SHA": gitSHA,
		},
	}

	// Execute build
	result := b.Build(ctx, req)

	return result, result.Error
}
