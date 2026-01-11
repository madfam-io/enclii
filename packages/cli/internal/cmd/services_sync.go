package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/madfam-org/enclii/packages/cli/internal/client"
	"github.com/madfam-org/enclii/packages/cli/internal/config"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// RawServiceSpec is used to parse the full YAML structure including source.git fields
type RawServiceSpec struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name    string `yaml:"name"`
		Project string `yaml:"project"`
	} `yaml:"metadata"`
	Spec struct {
		Build struct {
			Type   string `yaml:"type"`
			Source struct {
				Git struct {
					Repository string `yaml:"repository"`
					Branch     string `yaml:"branch"`
					Path       string `yaml:"path"`
					AutoDeploy bool   `yaml:"autoDeploy"`
				} `yaml:"git"`
			} `yaml:"source"`
		} `yaml:"build"`
	} `yaml:"spec"`
}

func NewServicesSyncCommand(cfg *config.Config) *cobra.Command {
	var dir string
	var dryRun bool
	var projectSlug string

	cmd := &cobra.Command{
		Use:   "services-sync",
		Short: "Sync service definitions from YAML files to Enclii",
		Long: `Reads service YAML files from a directory and registers missing services in Enclii.

This command is useful for bootstrapping services from dogfooding specs or
maintaining service definitions as code.

Examples:
  # Sync services from dogfooding directory
  enclii services-sync --dir dogfooding/ --project enclii

  # Dry run to see what would be created
  enclii services-sync --dir dogfooding/ --project enclii --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServicesSync(cfg, dir, projectSlug, dryRun)
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "dogfooding/", "Directory containing service YAML files")
	cmd.Flags().StringVar(&projectSlug, "project", "enclii", "Project slug to register services under")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")

	return cmd
}

func runServicesSync(cfg *config.Config, dir, projectSlug string, dryRun bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create API client
	apiClient := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Check API health
	fmt.Println("Checking API connection...")
	health, err := apiClient.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}
	fmt.Printf("Connected to %s (version %s)\n\n", health.Service, health.Version)

	// Get existing services
	fmt.Printf("Fetching existing services for project '%s'...\n", projectSlug)
	existingServices, err := apiClient.ListServices(ctx, projectSlug)
	if err != nil {
		// If project doesn't exist, we might need to create it
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			fmt.Printf("Warning: Project '%s' not found. Services will be created if project exists.\n", projectSlug)
			existingServices = []*types.Service{}
		} else {
			return fmt.Errorf("failed to list existing services: %w", err)
		}
	}

	existingMap := make(map[string]*types.Service)
	for _, svc := range existingServices {
		existingMap[svc.Name] = svc
	}
	fmt.Printf("   Found %d existing services\n\n", len(existingServices))

	// Read YAML files from directory
	fmt.Printf("Scanning '%s' for service YAML files...\n", dir)
	specs, err := readRawServiceSpecs(dir)
	if err != nil {
		return fmt.Errorf("failed to read service specs: %w", err)
	}
	fmt.Printf("   Found %d service specifications\n\n", len(specs))

	if len(specs) == 0 {
		fmt.Println("No service YAML files found. Nothing to sync.")
		return nil
	}

	// Determine actions
	var toCreate []*RawServiceSpec
	var alreadyExists []*RawServiceSpec

	for _, s := range specs {
		if _, exists := existingMap[s.Metadata.Name]; exists {
			alreadyExists = append(alreadyExists, s)
		} else {
			toCreate = append(toCreate, s)
		}
	}

	// Print summary
	fmt.Println("Sync Summary:")
	fmt.Printf("   Already registered: %d\n", len(alreadyExists))
	for _, s := range alreadyExists {
		fmt.Printf("      - %s\n", s.Metadata.Name)
	}
	fmt.Printf("   To be created: %d\n", len(toCreate))
	for _, s := range toCreate {
		fmt.Printf("      - %s\n", s.Metadata.Name)
	}
	fmt.Println()

	if len(toCreate) == 0 {
		fmt.Println("All services are already registered. Nothing to do.")
		return nil
	}

	if dryRun {
		fmt.Println("DRY RUN - No changes will be made")
		fmt.Println()
		for _, s := range toCreate {
			fmt.Printf("Would create service '%s':\n", s.Metadata.Name)
			fmt.Printf("  Project: %s\n", projectSlug)
			fmt.Printf("  Git Repo: %s\n", getGitRepoFromSpec(s))
			fmt.Printf("  App Path: %s\n", s.Spec.Build.Source.Git.Path)
			fmt.Printf("  Build Type: %s\n", s.Spec.Build.Type)
			fmt.Printf("  Auto Deploy: %t\n", s.Spec.Build.Source.Git.AutoDeploy)
			fmt.Println()
		}
		return nil
	}

	// Create missing services
	fmt.Println("Creating missing services...")
	successCount := 0
	failCount := 0

	for _, s := range toCreate {
		service := rawSpecToService(s)
		fmt.Printf("   Creating '%s'... ", s.Metadata.Name)

		createdService, err := apiClient.CreateService(ctx, projectSlug, service)
		if err != nil {
			fmt.Printf("Failed: %v\n", err)
			failCount++
			continue
		}

		fmt.Printf("Created (ID: %s)\n", createdService.ID)
		successCount++
	}

	fmt.Println()
	fmt.Printf("Results: %d created, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("%d services failed to create", failCount)
	}

	fmt.Println("\nSync complete! Services are now registered and ready for GitHub webhooks.")
	return nil
}

func readRawServiceSpecs(dir string) ([]*RawServiceSpec, error) {
	var specs []*RawServiceSpec

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("directory not found: %s", dir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	// Walk directory looking for YAML files
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Skip non-service files (e.g., kustomization.yaml, etc.)
		filename := strings.ToLower(info.Name())
		if strings.Contains(filename, "kustomization") ||
			strings.Contains(filename, "patch") ||
			strings.Contains(filename, "secret") {
			return nil
		}

		// Try to parse as service spec
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("   Warning: Skipping %s (cannot read file)\n", filepath.Base(path))
			return nil
		}

		var spec RawServiceSpec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			fmt.Printf("   Warning: Skipping %s (not valid YAML)\n", filepath.Base(path))
			return nil
		}

		// Only include Service kind
		if spec.Kind != "Service" {
			return nil
		}

		// Validate minimum required fields
		if spec.Metadata.Name == "" {
			fmt.Printf("   Warning: Skipping %s (missing metadata.name)\n", filepath.Base(path))
			return nil
		}

		specs = append(specs, &spec)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return specs, nil
}

func getGitRepoFromSpec(s *RawServiceSpec) string {
	if s.Spec.Build.Source.Git.Repository != "" {
		return s.Spec.Build.Source.Git.Repository
	}
	// Default for enclii monorepo
	return "https://github.com/madfam-org/enclii"
}

func rawSpecToService(s *RawServiceSpec) *types.Service {
	gitRepo := getGitRepoFromSpec(s)
	appPath := s.Spec.Build.Source.Git.Path
	autoDeploy := s.Spec.Build.Source.Git.AutoDeploy
	autoDeployBranch := s.Spec.Build.Source.Git.Branch
	if autoDeployBranch == "" {
		autoDeployBranch = "main"
	}

	buildType := s.Spec.Build.Type
	if buildType == "" {
		buildType = "auto"
	}

	return &types.Service{
		Name:             s.Metadata.Name,
		GitRepo:          gitRepo,
		AppPath:          appPath,
		AutoDeploy:       autoDeploy,
		AutoDeployBranch: autoDeployBranch,
		AutoDeployEnv:    "production",
		BuildConfig: types.BuildConfig{
			Type: types.BuildType(buildType),
		},
	}
}
