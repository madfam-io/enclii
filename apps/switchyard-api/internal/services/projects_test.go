package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/testutil"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

func newTestProjectService() (*ProjectService, *db.Repositories) {
	repos := testutil.MockRepositories()
	auditLogger := audit.NewAsyncLogger(repos.AuditLogs, logrus.New(), 100)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := NewProjectService(repos, auditLogger, logger)
	return service, repos
}

func TestProjectService_CreateProject(t *testing.T) {
	tests := []struct {
		name    string
		req     *CreateProjectRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid project",
			req: &CreateProjectRequest{
				Name:   "Test Project",
				Slug:   "test-project",
				UserID: uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "empty name",
			req: &CreateProjectRequest{
				Name:   "",
				Slug:   "test-project",
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "name too long",
			req: &CreateProjectRequest{
				Name:   string(make([]byte, 101)),
				Slug:   "test-project",
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "invalid slug format - uppercase",
			req: &CreateProjectRequest{
				Name:   "Test Project",
				Slug:   "Test-Project",
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "invalid slug format - special chars",
			req: &CreateProjectRequest{
				Name:   "Test Project",
				Slug:   "test_project",
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "slug too short",
			req: &CreateProjectRequest{
				Name:   "Test",
				Slug:   "ab",
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "slug too long",
			req: &CreateProjectRequest{
				Name:   "Test",
				Slug:   string(make([]byte, 51)),
				UserID: uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newTestProjectService()
			ctx := context.Background()

			resp, err := service.CreateProject(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateProject() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("CreateProject() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateProject() unexpected error: %v", err)
				return
			}

			if resp.Project == nil {
				t.Error("CreateProject() project is nil")
				return
			}

			if resp.Project.Name != tt.req.Name {
				t.Errorf("CreateProject() name = %v, want %v", resp.Project.Name, tt.req.Name)
			}

			if resp.Project.Slug != tt.req.Slug {
				t.Errorf("CreateProject() slug = %v, want %v", resp.Project.Slug, tt.req.Slug)
			}
		})
	}
}

func TestProjectService_CreateProject_DuplicateSlug(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	// Create first project
	existingProject := &types.Project{
		ID:        uuid.New(),
		Name:      "Existing",
		Slug:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Projects.Create(existingProject)

	// Try to create with same slug
	req := &CreateProjectRequest{
		Name:   "New Project",
		Slug:   "test-project",
		UserID: uuid.New(),
	}

	_, err := service.CreateProject(ctx, req)

	if err == nil {
		t.Error("CreateProject() expected error for duplicate slug, got nil")
		return
	}

	if !errors.Is(err, errors.ErrSlugAlreadyExists) {
		t.Errorf("CreateProject() error = %v, want ErrSlugAlreadyExists", err)
	}
}

func TestProjectService_GetProject(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	// Create a project
	project := &types.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		Slug:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Projects.Create(project)

	tests := []struct {
		name       string
		identifier string
		wantErr    bool
	}{
		{
			name:       "get by ID",
			identifier: project.ID.String(),
			wantErr:    false,
		},
		{
			name:       "get by slug",
			identifier: project.Slug,
			wantErr:    false,
		},
		{
			name:       "non-existent project",
			identifier: uuid.New().String(),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetProject(ctx, tt.identifier)

			if tt.wantErr {
				if err == nil {
					t.Error("GetProject() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetProject() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("GetProject() result is nil")
			}
		})
	}
}

func TestProjectService_ListProjects(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	// Create multiple projects
	for i := 0; i < 3; i++ {
		project := &types.Project{
			ID:        uuid.New(),
			Name:      "Project " + string(rune('A'+i)),
			Slug:      "project-" + string(rune('a'+i)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repos.Projects.Create(project)
	}

	projects, err := service.ListProjects(ctx)

	if err != nil {
		t.Errorf("ListProjects() unexpected error: %v", err)
		return
	}

	if len(projects) != 3 {
		t.Errorf("ListProjects() count = %d, want 3", len(projects))
	}
}

func TestProjectService_CreateService(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	// Create a project first
	project := &types.Project{
		ID:        uuid.New(),
		Name:      "Test Project",
		Slug:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Projects.Create(project)

	tests := []struct {
		name    string
		req     *CreateServiceRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid service",
			req: &CreateServiceRequest{
				ProjectID: project.ID,
				Name:      "test-service",
				GitRepo:   "https://github.com/user/repo",
				BuildConfig: types.BuildConfig{
					Type: types.BuildTypeAuto,
				},
				UserID: uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "empty name",
			req: &CreateServiceRequest{
				ProjectID: project.ID,
				Name:      "",
				GitRepo:   "https://github.com/user/repo",
				UserID:    uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "name too long",
			req: &CreateServiceRequest{
				ProjectID: project.ID,
				Name:      string(make([]byte, 101)),
				GitRepo:   "https://github.com/user/repo",
				UserID:    uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "invalid git repo URL",
			req: &CreateServiceRequest{
				ProjectID: project.ID,
				Name:      "test-service",
				GitRepo:   "not-a-git-url",
				UserID:    uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrValidation,
		},
		{
			name: "valid git@ URL",
			req: &CreateServiceRequest{
				ProjectID: project.ID,
				Name:      "test-service",
				GitRepo:   "git@github.com:user/repo.git",
				UserID:    uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "non-existent project",
			req: &CreateServiceRequest{
				ProjectID: uuid.New(),
				Name:      "test-service",
				GitRepo:   "https://github.com/user/repo",
				UserID:    uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrProjectNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.CreateService(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateService() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("CreateService() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateService() unexpected error: %v", err)
				return
			}

			if resp.Service == nil {
				t.Error("CreateService() service is nil")
				return
			}

			if resp.Service.Name != tt.req.Name {
				t.Errorf("CreateService() name = %v, want %v", resp.Service.Name, tt.req.Name)
			}
		})
	}
}

func TestProjectService_GetService(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	// Create a service
	svc := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	tests := []struct {
		name      string
		serviceID uuid.UUID
		wantErr   bool
	}{
		{
			name:      "existing service",
			serviceID: svc.ID,
			wantErr:   false,
		},
		{
			name:      "non-existent service",
			serviceID: uuid.New(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetService(ctx, tt.serviceID)

			if tt.wantErr {
				if err == nil {
					t.Error("GetService() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetService() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("GetService() result is nil")
			}
		})
	}
}

func TestProjectService_ListServices(t *testing.T) {
	service, repos := newTestProjectService()
	ctx := context.Background()

	projectID := uuid.New()

	// Create a project
	project := &types.Project{
		ID:        projectID,
		Name:      "Test Project",
		Slug:      "test-project",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Projects.Create(project)

	// Create multiple services
	for i := 0; i < 3; i++ {
		svc := &types.Service{
			ID:        uuid.New(),
			ProjectID: projectID,
			Name:      "service-" + string(rune('a'+i)),
			GitRepo:   "https://github.com/user/repo",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repos.Services.Create(svc)
	}

	services, err := service.ListServices(ctx, projectID)

	if err != nil {
		t.Errorf("ListServices() unexpected error: %v", err)
		return
	}

	if len(services) != 3 {
		t.Errorf("ListServices() count = %d, want 3", len(services))
	}
}

func Test_isValidSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"test-project", true},
		{"my-app", true},
		{"app123", true},
		{"123app", true},
		{"test-app-123", true},
		{"Test-Project", false}, // uppercase
		{"test_project", false}, // underscore
		{"test project", false}, // space
		{"-test", false},        // starts with hyphen
		{"test-", false},        // ends with hyphen
		{"", false},
		{"ab", true}, // minimum length at boundary
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			if got := isValidSlug(tt.slug); got != tt.want {
				t.Errorf("isValidSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}

func Test_isValidGitRepo(t *testing.T) {
	tests := []struct {
		repo string
		want bool
	}{
		{"https://github.com/user/repo", true},
		{"http://gitlab.com/user/repo", true},
		{"git@github.com:user/repo.git", true},
		{"not-a-git-url", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			if got := isValidGitRepo(tt.repo); got != tt.want {
				t.Errorf("isValidGitRepo(%q) = %v, want %v", tt.repo, got, tt.want)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Test Project", "test-project"},
		{"My App", "my-app"},
		{"TEST-APP", "test-app"},
		{"App@123", "app123"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"---test---", "test"},
		{"a", "a-"},      // Will append timestamp
		{"ab", "ab-"},    // Will append timestamp
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := GenerateSlug(tt.name)

			// For very short names, just check it's not empty and longer than input
			if len(tt.name) < 3 {
				if slug == "" {
					t.Error("GenerateSlug() returned empty string")
				}
				if len(slug) <= len(tt.name) {
					t.Errorf("GenerateSlug() should append timestamp for short names")
				}
				return
			}

			if slug != tt.expected {
				t.Errorf("GenerateSlug(%q) = %v, want %v", tt.name, slug, tt.expected)
			}
		})
	}
}
