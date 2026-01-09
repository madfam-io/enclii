package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"gopkg.in/yaml.v3"
)

func TestParseServiceSpec(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "enclii-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create valid service.yaml
	validSpec := &types.ServiceSpec{
		APIVersion: "enclii.dev/v1alpha",
		Kind:       "Service",
		Metadata: types.ServiceMetadata{
			Name:    "test-service",
			Project: "test-project",
		},
		Spec: types.ServiceSpecConfig{
			Build: types.BuildSpec{
				Type: "auto",
			},
			Runtime: types.RuntimeSpec{
				Port:        8080,
				Replicas:    2,
				HealthCheck: "/health",
			},
			Env: []types.EnvVar{
				{Name: "NODE_ENV", Value: "production"},
			},
		},
	}

	data, err := yaml.Marshal(validSpec)
	if err != nil {
		t.Fatal(err)
	}

	serviceYamlPath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(serviceYamlPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Create package.json to satisfy auto-detection
	packageJsonPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packageJsonPath, []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to tmpDir so auto-detection works (ParseServiceSpec uses os.Getwd())
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Test parsing
	parser := NewParser()
	parsedSpec, err := parser.ParseServiceSpec(serviceYamlPath)
	if err != nil {
		t.Fatalf("Failed to parse valid service spec: %v", err)
	}

	// Verify parsed values
	if parsedSpec.Metadata.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", parsedSpec.Metadata.Name)
	}

	if parsedSpec.Spec.Runtime.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", parsedSpec.Spec.Runtime.Port)
	}
}

func TestValidateServiceSpec(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		spec    *types.ServiceSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &types.ServiceSpec{
				APIVersion: "enclii.dev/v1alpha",
				Kind:       "Service",
				Metadata: types.ServiceMetadata{
					Name:    "valid-service",
					Project: "valid-project",
				},
				Spec: types.ServiceSpecConfig{
					Build: types.BuildSpec{Type: "auto"},
					Runtime: types.RuntimeSpec{
						Port:        8080,
						Replicas:    2,
						HealthCheck: "/health",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing API version",
			spec: &types.ServiceSpec{
				Kind: "Service",
				Metadata: types.ServiceMetadata{
					Name:    "test-service",
					Project: "test-project",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			spec: &types.ServiceSpec{
				APIVersion: "enclii.dev/v1alpha",
				Kind:       "Service",
				Metadata: types.ServiceMetadata{
					Name:    "test-service",
					Project: "test-project",
				},
				Spec: types.ServiceSpecConfig{
					Build:   types.BuildSpec{Type: "auto"},
					Runtime: types.RuntimeSpec{Port: 0, Replicas: 1},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid service name",
			spec: &types.ServiceSpec{
				APIVersion: "enclii.dev/v1alpha",
				Kind:       "Service",
				Metadata: types.ServiceMetadata{
					Name:    "Test_Service", // Invalid DNS name
					Project: "test-project",
				},
				Spec: types.ServiceSpecConfig{
					Build:   types.BuildSpec{Type: "auto"},
					Runtime: types.RuntimeSpec{Port: 8080, Replicas: 1},
				},
			},
			wantErr: true,
		},
	}

	// Create temp directory for validation tests
	tmpDir, err := os.MkdirTemp("", "enclii-validation-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json for auto-detection
	packageJsonPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packageJsonPath, []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateServiceSpec(tt.spec, tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServiceSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidDNSName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid lowercase", "test-service", true},
		{"valid with numbers", "service123", true},
		{"invalid uppercase", "Test-Service", false},
		{"invalid underscore", "test_service", false},
		{"invalid start with dash", "-test", false},
		{"invalid end with dash", "test-", false},
		{"empty string", "", false},
		{"too long", "a-very-long-service-name-that-exceeds-the-maximum-length-of-63-characters", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidDNSName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidDNSName(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidEnvVarName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid uppercase", "NODE_ENV", true},
		{"valid with numbers", "PORT_8080", true},
		{"valid lowercase", "database_url", true},
		{"invalid start with number", "8080_PORT", false},
		{"invalid special chars", "NODE-ENV", false},
		{"invalid empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEnvVarName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidEnvVarName(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateServiceSpec(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name      string
		buildType string
		wantPort  int
	}{
		{"node service", "node", 3000},
		{"python service", "python", 8000},
		{"go service", "go", 8080},
		{"default service", "unknown", 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := parser.GenerateServiceSpec("test-service", "test-project", tt.buildType)

			if spec.APIVersion != "enclii.dev/v1alpha" {
				t.Errorf("Expected API version 'enclii.dev/v1alpha', got '%s'", spec.APIVersion)
			}

			if spec.Spec.Runtime.Port != tt.wantPort {
				t.Errorf("Expected port %d, got %d", tt.wantPort, spec.Spec.Runtime.Port)
			}

			if spec.Spec.Build.Type != tt.buildType {
				t.Errorf("Expected build type '%s', got '%s'", tt.buildType, spec.Spec.Build.Type)
			}
		})
	}
}
