package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/madfam/enclii/packages/cli/internal/config"
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

	// TODO: Fetch real data from API
	// Mock data for demonstration
	services := []ServiceStatus{
		{
			Name:     "api",
			Status:   "running",
			Health:   "healthy",
			Replicas: "2/2",
			Version:  "v2024.01.15-14.02",
			Uptime:   "2h 15m",
		},
		{
			Name:     "worker",
			Status:   "running",
			Health:   "healthy",
			Replicas: "1/1",
			Version:  "v2024.01.15-14.02",
			Uptime:   "2h 15m",
		},
	}

	// Print header
	fmt.Printf("%-12s %-10s %-10s %-10s %-20s %-10s\n",
		"NAME", "STATUS", "HEALTH", "REPLICAS", "VERSION", "UPTIME")
	fmt.Println(strings.Repeat("─", 80))

	// Print services
	for _, svc := range services {
		statusColor := getStatusColor(svc.Status)
		healthColor := getHealthColor(svc.Health)
		
		fmt.Printf("%-12s %s%-10s%s %s%-10s%s %-10s %-20s %-10s\n",
			svc.Name,
			statusColor, svc.Status, "\033[0m",
			healthColor, svc.Health, "\033[0m",
			svc.Replicas, svc.Version, svc.Uptime)
	}

	fmt.Println()
	fmt.Println("💡 Use 'enclii logs <service>' to view logs")
	fmt.Println("💡 Use 'enclii deploy --env <env>' to deploy updates")

	return nil
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