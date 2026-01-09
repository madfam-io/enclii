package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/madfam/enclii/packages/cli/internal/config"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

func NewInitCommand(cfg *config.Config) *cobra.Command {
	var templateName string

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new Enclii project",
		Long:  "Create a new service.yaml configuration file and optionally scaffold project structure",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serviceName string
			if len(args) > 0 {
				serviceName = args[0]
			} else {
				// Default to current directory name
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				serviceName = filepath.Base(wd)
			}

			// Get project name (could be different from service name)
			projectName := serviceName // For MVP, project and service names are the same

			return initializeService(serviceName, projectName, templateName)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "auto", "Template to use (auto, node, go, python)")

	return cmd
}

func initializeService(serviceName, projectName, templateName string) error {
	fmt.Printf("🚂 Initializing Enclii service '%s'...\n", serviceName)

	// Check if service.yaml already exists
	serviceYamlPath := "service.yaml"
	if _, err := os.Stat(serviceYamlPath); err == nil {
		return fmt.Errorf("service.yaml already exists in current directory")
	}

	// Create service spec
	spec := &types.ServiceSpec{
		APIVersion: "enclii.dev/v1alpha",
		Kind:       "Service",
		Metadata: types.ServiceMetadata{
			Name:    serviceName,
			Project: projectName,
		},
		Spec: types.ServiceSpecConfig{
			Build: types.BuildSpec{
				Type: templateName,
			},
			Runtime: types.RuntimeSpec{
				Port:        detectPort(templateName),
				Replicas:    2,
				HealthCheck: "/health",
			},
			Env: []types.EnvVar{
				{
					Name:  "NODE_ENV",
					Value: "production",
				},
			},
		},
	}

	// Customize based on template
	switch templateName {
	case "node", "javascript", "typescript":
		spec.Spec.Runtime.Port = 3000
		spec.Spec.Env = []types.EnvVar{
			{Name: "NODE_ENV", Value: "production"},
			{Name: "PORT", Value: "3000"},
		}
	case "go":
		spec.Spec.Runtime.Port = 8080
		spec.Spec.Env = []types.EnvVar{
			{Name: "GO_ENV", Value: "production"},
			{Name: "PORT", Value: "8080"},
		}
	case "python":
		spec.Spec.Runtime.Port = 8000
		spec.Spec.Env = []types.EnvVar{
			{Name: "PYTHONENV", Value: "production"},
			{Name: "PORT", Value: "8000"},
		}
	default:
		// Auto-detect or use defaults
		spec.Spec.Runtime.Port = 8080
	}

	// Write service.yaml
	yamlData, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal service spec: %w", err)
	}

	if err := os.WriteFile(serviceYamlPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write service.yaml: %w", err)
	}

	fmt.Printf("✅ Created %s\n", serviceYamlPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review and customize %s\n", serviceYamlPath)
	fmt.Println("  2. Run 'enclii deploy' to deploy to development")
	fmt.Println("  3. Run 'enclii deploy --env prod' to deploy to production")
	fmt.Println()
	fmt.Printf("💡 Learn more at https://enclii.dev/docs\n")

	return nil
}

func detectPort(template string) int {
	switch strings.ToLower(template) {
	case "node", "javascript", "typescript", "react", "next", "nuxt":
		return 3000
	case "python", "django", "flask", "fastapi":
		return 8000
	case "go", "gin", "echo", "fiber":
		return 8080
	case "ruby", "rails", "sinatra":
		return 3000
	case "java", "spring", "springboot":
		return 8080
	case "php", "laravel", "symfony":
		return 8080
	default:
		return 8080 // Default port
	}
}
