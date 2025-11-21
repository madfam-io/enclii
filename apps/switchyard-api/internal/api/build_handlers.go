package api

import (
	"context"
	"fmt"
	"net/http"
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
func (h *Handler) triggerBuild(service *types.Service, release *types.Release, gitSHA string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

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
		h.repos.Releases.UpdateStatus(release.ID, types.ReleaseStatusFailed)
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
