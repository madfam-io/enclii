package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// ProjectService handles project and service management business logic
type ProjectService struct {
	repos       *db.Repositories
	auditLogger *audit.AsyncLogger
	logger      *logrus.Logger
}

// NewProjectService creates a new project service
func NewProjectService(
	repos *db.Repositories,
	auditLogger *audit.AsyncLogger,
	logger *logrus.Logger,
) *ProjectService {
	return &ProjectService{
		repos:       repos,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Name   string
	Slug   string
	UserID uuid.UUID
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
	existing, _ := s.repos.Projects.GetBySlug(req.Slug)
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
		ID:        uuid.New(),
		Name:      req.Name,
		Slug:      req.Slug,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repos.Projects.Create(project); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "project_created",
		ResourceType: "project",
		ResourceID:   project.ID.String(),
		Outcome:      "success",
	})

	return &CreateProjectResponse{
		Project: project,
	}, nil
}

// GetProject retrieves a project by ID or slug
func (s *ProjectService) GetProject(ctx context.Context, identifier string) (*types.Project, error) {
	// Try as UUID first
	if id, err := uuid.Parse(identifier); err == nil {
		project, err := s.repos.Projects.GetByID(ctx, id)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrProjectNotFound)
		}
		return project, nil
	}

	// Try as slug
	project, err := s.repos.Projects.GetBySlug(identifier)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	return project, nil
}

// ListProjects lists all projects
func (s *ProjectService) ListProjects(ctx context.Context) ([]*types.Project, error) {
	projects, err := s.repos.Projects.List()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return projects, nil
}

// CreateServiceRequest represents a request to create a service
type CreateServiceRequest struct {
	ProjectID   uuid.UUID
	Name        string
	GitRepo     string
	BuildConfig types.BuildConfig
	UserID      uuid.UUID
}

// CreateServiceResponse represents the response from creating a service
type CreateServiceResponse struct {
	Service *types.Service
}

// CreateService creates a new service in a project
func (s *ProjectService) CreateService(ctx context.Context, req *CreateServiceRequest) (*CreateServiceResponse, error) {
	// Validate project exists
	_, err := s.repos.Projects.GetByID(ctx, req.ProjectID)
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
		ID:          uuid.New(),
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		GitRepo:     req.GitRepo,
		BuildConfig: req.BuildConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.Services.Create(service); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "service_created",
		ResourceType: "service",
		ResourceID:   service.ID.String(),
		Outcome:      "success",
		Context: map[string]interface{}{
			"project_id": req.ProjectID.String(),
		},
	})

	return &CreateServiceResponse{
		Service: service,
	}, nil
}

// GetService retrieves a service by ID
func (s *ProjectService) GetService(ctx context.Context, serviceID uuid.UUID) (*types.Service, error) {
	service, err := s.repos.Services.GetByID(serviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrServiceNotFound)
	}

	return service, nil
}

// ListServices lists all services for a project
func (s *ProjectService) ListServices(ctx context.Context, projectID uuid.UUID) ([]*types.Service, error) {
	// Validate project exists
	_, err := s.repos.Projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	services, err := s.repos.Services.ListByProject(projectID)
	if err != nil {
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
