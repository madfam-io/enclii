package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

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

	// TODO: Implement actual log streaming
	// This would connect to the API and stream logs
	
	// Mock log output for demonstration
	mockLogs := []string{
		"2024-01-15 10:30:01 [INFO] Server starting on port 8080",
		"2024-01-15 10:30:02 [INFO] Database connection established",
		"2024-01-15 10:30:03 [INFO] Health check endpoint ready",
		"2024-01-15 10:30:10 [INFO] GET /health - 200 OK (2ms)",
		"2024-01-15 10:31:05 [INFO] GET /api/projects - 200 OK (15ms)",
	}

	for _, log := range mockLogs {
		fmt.Println(log)
	}

	if follow {
		fmt.Println("⏳ Waiting for more logs... (Press Ctrl+C to exit)")
		// In real implementation, this would stream logs continuously
		select {} // Block forever in demo
	}

	return nil
}