package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"gopkg.in/yaml.v3"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (p *Parser) ParseServiceSpec(path string) (*types.ServiceSpec, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read service spec file: %w", err)
	}

	// Parse YAML
	var spec types.ServiceSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse service spec YAML: %w", err)
	}

	// Use current working directory as project root for validation
	// This allows spec files to be placed anywhere while paths remain relative to project root
	projectDir, err := os.Getwd()
	if err != nil {
		projectDir = filepath.Dir(path)
	}

	// Validate the spec
	if err := p.ValidateServiceSpec(&spec, projectDir); err != nil {
		return nil, fmt.Errorf("service spec validation failed: %w", err)
	}

	return &spec, nil
}

func (p *Parser) ValidateServiceSpec(spec *types.ServiceSpec, projectDir string) error {
	var errors []ValidationError

	// Validate API version
	if spec.APIVersion == "" {
		errors = append(errors, ValidationError{
			Field:   "apiVersion",
			Message: "is required",
		})
	} else if spec.APIVersion != "enclii.dev/v1alpha" && spec.APIVersion != "enclii.dev/v1" {
		errors = append(errors, ValidationError{
			Field:   "apiVersion",
			Message: "must be 'enclii.dev/v1alpha' or 'enclii.dev/v1'",
		})
	}

	// Validate kind
	if spec.Kind == "" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "is required",
		})
	} else if spec.Kind != "Service" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "must be 'Service'",
		})
	}

	// Validate metadata
	if err := p.validateMetadata(&spec.Metadata); err != nil {
		if ve, ok := err.(ValidationError); ok {
			errors = append(errors, ve)
		} else {
			errors = append(errors, ValidationError{Field: "metadata", Message: err.Error()})
		}
	}

	// Validate spec
	if err := p.validateSpec(&spec.Spec, projectDir); err != nil {
		if ve, ok := err.(ValidationError); ok {
			errors = append(errors, ve)
		} else {
			errors = append(errors, ValidationError{Field: "spec", Message: err.Error()})
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %v", errors)
	}

	return nil
}

func (p *Parser) validateMetadata(metadata *types.ServiceMetadata) error {
	if metadata.Name == "" {
		return ValidationError{Field: "metadata.name", Message: "is required"}
	}

	// Validate name format (DNS-compatible)
	if !isValidDNSName(metadata.Name) {
		return ValidationError{
			Field:   "metadata.name",
			Message: "must be a valid DNS name (lowercase letters, numbers, and hyphens only)",
		}
	}

	if metadata.Project == "" {
		return ValidationError{Field: "metadata.project", Message: "is required"}
	}

	if !isValidDNSName(metadata.Project) {
		return ValidationError{
			Field:   "metadata.project",
			Message: "must be a valid DNS name (lowercase letters, numbers, and hyphens only)",
		}
	}

	return nil
}

func (p *Parser) validateSpec(spec *types.ServiceSpecConfig, projectDir string) error {
	// Validate build configuration
	if err := p.validateBuildSpec(&spec.Build, projectDir); err != nil {
		return err
	}

	// Validate runtime configuration
	if err := p.validateRuntimeSpec(&spec.Runtime); err != nil {
		return err
	}

	// Validate environment variables
	if err := p.validateEnvVars(spec.Env); err != nil {
		return err
	}

	return nil
}

func (p *Parser) validateBuildSpec(build *types.BuildSpec, projectDir string) error {
	validTypes := []string{"auto", "dockerfile", "buildpack"}
	validType := false
	for _, t := range validTypes {
		if build.Type == t {
			validType = true
			break
		}
	}

	if !validType {
		return ValidationError{
			Field:   "spec.build.type",
			Message: fmt.Sprintf("must be one of: %s", strings.Join(validTypes, ", ")),
		}
	}

	// If dockerfile is specified, check if it exists
	if build.Type == "dockerfile" && build.Dockerfile != "" {
		dockerfilePath := filepath.Join(projectDir, build.Dockerfile)
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			return ValidationError{
				Field:   "spec.build.dockerfile",
				Message: fmt.Sprintf("file does not exist: %s", build.Dockerfile),
			}
		}
	}

	// Auto-detect validation
	if build.Type == "auto" {
		if err := p.validateAutoDetection(projectDir); err != nil {
			return ValidationError{
				Field:   "spec.build.type",
				Message: fmt.Sprintf("auto-detection failed: %v", err),
			}
		}
	}

	return nil
}

