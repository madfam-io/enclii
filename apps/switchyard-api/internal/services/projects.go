package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// ProjectService handles project and service management business logic
type ProjectService struct {
	repos  *db.Repositories
	logger *logrus.Logger
}

// NewProjectService creates a new project service
func NewProjectService(
	repos *db.Repositories,
	logger *logrus.Logger,
) *ProjectService {
	return &ProjectService{
		repos:  repos,
		logger: logger,
	}
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Name        string
	Slug        string
	Description string
	UserID      string
	UserEmail   string
	UserRole    string
}

// CreateProjectResponse represents the response from creating a project
type CreateProjectResponse struct {
	Project *types.Project
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, req *CreateProjectRequest) (*CreateProjectResponse, error) {
	// Validate input
	if err := s.validateProjectInput(req.Name, req.Slug); err != nil {
		return nil, err
	}

	// Check if slug already exists
	existing, _ := s.repos.Project.GetBySlug(ctx, req.Slug)
	if existing != nil {
		return nil, errors.ErrSlugAlreadyExists.WithDetails(map[string]any{
			"slug": req.Slug,
		})
	}

	s.logger.WithFields(logrus.Fields{
		"name": req.Name,
		"slug": req.Slug,
	}).Info("Creating new project")

	// Create project
	project := &types.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.Project.Create(ctx, project); err != nil {
		s.logger.Error("Failed to create project", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "project_created",
		ResourceType: "project",
		ResourceID:   project.ID,
		ResourceName: project.Name,
		Outcome:      "success",
	})

	return &CreateProjectResponse{
		Project: project,
	}, nil
}

// GetProject retrieves a project by slug
func (s *ProjectService) GetProject(ctx context.Context, slug string) (*types.Project, error) {
	project, err := s.repos.Project.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.Error("Failed to get project", "slug", slug, "error", err)
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	return project, nil
}

// ListProjects lists all projects
func (s *ProjectService) ListProjects(ctx context.Context) ([]*types.Project, error) {
	projects, err := s.repos.Project.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list projects", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return projects, nil
}

// CreateServiceRequest represents a request to create a service
type CreateServiceRequest struct {
	ProjectID   string
	Name        string
	GitRepo     string
	BuildConfig types.BuildConfig
	UserID      string
	UserEmail   string
	UserRole    string
}

// CreateServiceResponse represents the response from creating a service
type CreateServiceResponse struct {
	Service *types.Service
}

// CreateService creates a new service in a project
func (s *ProjectService) CreateService(ctx context.Context, req *CreateServiceRequest) (*CreateServiceResponse, error) {
	// Validate project exists
	_, err := s.repos.Project.GetByID(ctx, req.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	// Validate input
	if err := s.validateServiceInput(req.Name, req.GitRepo); err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"project_id": req.ProjectID,
		"name":       req.Name,
		"git_repo":   req.GitRepo,
	}).Info("Creating new service")

	// Create service
	service := &types.Service{
		ID:          uuid.New().String(),
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		GitRepo:     req.GitRepo,
		BuildConfig: req.BuildConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.Service.Create(ctx, service); err != nil {
		s.logger.Error("Failed to create service", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "service_created",
		ResourceType: "service",
		ResourceID:   service.ID,
		ResourceName: service.Name,
		Outcome:      "success",
		Context: map[string]interface{}{
			"project_id": req.ProjectID,
		},
	})

	return &CreateServiceResponse{
		Service: service,
	}, nil
}

// GetService retrieves a service by ID
func (s *ProjectService) GetService(ctx context.Context, serviceID string) (*types.Service, error) {
	service, err := s.repos.Service.GetByID(ctx, serviceID)
	if err != nil {
		s.logger.Error("Failed to get service", "service_id", serviceID, "error", err)
		return nil, errors.Wrap(err, errors.ErrServiceNotFound)
	}

	return service, nil
}

// ListServices lists all services for a project
func (s *ProjectService) ListServices(ctx context.Context, projectSlug string) ([]*types.Service, error) {
	// Validate project exists
	project, err := s.repos.Project.GetBySlug(ctx, projectSlug)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	services, err := s.repos.Service.ListByProject(ctx, project.ID)
	if err != nil {
		s.logger.Error("Failed to list services", "project_slug", projectSlug, "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return services, nil
}

// validateProjectInput validates project creation input
func (s *ProjectService) validateProjectInput(name, slug string) error {
	if strings.TrimSpace(name) == "" {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name is required",
		})
	}

	if len(name) > 100 {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name must be 100 characters or less",
		})
	}

	// Validate slug format (lowercase, alphanumeric, hyphens)
	if !isValidSlug(slug) {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "slug",
			"reason": "Slug must contain only lowercase letters, numbers, and hyphens",
		})
	}

	if len(slug) < 3 || len(slug) > 50 {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "slug",
			"reason": "Slug must be between 3 and 50 characters",
		})
	}

	return nil
}

// validateServiceInput validates service creation input
func (s *ProjectService) validateServiceInput(name, gitRepo string) error {
	if strings.TrimSpace(name) == "" {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name is required",
		})
	}

	if len(name) > 100 {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "name",
			"reason": "Name must be 100 characters or less",
		})
	}

	// Validate git repository URL format
	if !isValidGitRepo(gitRepo) {
		return errors.ErrValidation.WithDetails(map[string]any{
			"field":  "git_repo",
			"reason": "Invalid git repository URL",
		})
	}

	return nil
}

// isValidSlug checks if a slug is valid (lowercase alphanumeric + hyphens)
func isValidSlug(slug string) bool {
	slugRegex := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	return slugRegex.MatchString(slug)
}

// isValidGitRepo checks if a git repository URL is valid
func isValidGitRepo(repo string) bool {
	if strings.TrimSpace(repo) == "" {
		return false
	}

	// Simple validation: must start with http(s):// or git@
	return strings.HasPrefix(repo, "http://") ||
		strings.HasPrefix(repo, "https://") ||
		strings.HasPrefix(repo, "git@")
}

// GenerateSlug generates a URL-friendly slug from a name
func GenerateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Ensure minimum length
	if len(slug) < 3 {
		slug = fmt.Sprintf("%s-%d", slug, time.Now().Unix()%10000)
	}

	return slug
}
