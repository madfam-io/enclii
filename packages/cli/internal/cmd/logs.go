package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/madfam/enclii/packages/cli/internal/client"
	"github.com/madfam/enclii/packages/cli/internal/config"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

func NewLogsCommand(cfg *config.Config) *cobra.Command {
	var follow bool
	var environment string
	var lines int

	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Show service logs",
		Long:  "Display logs for a service in the specified environment",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serviceName string
			if len(args) > 0 {
				serviceName = args[0]
			}
			return showLogs(cfg, serviceName, environment, follow, lines)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().StringVarP(&environment, "env", "e", "dev", "Environment to show logs for")
	cmd.Flags().IntVarP(&lines, "lines", "n", 100, "Number of lines to show")

	return cmd
}

func showLogs(cfg *config.Config, serviceName, environment string, follow bool, lines int) error {
	if serviceName == "" {
		// Try to detect from service.yaml
		serviceName = "api" // Default for MVP
	}

	fmt.Printf("📋 Showing logs for %s in %s environment", serviceName, environment)
	if follow {
		fmt.Printf(" (following)")
	}
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")

	ctx := context.Background()
	apiClient := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Step 1: Get project (for MVP, we'll use a default project slug)
	// In production, this would come from service.yaml or .enclii/config
	projectSlug := cfg.Project
	if projectSlug == "" {
		projectSlug = "default" // Default project for MVP
	}

	fmt.Println("🔍 Finding deployment...")

	// Step 2: List services in the project to find the service by name
	services, err := apiClient.ListServices(ctx, projectSlug)
	if err != nil {
		fmt.Printf("❌ Failed to list services: %v\n", err)
		fmt.Println()
		fmt.Println("💡 Alternative: Query logs directly from Kubernetes:")
		fmt.Printf("   kubectl logs -l enclii.dev/service=%s -n enclii-dev --tail=%d", serviceName, lines)
		if follow {
			fmt.Print(" -f")
		}
		fmt.Println()
		return err
	}

	// Find the service by name
	var targetService *types.Service
	for _, svc := range services {
		if svc.Name == serviceName {
			targetService = svc
			break
		}
	}

	if targetService == nil {
		fmt.Printf("❌ Service '%s' not found in project '%s'\n", serviceName, projectSlug)
		fmt.Println()
		fmt.Println("💡 Available services:")
		for _, svc := range services {
			fmt.Printf("   - %s\n", svc.Name)
		}
		return fmt.Errorf("service not found")
	}

	// Step 3: Get the latest deployment for this service
	deploymentResp, err := apiClient.GetLatestDeployment(ctx, targetService.ID.String())
	if err != nil {
		fmt.Printf("❌ Failed to get latest deployment: %v\n", err)
		fmt.Printf("💡 Try deploying the service first: enclii deploy --env %s\n", environment)
		return err
	}

	if deploymentResp.Deployment == nil {
		fmt.Println("❌ No active deployment found for this service")
		return fmt.Errorf("no deployment found")
	}

	fmt.Printf("✅ Found deployment: %s\n", deploymentResp.Deployment.ID)
	if deploymentResp.Release != nil {
		fmt.Printf("   Version: %s (git: %s)\n", deploymentResp.Release.Version, deploymentResp.Release.GitSHA[:7])
	}
	fmt.Println()

	// Step 4: Stream logs from the deployment
	opts := client.LogOptions{
		Follow: follow,
		Lines:  lines,
	}

	logs, err := apiClient.GetLogsRaw(ctx, deploymentResp.Deployment.ID.String(), opts)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve logs: %v\n", err)
		return err
	}

	// Print the logs
	fmt.Println(logs)

	if follow {
		fmt.Println("⏳ Waiting for more logs... (Press Ctrl+C to exit)")
		// TODO: Implement real log streaming with SSE/WebSocket
		// For now, this is a one-time fetch
	}

	return nil
}
