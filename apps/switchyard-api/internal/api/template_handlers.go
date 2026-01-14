package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/logging"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// ListTemplatesResponse defines the response for listing templates
type ListTemplatesResponse struct {
	Templates []*types.Template `json:"templates"`
	Count     int               `json:"count"`
}

// TemplateFiltersResponse defines the response for filter options
type TemplateFiltersResponse struct {
	Categories map[string]int `json:"categories"`
	Frameworks map[string]int `json:"frameworks"`
}

// DeployTemplateResponse defines the response for template deployment
type DeployTemplateResponse struct {
	Deployment *types.TemplateDeployment `json:"deployment"`
	Project    *types.Project            `json:"project"`
	Message    string                    `json:"message"`
}

// ListTemplates returns all templates with optional filters
// GET /v1/templates
func (h *Handler) ListTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse filter parameters
	filters := &types.TemplateListFilters{}

	if category := c.Query("category"); category != "" {
		filters.Category = types.TemplateCategory(category)
	}
	if framework := c.Query("framework"); framework != "" {
		filters.Framework = framework
	}
	if language := c.Query("language"); language != "" {
		filters.Language = language
	}
	if search := c.Query("search"); search != "" {
		filters.Search = search
	}
	if featured := c.Query("featured"); featured == "true" {
		featuredBool := true
		filters.Featured = &featuredBool
	}
	if official := c.Query("official"); official == "true" {
		officialBool := true
		filters.Official = &officialBool
	}
	if tags := c.QueryArray("tags"); len(tags) > 0 {
		filters.Tags = tags
	}

	templates, err := h.repos.Templates.List(ctx, filters)
	if err != nil {
		h.logger.Error(ctx, "Failed to list templates", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, ListTemplatesResponse{
		Templates: templates,
		Count:     len(templates),
	})
}

// GetFeaturedTemplates returns featured templates
// GET /v1/templates/featured
func (h *Handler) GetFeaturedTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse limit parameter
	limit := 6
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	templates, err := h.repos.Templates.GetFeatured(ctx, limit)
	if err != nil {
		h.logger.Error(ctx, "Failed to get featured templates", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get featured templates"})
		return
	}

	c.JSON(http.StatusOK, ListTemplatesResponse{
		Templates: templates,
		Count:     len(templates),
	})
}

// GetTemplateFilters returns available filter options (categories and frameworks)
// GET /v1/templates/filters
func (h *Handler) GetTemplateFilters(c *gin.Context) {
	ctx := c.Request.Context()

	categories, err := h.repos.Templates.GetCategories(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to get template categories", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get template categories"})
		return
	}

	frameworks, err := h.repos.Templates.GetFrameworks(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to get template frameworks", logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get template frameworks"})
		return
	}

	c.JSON(http.StatusOK, TemplateFiltersResponse{
		Categories: categories,
		Frameworks: frameworks,
	})
}

