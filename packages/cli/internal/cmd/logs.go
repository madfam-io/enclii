package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/madfam/enclii/packages/cli/internal/client"
	"github.com/madfam/enclii/packages/cli/internal/config"
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
	client := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Note: This is a simplified implementation
	// In production, we would:
	// 1. Get project from service.yaml or config
	// 2. List services in that project to find the service by name
	// 3. Get the latest deployment for that service
	// 4. Stream logs from that deployment

	// For now, we'll use a placeholder deployment ID
	// The real implementation would query the API to get the current deployment ID
	fmt.Println("🔍 Finding deployment...")

	// TODO: Add API endpoint to get deployment by service name and environment
	// For now, showing error message with instructions
	fmt.Println()
	fmt.Println("⚠️  Log streaming requires deployment ID")
	fmt.Println("💡 Use 'enclii ps' to list deployments")
	fmt.Println("💡 Then use 'enclii logs --deployment <id>' to view logs")
	fmt.Println()
	fmt.Println("Alternative: Query logs directly from Kubernetes:")
	fmt.Printf("   kubectl logs -l enclii.dev/service=%s -n enclii-dev --tail=%d", serviceName, lines)
	if follow {
		fmt.Print(" -f")
	}
	fmt.Println()

	return nil
}