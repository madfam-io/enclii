package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// DashboardStats represents the dashboard statistics
type DashboardStats struct {
	HealthyServices  int    `json:"healthy_services"`
	DeploymentsToday int    `json:"deployments_today"`
	ActiveProjects   int    `json:"active_projects"`
	AvgDeployTime    string `json:"avg_deploy_time"`
}

// RecentActivity represents a recent activity item
type RecentActivity struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Status    string                 `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceOverview represents a service in the dashboard
type ServiceOverview struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	Replicas    string `json:"replicas"`
}

// DashboardResponse contains all dashboard data
type DashboardResponse struct {
	Stats      DashboardStats    `json:"stats"`
	Activities []RecentActivity  `json:"activities"`
	Services   []ServiceOverview `json:"services"`
}

// GetDashboardStats returns public dashboard statistics
// This endpoint does not require authentication for local development
func (h *Handler) GetDashboardStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get stats from database
	stats, err := h.getDashboardStats(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to get dashboard stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get dashboard stats"})
		return
	}

	// Get recent activities
	activities, err := h.getRecentActivities(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to get recent activities")
		activities = []RecentActivity{} // Return empty array on error
	}

	// Get services overview
	services, err := h.getServicesOverview(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to get services overview")
		services = []ServiceOverview{} // Return empty array on error
	}

	c.JSON(http.StatusOK, DashboardResponse{
		Stats:      stats,
		Activities: activities,
		Services:   services,
	})
}

// getDashboardStats retrieves dashboard statistics from the database
func (h *Handler) getDashboardStats(ctx context.Context) (DashboardStats, error) {
	stats := DashboardStats{
		AvgDeployTime: "N/A",
	}

	// Count active projects
	projects, err := h.projectService.ListProjects(ctx)
	if err == nil {
		stats.ActiveProjects = len(projects)
	}

	// Count services and their health status
	healthyCount := 0
	for _, project := range projects {
		services, err := h.projectService.ListServices(ctx, project.Slug)
		if err != nil {
			continue
		}
		for _, svc := range services {
			// Get latest deployment status for each service
			deployment, err := h.repos.Deployments.GetLatestByService(ctx, svc.ID.String())
			if err == nil && deployment != nil {
				if deployment.Health == types.HealthStatusHealthy {
					healthyCount++
				}
			} else {
				// Fallback: Query Kubernetes directly for services without deployment records
				namespace := strings.ToLower(project.Slug)
				k8sStatus, err := h.k8sClient.GetDeploymentStatusInfo(ctx, namespace, svc.Name)
				if err == nil && k8sStatus != nil {
					if k8sStatus.AvailableReplicas == k8sStatus.Replicas && k8sStatus.Replicas > 0 {
						healthyCount++
					}
				}
			}
		}
	}
	stats.HealthyServices = healthyCount

	// Count deployments today
	todayStart := time.Now().Truncate(24 * time.Hour)
	deploymentsToday := 0

	// Iterate through all projects and services to count deployments
	for _, project := range projects {
		services, err := h.projectService.ListServices(ctx, project.Slug)
		if err != nil {
			continue
		}
		for _, svc := range services {
			releases, err := h.repos.Releases.ListByService(svc.ID)
			if err != nil {
				continue
			}
			for _, release := range releases {
				deployments, err := h.repos.Deployments.ListByRelease(ctx, release.ID.String())
				if err != nil {
					continue
				}
				for _, d := range deployments {
					if d.CreatedAt.After(todayStart) {
						deploymentsToday++
					}
				}
			}
		}
	}
	stats.DeploymentsToday = deploymentsToday

	// Calculate average deploy time (simplified - using recent deployments)
	// In a real implementation, this would query deployment metrics
	if deploymentsToday > 0 {
		stats.AvgDeployTime = "2.3m" // Placeholder - would calculate from actual data
	}

	return stats, nil
}

// getRecentActivities retrieves recent deployment activities
func (h *Handler) getRecentActivities(ctx context.Context) ([]RecentActivity, error) {
	activities := []RecentActivity{}

	// Get all projects
	projects, err := h.projectService.ListProjects(ctx)
	if err != nil {
		return activities, err
	}

	// Collect recent deployments from all services
	for _, project := range projects {
		services, err := h.projectService.ListServices(ctx, project.Slug)
		if err != nil {
			h.logger.Warn(ctx, "Failed to list services for project, skipping",
				logging.String("project", project.Slug),
				logging.Error("error", err))
			continue
		}

		for _, svc := range services {
			releases, err := h.repos.Releases.ListByService(svc.ID)
			if err != nil {
				h.logger.Warn(ctx, "Failed to list releases for service, skipping",
					logging.String("service_id", svc.ID.String()),
					logging.String("service_name", svc.Name),
					logging.Error("error", err))
				continue
			}

			for _, release := range releases {
				deployments, err := h.repos.Deployments.ListByRelease(ctx, release.ID.String())
				if err != nil {
					h.logger.Warn(ctx, "Failed to list deployments for release, skipping",
						logging.String("release_id", release.ID.String()),
						logging.Error("error", err))
					continue
				}

				for _, d := range deployments {
					status := "pending"
					switch d.Status {
					case types.DeploymentStatusRunning:
						status = "success"
					case types.DeploymentStatusFailed:
						status = "failed"
					case types.DeploymentStatusPending:
						status = "pending"
					default:
						// Handle any unexpected status values gracefully
						h.logger.Debug(ctx, "Unexpected deployment status, defaulting to pending",
							logging.String("deployment_id", d.ID.String()),
							logging.String("status", string(d.Status)))
						status = string(d.Status) // Use the actual status value for transparency
					}

					activities = append(activities, RecentActivity{
						ID:        d.ID.String(),
						Type:      "deployment",
						Message:   "Deployed " + svc.Name + " to " + project.Name,
						Timestamp: d.CreatedAt,
						Status:    status,
						Metadata: map[string]interface{}{
							"version":     release.Version,
							"environment": "production",
						},
					})
				}
			}
		}
	}

	// Sort by timestamp (most recent first) and limit to 10
	// Simple bubble sort for small lists
	for i := 0; i < len(activities)-1; i++ {
		for j := 0; j < len(activities)-i-1; j++ {
			if activities[j].Timestamp.Before(activities[j+1].Timestamp) {
				activities[j], activities[j+1] = activities[j+1], activities[j]
			}
		}
	}

	if len(activities) > 10 {
		activities = activities[:10]
	}

	return activities, nil
}

// getServicesOverview retrieves an overview of all services
func (h *Handler) getServicesOverview(ctx context.Context) ([]ServiceOverview, error) {
	overview := []ServiceOverview{}

	// Get all projects
	projects, err := h.projectService.ListProjects(ctx)
	if err != nil {
		return overview, err
	}

	for _, project := range projects {
		services, err := h.projectService.ListServices(ctx, project.Slug)
		if err != nil {
			continue
		}

		for _, svc := range services {
			status := "unknown"
			version := "N/A"
			replicas := "0/0"

			// Get latest deployment from database
			deployment, err := h.repos.Deployments.GetLatestByService(ctx, svc.ID.String())
			if err == nil && deployment != nil {
				switch deployment.Health {
				case types.HealthStatusHealthy:
					status = "healthy"
				case types.HealthStatusUnhealthy:
					status = "unhealthy"
				default:
					status = "unknown"
				}
				replicas = "1/1" // Simplified - would get from K8s in production

				// Get release for version
				release, err := h.repos.Releases.GetByID(deployment.ReleaseID)
				if err == nil {
					version = release.Version
				}
			} else {
				// Fallback: Query Kubernetes directly for services without deployment records
				// This handles services deployed manually via kubectl
				namespace := strings.ToLower(project.Slug)
				k8sStatus, err := h.k8sClient.GetDeploymentStatusInfo(ctx, namespace, svc.Name)
				if err == nil && k8sStatus != nil {
					// Determine health from K8s replica counts
					if k8sStatus.AvailableReplicas == k8sStatus.Replicas && k8sStatus.Replicas > 0 {
						status = "healthy"
					} else if k8sStatus.AvailableReplicas > 0 {
						status = "unhealthy"
					}
					replicas = fmt.Sprintf("%d/%d", k8sStatus.AvailableReplicas, k8sStatus.Replicas)
					// Use image tag as version fallback
					if k8sStatus.ImageTag != "" {
						version = k8sStatus.ImageTag
					}
				}
			}

			overview = append(overview, ServiceOverview{
				ID:          svc.ID.String(),
				Name:        svc.Name,
				ProjectName: project.Name,
				Environment: "production", // Simplified
				Status:      status,
				Version:     version,
				Replicas:    replicas,
			})
		}
	}

	return overview, nil
}
