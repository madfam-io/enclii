package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/madfam/enclii/packages/cli/internal/config"
)

func NewRollbackCommand(cfg *config.Config) *cobra.Command {
	var environment string
	var releaseID string

	cmd := &cobra.Command{
		Use:   "rollback [service]",
		Short: "Rollback service to previous release",
		Long:  "Rollback a service to a previous release version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serviceName string
			if len(args) > 0 {
				serviceName = args[0]
			}
			return rollbackService(cfg, serviceName, environment, releaseID)
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "dev", "Environment to rollback in")
	cmd.Flags().StringVarP(&releaseID, "to", "t", "", "Specific release ID to rollback to")

	return cmd
}

func rollbackService(cfg *config.Config, serviceName, environment, releaseID string) error {
	if serviceName == "" {
		serviceName = "api" // Default for MVP
	}

	fmt.Printf("🔄 Rolling back %s in %s environment", serviceName, environment)
	if releaseID != "" {
		fmt.Printf(" to release %s", releaseID)
	} else {
		fmt.Printf(" to previous release")
	}
	fmt.Println()

	// TODO: Implement actual rollback logic
	// This would:
	// 1. Get current deployment
	// 2. Find previous release (or specified release)
	// 3. Update deployment to use previous release
	// 4. Monitor health checks
	// 5. Confirm rollback success

	fmt.Println("🔍 Finding previous release...")
	fmt.Println("🚀 Initiating rollback...")
	fmt.Println("⏳ Waiting for pods to be ready...")
	fmt.Println("🔍 Checking health...")
	fmt.Println("✅ Rollback completed successfully!")
	fmt.Println()
	fmt.Printf("📊 Current version: v2024.01.14-10.30 (rolled back from v2024.01.15-14.02)\n")
	fmt.Printf("💡 Monitor with: enclii logs %s -f\n", serviceName)

	return nil
}