func (p *Parser) validateRuntimeSpec(runtime *types.RuntimeSpec) error {
	// Validate port
	if runtime.Port <= 0 || runtime.Port > 65535 {
		return ValidationError{
			Field:   "spec.runtime.port",
			Message: "must be between 1 and 65535",
		}
	}

	// Validate replicas
	if runtime.Replicas < 0 {
		return ValidationError{
			Field:   "spec.runtime.replicas",
			Message: "must be greater than or equal to 0",
		}
	}

	if runtime.Replicas > 100 {
		return ValidationError{
			Field:   "spec.runtime.replicas",
			Message: "must be less than or equal to 100 (for safety)",
		}
	}

	// Validate health check path
	if runtime.HealthCheck != "" && !strings.HasPrefix(runtime.HealthCheck, "/") {
		return ValidationError{
			Field:   "spec.runtime.healthCheck",
			Message: "must start with '/' if specified",
		}
	}

	return nil
}

func (p *Parser) validateEnvVars(envVars []types.EnvVar) error {
	seenNames := make(map[string]bool)

	for i, env := range envVars {
		if env.Name == "" {
			return ValidationError{
				Field:   fmt.Sprintf("spec.env[%d].name", i),
				Message: "is required",
			}
		}

		// Check for duplicates
		if seenNames[env.Name] {
			return ValidationError{
				Field:   fmt.Sprintf("spec.env[%d].name", i),
				Message: fmt.Sprintf("duplicate environment variable: %s", env.Name),
			}
		}
		seenNames[env.Name] = true

		// Validate environment variable name format
		if !isValidEnvVarName(env.Name) {
			return ValidationError{
				Field:   fmt.Sprintf("spec.env[%d].name", i),
				Message: "must contain only uppercase letters, numbers, and underscores",
			}
		}
	}

	return nil
}

func (p *Parser) validateAutoDetection(projectDir string) error {
	detectedFiles := []string{}

	checkFiles := map[string]string{
		"package.json":     "Node.js",
		"go.mod":           "Go",
		"requirements.txt": "Python",
		"Gemfile":          "Ruby",
		"pom.xml":          "Java",
		"Dockerfile":       "Docker",
	}

	for file, tech := range checkFiles {
		if _, err := os.Stat(filepath.Join(projectDir, file)); err == nil {
			detectedFiles = append(detectedFiles, tech)
		}
	}

	if len(detectedFiles) == 0 {
		return fmt.Errorf("no supported project files found (package.json, go.mod, requirements.txt, Gemfile, pom.xml, or Dockerfile)")
	}

	return nil
}

func isValidDNSName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}

	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}

	return true
}

func isValidEnvVarName(name string) bool {
	if len(name) == 0 {
		return false
	}

	for i, r := range name {
		if i == 0 {
			// First character must be letter or underscore
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
				return false
			}
		} else {
			// Subsequent characters can be letters, numbers, or underscores
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}

	return true
}

// GenerateServiceSpec creates a new service spec with sensible defaults
func (p *Parser) GenerateServiceSpec(name, project, buildType string) *types.ServiceSpec {
	port := 8080
	if buildType == "node" || buildType == "javascript" || buildType == "typescript" {
		port = 3000
	} else if buildType == "python" {
		port = 8000
	}

	return &types.ServiceSpec{
		APIVersion: "enclii.dev/v1alpha",
		Kind:       "Service",
		Metadata: types.ServiceMetadata{
			Name:    name,
			Project: project,
		},
		Spec: types.ServiceSpecConfig{
			Build: types.BuildSpec{
				Type: buildType,
			},
			Runtime: types.RuntimeSpec{
				Port:        port,
				Replicas:    2,
				HealthCheck: "/health",
			},
			Env: []types.EnvVar{
				{Name: "NODE_ENV", Value: "production"},
				{Name: "PORT", Value: fmt.Sprintf("%d", port)},
			},
		},
	}
}