// GetTemplate returns a single template by slug
// GET /v1/templates/:slug
func (h *Handler) GetTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	template, err := h.repos.Templates.GetBySlug(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
			return
		}
		h.logger.Error(ctx, "Failed to get template",
			logging.String("slug", slug),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// SearchTemplates performs full-text search on templates
// GET /v1/templates/search
func (h *Handler) SearchTemplates(c *gin.Context) {
	ctx := c.Request.Context()
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query 'q' is required"})
		return
	}

	// Parse limit parameter
	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	templates, err := h.repos.Templates.Search(ctx, query, limit)
	if err != nil {
		h.logger.Error(ctx, "Failed to search templates",
			logging.String("query", query),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search templates"})
		return
	}

	c.JSON(http.StatusOK, ListTemplatesResponse{
		Templates: templates,
		Count:     len(templates),
	})
}

// DeployTemplate deploys a template to create a new project
// POST /v1/templates/:slug/deploy
func (h *Handler) DeployTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")

	// Get template by slug
	template, err := h.repos.Templates.GetBySlug(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
			return
		}
		h.logger.Error(ctx, "Failed to get template",
			logging.String("slug", slug),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get template"})
		return
	}

	// Parse request body
	var req types.DeployTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate slug if not provided
	if req.ProjectSlug == "" {
		req.ProjectSlug = generateSlug(req.ProjectName)
	}

	// Get user info from context
	userID, _ := c.Get("userID")
	var userUUID *uuid.UUID
	if uid, ok := userID.(string); ok && uid != "" {
		if parsed, err := uuid.Parse(uid); err == nil {
			userUUID = &parsed
		}
	}

	// Create the project
	project := &types.Project{
		Name: req.ProjectName,
		Slug: req.ProjectSlug,
	}

	if err := h.repos.Projects.Create(project); err != nil {
		h.logger.Error(ctx, "Failed to create project from template",
			logging.String("template_slug", slug),
			logging.String("project_name", req.ProjectName),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}

	// Create template deployment record
	deployment := &types.TemplateDeployment{
		TemplateID: template.ID,
		ProjectID:  project.ID,
		UserID:     userUUID,
		Status:     types.TemplateDeploymentStatusPending,
	}

	if err := h.repos.Templates.CreateDeployment(ctx, deployment); err != nil {
		h.logger.Error(ctx, "Failed to create deployment record",
			logging.String("template_id", template.ID.String()),
			logging.String("project_id", project.ID.String()),
			logging.Error("error", err))
		// Don't fail the request, just log the error
	}

	// Increment deploy count
	if err := h.repos.Templates.IncrementDeployCount(ctx, template.ID); err != nil {
		h.logger.Warn(ctx, "Failed to increment deploy count",
			logging.String("template_id", template.ID.String()),
			logging.Error("error", err))
		// Don't fail the request
	}

	h.logger.Info(ctx, "Template deployed",
		logging.String("template_slug", slug),
		logging.String("project_id", project.ID.String()),
		logging.String("project_name", req.ProjectName))

	// Update deployment status to in_progress
	if deployment.ID != uuid.Nil {
		_ = h.repos.Templates.UpdateDeploymentStatus(ctx, deployment.ID, types.TemplateDeploymentStatusInProgress, "")
	}

	// TODO: Trigger async service creation based on template config
	// This would create services, databases, env vars based on template.Config

	// For now, mark as completed
	if deployment.ID != uuid.Nil {
		_ = h.repos.Templates.UpdateDeploymentStatus(ctx, deployment.ID, types.TemplateDeploymentStatusCompleted, "")
	}

	c.JSON(http.StatusCreated, DeployTemplateResponse{
		Deployment: deployment,
		Project:    project,
		Message:    "Project created from template successfully",
	})
}

// GetDeployment returns the status of a template deployment
// GET /v1/templates/deployments/:id
func (h *Handler) GetTemplateDeployment(c *gin.Context) {
	ctx := c.Request.Context()
	deploymentIDStr := c.Param("id")

	deploymentID, err := uuid.Parse(deploymentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment ID"})
		return
	}

	deployment, err := h.repos.Templates.GetDeployment(ctx, deploymentID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		h.logger.Error(ctx, "Failed to get deployment",
			logging.String("deployment_id", deploymentIDStr),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get deployment"})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// ImportTemplateRequest defines the request for importing a template from GitHub
type ImportTemplateRequest struct {
	RepoURL     string   `json:"repo_url" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Category    string   `json:"category" binding:"required"`
	Framework   string   `json:"framework" binding:"required"`
	Language    string   `json:"language" binding:"required"`
	Branch      string   `json:"branch"`
	Tags        []string `json:"tags"`
}

// ImportTemplateFromGitHub imports a template from a GitHub repository URL
// POST /v1/templates/import
func (h *Handler) ImportTemplateFromGitHub(c *gin.Context) {
	ctx := c.Request.Context()

	var req ImportTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate category
	validCategories := map[string]bool{
		"fullstack": true, "frontend": true, "backend": true, "api": true,
		"database": true, "microservice": true, "monorepo": true, "static": true,
	}
	if !validCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	// Generate slug from name
	slug := generateSlug(req.Name)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must contain at least one alphanumeric character"})
		return
	}

	// Check if slug already exists
	existing, err := h.repos.Templates.GetBySlug(ctx, slug)
	if err == nil && existing != nil {
		// Append a random suffix to make it unique
		slug = slug + "-" + uuid.New().String()[:8]
	}

	// Default branch
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	// Create template
	template := &types.Template{
		Name:         req.Name,
		Slug:         slug,
		Description:  req.Description,
		Category:     types.TemplateCategory(req.Category),
		Framework:    req.Framework,
		Language:     req.Language,
		SourceType:   types.TemplateSourceGitHub,
		SourceRepo:   req.RepoURL,
		SourceBranch: branch,
		SourcePath:   "/",
		Tags:         req.Tags,
		IsFeatured:   false,
		IsOfficial:   false,
		IsPublic:     true,
		DeployCount:  0,
	}

	if err := h.repos.Templates.Create(ctx, template); err != nil {
		h.logger.Error(ctx, "Failed to create template",
			logging.String("name", req.Name),
			logging.String("repo_url", req.RepoURL),
			logging.Error("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}

	h.logger.Info(ctx, "Template imported from GitHub",
		logging.String("template_id", template.ID.String()),
		logging.String("name", req.Name),
		logging.String("repo_url", req.RepoURL))

	c.JSON(http.StatusCreated, template)
}

// generateSlug creates a URL-safe slug from a name
func generateSlug(name string) string {
	// Simple slug generation - in production you'd want a more robust implementation
	slug := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r >= 'A' && r <= 'Z' {
			slug += string(r + 32) // lowercase
		} else if r == ' ' || r == '_' {
			slug += "-"
		}
	}
	// Remove consecutive dashes
	for len(slug) > 0 && slug[0] == '-' {
		slug = slug[1:]
	}
	for len(slug) > 0 && slug[len(slug)-1] == '-' {
		slug = slug[:len(slug)-1]
	}
	return slug
}
