package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// BuildService triggers a build for a service from a given git SHA
func (h *Handler) BuildService(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	var req struct {
		GitSHA string `json:"git_sha" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get service details
	service, err := h.repos.Services.GetByID(serviceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to get service", logging.Error("db_error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
		return
	}

	// Create release record
	release := &types.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		Version:   "v" + time.Now().Format("20060102-150405") + "-" + req.GitSHA[:7],
		ImageURI:  h.config.Registry + "/" + service.Name + ":" + req.GitSHA[:7],
		GitSHA:    req.GitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repos.Releases.Create(release); err != nil {
		h.logger.Error(ctx, "Failed to create release", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create release"})
		return
	}

	// Trigger async build process
	go h.triggerBuild(service, release, req.GitSHA)

	c.JSON(http.StatusCreated, release)
}

// triggerBuild is a helper method that executes the build process asynchronously
// Uses a semaphore to serialize builds and prevent OOM from concurrent operations
func (h *Handler) triggerBuild(service *types.Service, release *types.Release, gitSHA string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Acquire build semaphore (blocks if another build is running)
	h.logger.Info(ctx, "Waiting for build slot",
		logging.String("service_id", service.ID.String()),
		logging.String("release_id", release.ID.String()))

	select {
	case h.buildSemaphore <- struct{}{}:
		// Acquired semaphore, ensure we release it when done
		defer func() { <-h.buildSemaphore }()
	case <-ctx.Done():
		h.logger.Error(ctx, "Build timed out waiting for semaphore",
			logging.String("release_id", release.ID.String()))
		_ = h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
		return
	}

	h.logger.Info(ctx, "Starting build process",
		logging.String("service_id", service.ID.String()),
		logging.String("release_id", release.ID.String()),
		logging.String("git_sha", gitSHA))

	// Execute the build
	buildResult := h.builder.BuildFromGit(ctx, service, gitSHA)

	if !buildResult.Success {
		h.logger.Error(ctx, "Build failed",
			logging.String("release_id", release.ID.String()),
			logging.Error("build_error", buildResult.Error))

		// Update release status to failed
		if err := h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed); err != nil {
			h.logger.Error(ctx, "Failed to update release status", logging.Error("db_error", err))
		}

		// Store build logs (in production, we'd save these to a logging service or database)
		h.logger.Error(ctx, "Build logs", logging.String("logs", fmt.Sprintf("%v", buildResult.Logs)))
		return
	}

	// Update release with image URI and mark as ready
	release.ImageURI = buildResult.ImageURI

	// Persist the actual image URI to the database (builder generates versioned tags)
	if err := h.repos.Releases.UpdateImageURI(release.ID, buildResult.ImageURI); err != nil {
		h.logger.Error(ctx, "Failed to update release image URI", logging.Error("db_error", err))
		_ = h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
		return
	}
	h.logger.Info(ctx, "✓ Release image URI updated", logging.String("image_uri", buildResult.ImageURI))

	// Store SBOM if generated
	if buildResult.SBOMGenerated && buildResult.SBOM != nil {
		h.logger.Info(ctx, "Storing SBOM",
			logging.String("release_id", release.ID.String()),
			logging.String("format", buildResult.SBOMFormat),
			logging.Int("package_count", buildResult.SBOM.PackageCount))

		if err := h.repos.Releases.UpdateSBOM(ctx, release.ID, buildResult.SBOM.Content, buildResult.SBOMFormat); err != nil {
			// SBOM storage failure is non-fatal - log warning and continue
			h.logger.Error(ctx, "Failed to store SBOM (non-fatal)", logging.Error("db_error", err))
		} else {
			h.logger.Info(ctx, "✓ SBOM stored successfully")
		}
	}

	// Store signature if generated
	if buildResult.ImageSigned && buildResult.Signature != nil {
		h.logger.Info(ctx, "Storing image signature",
			logging.String("release_id", release.ID.String()),
			logging.String("signing_method", buildResult.Signature.SigningMethod))

		if err := h.repos.Releases.UpdateSignature(ctx, release.ID, buildResult.Signature.Signature); err != nil {
			// Signature storage failure is non-fatal - log warning and continue
			h.logger.Error(ctx, "Failed to store signature (non-fatal)", logging.Error("db_error", err))
		} else {
			h.logger.Info(ctx, "✓ Image signature stored successfully")
		}
	}

	if err := h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusReady); err != nil {
		h.logger.Error(ctx, "Failed to update release status", logging.Error("db_error", err))
		_ = h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
		return
	}

	h.logger.Info(ctx, "Build completed successfully",
		logging.String("release_id", release.ID.String()),
		logging.String("image_uri", buildResult.ImageURI),
		logging.String("duration", buildResult.Duration.String()))

	// Log build details for debugging
	for _, log := range buildResult.Logs {
		h.logger.Debug(ctx, "Build log", logging.String("line", log))
	}

	// Record metrics
	// TODO: Use proper metrics methods once available
	// monitoring.RecordBuild("success", "git", buildResult.Duration)

	// Auto-deploy if enabled for this service
	if service.AutoDeploy && service.AutoDeployEnv != "" {
		h.triggerAutoDeploy(ctx, service, release)
	}
}

// triggerAutoDeploy creates a deployment for the successful build if auto-deploy is configured
func (h *Handler) triggerAutoDeploy(ctx context.Context, service *types.Service, release *types.Release) {
	h.logger.Info(ctx, "Auto-deploy triggered",
		logging.String("service_id", service.ID.String()),
		logging.String("service_name", service.Name),
		logging.String("release_id", release.ID.String()),
		logging.String("target_env", service.AutoDeployEnv))

	// Look up the target environment
	env, err := h.repos.Environments.GetByProjectAndName(service.ProjectID, service.AutoDeployEnv)
	if err != nil {
		// Environment doesn't exist - auto-create it
		h.logger.Info(ctx, "Auto-creating missing environment for auto-deploy",
			logging.String("environment", service.AutoDeployEnv),
			logging.String("project_id", service.ProjectID.String()))

		// Generate kubernetes namespace from environment name
		kubeNamespace := "enclii-" + strings.ToLower(strings.ReplaceAll(service.AutoDeployEnv, "_", "-"))

		env = &types.Environment{
			ProjectID:     service.ProjectID,
			Name:          service.AutoDeployEnv,
			KubeNamespace: kubeNamespace,
		}
		if err := h.repos.Environments.Create(env); err != nil {
			h.logger.Error(ctx, "Auto-deploy failed: could not create environment",
				logging.String("environment", service.AutoDeployEnv),
				logging.Error("db_error", err))
			return
		}

		h.logger.Info(ctx, "Successfully created environment for auto-deploy",
			logging.String("environment_id", env.ID.String()),
			logging.String("environment", service.AutoDeployEnv),
			logging.String("kube_namespace", kubeNamespace))
	}

	// Create deployment record
	deployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     release.ID,
		EnvironmentID: env.ID,
		Replicas:      1, // Default to 1 replica for auto-deploy
		Status:        types.DeploymentStatusPending,
		Health:        types.HealthStatusUnknown,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.repos.Deployments.Create(deployment); err != nil {
		h.logger.Error(ctx, "Auto-deploy failed: could not create deployment",
			logging.String("service_id", service.ID.String()),
			logging.Error("db_error", err))
		return
	}

	// Schedule deployment with reconciler (high priority)
	h.reconciler.ScheduleReconciliation(deployment.ID.String(), 1)

	h.logger.Info(ctx, "Auto-deploy scheduled successfully",
		logging.String("deployment_id", deployment.ID.String()),
		logging.String("service_name", service.Name),
		logging.String("environment", service.AutoDeployEnv))
}

// ListReleases returns all releases for a given service
func (h *Handler) ListReleases(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	serviceID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	releases, err := h.repos.Releases.ListByService(serviceID)
	if err != nil {
		h.logger.Error(ctx, "Failed to list releases", logging.Error("db_error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list releases"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}
