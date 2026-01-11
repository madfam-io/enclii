package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/madfam-org/enclii/packages/cli/internal/client"
	"github.com/madfam-org/enclii/packages/cli/internal/config"
)

func NewPsCommand(cfg *config.Config) *cobra.Command {
	var environment string

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List services and their status",
		Long:  "Show running services, their health, and resource usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listServices(cfg, environment)
		},
	}

	cmd.Flags().StringVarP(&environment, "env", "e", "dev", "Environment to list services for")

	return cmd
}

func listServices(cfg *config.Config, environment string) error {
	fmt.Printf("📊 Services in %s environment\n", environment)
	fmt.Println()

	ctx := context.Background()
	apiClient := client.NewAPIClient(cfg.APIEndpoint, cfg.APIToken)

	// Get project slug from config
	projectSlug := cfg.Project
	if projectSlug == "" {
		projectSlug = "default"
	}

	// Fetch services from API
	fmt.Println("🔍 Fetching services...")
	services, err := apiClient.ListServices(ctx, projectSlug)
	if err != nil {
		fmt.Printf("❌ Failed to list services: %v\n", err)
		return err
	}

	if len(services) == 0 {
		fmt.Println("No services found in this project")
		fmt.Println("💡 Create a service with: enclii init")
		return nil
	}

	// Fetch deployment status for each service
	var serviceStatuses []ServiceStatus
	for _, svc := range services {
		status := ServiceStatus{
			Name:     svc.Name,
			Status:   "unknown",
			Health:   "unknown",
			Replicas: "0/0",
			Version:  "N/A",
			Uptime:   "N/A",
		}

		// Get latest deployment
		deploymentResp, err := apiClient.GetLatestDeployment(ctx, svc.ID.String())
		if err == nil && deploymentResp.Deployment != nil {
			deployment := deploymentResp.Deployment

			// Map deployment status
			status.Status = string(deployment.Status)
			status.Health = string(deployment.Health)
			status.Replicas = fmt.Sprintf("%d/%d", deployment.Replicas, deployment.Replicas)

			// Get version from release
			if deploymentResp.Release != nil {
				status.Version = deploymentResp.Release.Version
				if len(deploymentResp.Release.GitSHA) >= 7 {
					status.Version += fmt.Sprintf(" (%s)", deploymentResp.Release.GitSHA[:7])
				}
			}

			// Calculate uptime
			uptime := time.Since(deployment.CreatedAt)
			status.Uptime = formatDuration(uptime)
		}

		serviceStatuses = append(serviceStatuses, status)
	}

	// Print header
	fmt.Printf("%-15s %-12s %-12s %-12s %-30s %-10s\n",
		"NAME", "STATUS", "HEALTH", "REPLICAS", "VERSION", "UPTIME")
	fmt.Println(strings.Repeat("─", 95))

	// Print services
	for _, svc := range serviceStatuses {
		statusColor := getStatusColor(svc.Status)
		healthColor := getHealthColor(svc.Health)

		fmt.Printf("%-15s %s%-12s%s %s%-12s%s %-12s %-30s %-10s\n",
			svc.Name,
			statusColor, svc.Status, "\033[0m",
			healthColor, svc.Health, "\033[0m",
			svc.Replicas, svc.Version, svc.Uptime)
	}

	fmt.Println()
	fmt.Printf("Total: %d service(s)\n", len(serviceStatuses))
	fmt.Println()
	fmt.Println("💡 Use 'enclii logs <service>' to view logs")
	fmt.Println("💡 Use 'enclii deploy --env <env>' to deploy updates")

	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

type ServiceStatus struct {
	Name     string
	Status   string
	Health   string
	Replicas string
	Version  string
	Uptime   string
}

func getStatusColor(status string) string {
	switch status {
	case "running":
		return "\033[32m" // Green
	case "pending":
		return "\033[33m" // Yellow
	case "failed":
		return "\033[31m" // Red
	default:
		return "\033[37m" // White
	}
}

func getHealthColor(health string) string {
	switch health {
	case "healthy":
		return "\033[32m" // Green
	case "unhealthy":
		return "\033[31m" // Red
	default:
		return "\033[33m" // Yellow
	}
}
