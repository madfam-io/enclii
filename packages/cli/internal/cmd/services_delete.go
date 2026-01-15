package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/madfam-org/enclii/packages/cli/internal/client"
	"github.com/madfam-org/enclii/packages/cli/internal/config"
)

func NewServicesDeleteCommand(cfg *config.Config) *cobra.Command {
	var projectSlug string
	var serviceName string
	var serviceID string
	var force bool

	cmd := &cobra.Command{
		Use:   "services-delete",
		Short: "Delete a service from a project",
		Long: `Deletes a service and all associated resources (deployments, releases, etc.).

This operation is irreversible. Use with caution.

Examples:
  # Delete a service by name within a project
  enclii services-delete --project enclii --name janua-api

  # Delete a service by ID (if you know the UUID)
  enclii services-delete --id 12345678-1234-1234-1234-123456789abc

  # Force delete without confirmation prompt
  enclii services-delete --project enclii --name janua-api --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServicesDelete(cfg, projectSlug, serviceName, serviceID, force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug containing the service")
	cmd.Flags().StringVar(&serviceName, "name", "", "Name of the service to delete")
	cmd.Flags().StringVar(&serviceID, "id", "", "UUID of the service to delete (alternative to --project + --name)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func runServicesDelete(cfg *config.Config, projectSlug, serviceName, serviceID string, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate input
	if serviceID == "" && (projectSlug == "" || serviceName == "") {
		return fmt.Errorf("either --id or both --project and --name must be provided")
	}

	// Create API client
	apiClient := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Check API health
	fmt.Println("Connecting to API...")
	health, err := apiClient.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}
	fmt.Printf("Connected to %s (version %s)\n\n", health.Service, health.Version)

	// If service ID not provided, look up by project + name
	if serviceID == "" {
		fmt.Printf("Looking up service '%s' in project '%s'...\n", serviceName, projectSlug)
		services, err := apiClient.ListServices(ctx, projectSlug)
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}

		var foundService *struct {
			ID   string
			Name string
		}
		for _, svc := range services {
			if svc.Name == serviceName {
				foundService = &struct {
					ID   string
					Name string
				}{
					ID:   svc.ID.String(),
					Name: svc.Name,
				}
				break
			}
		}

		if foundService == nil {
			return fmt.Errorf("service '%s' not found in project '%s'", serviceName, projectSlug)
		}

		serviceID = foundService.ID
		fmt.Printf("Found service: %s (ID: %s)\n\n", foundService.Name, foundService.ID)
	}

	// Confirm deletion unless --force is set
	if !force {
		fmt.Printf("WARNING: This will permanently delete the service and all associated resources:\n")
		fmt.Printf("  - All deployments\n")
		fmt.Printf("  - All releases\n")
		fmt.Printf("  - All environment variables\n")
		fmt.Printf("  - All custom domains\n")
		fmt.Printf("  - All routes\n")
		fmt.Printf("\n")
		fmt.Printf("Service ID: %s\n", serviceID)
		if serviceName != "" {
			fmt.Printf("Service Name: %s\n", serviceName)
		}
		fmt.Printf("\n")
		fmt.Print("Type 'DELETE' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		confirmation, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		confirmation = strings.TrimSpace(confirmation)
		if confirmation != "DELETE" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Delete the service
	fmt.Printf("Deleting service %s...\n", serviceID)
	if err := apiClient.DeleteService(ctx, serviceID); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	fmt.Println("Service deleted successfully.")
	return nil
}
