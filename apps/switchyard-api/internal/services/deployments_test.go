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

func newTestDeploymentService() (*DeploymentService, *db.Repositories) {
	repos := testutil.MockRepositories()
	auditLogger := audit.NewAsyncLogger(repos.AuditLogs, logrus.New(), 100)
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Note: builder, k8s, sbom, signer are nil for now - can be mocked later if needed
	service := NewDeploymentService(repos, nil, nil, nil, nil, auditLogger, logger)
	return service, repos
}

func TestDeploymentService_BuildService(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	// Create a service first
	svc := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		BuildConfig: types.BuildConfig{
			Type: types.BuildTypeAuto,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	tests := []struct {
		name      string
		req       *BuildServiceRequest
		setupMock func()
		wantErr   bool
		errType   *errors.AppError
	}{
		{
			name: "valid build request",
			req: &BuildServiceRequest{
				ServiceID: svc.ID,
				GitSHA:    "abc123def456",
				UserID:    uuid.New(),
			},
			setupMock: func() {},
			wantErr:   false,
		},
		{
			name: "non-existent service",
			req: &BuildServiceRequest{
				ServiceID: uuid.New(),
				GitSHA:    "abc123def456",
				UserID:    uuid.New(),
			},
			setupMock: func() {},
			wantErr:   true,
			errType:   errors.ErrServiceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			resp, err := service.BuildService(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildService() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("BuildService() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("BuildService() unexpected error: %v", err)
				return
			}

			if resp.Release == nil {
				t.Error("BuildService() release is nil")
				return
			}

			if resp.Release.Status != types.ReleaseStatusBuilding {
				t.Errorf("BuildService() release status = %v, want %v", resp.Release.Status, types.ReleaseStatusBuilding)
			}

			if resp.Release.ServiceID != tt.req.ServiceID {
				t.Errorf("BuildService() service ID = %v, want %v", resp.Release.ServiceID, tt.req.ServiceID)
			}

			if resp.Release.GitSHA != tt.req.GitSHA {
				t.Errorf("BuildService() git SHA = %v, want %v", resp.Release.GitSHA, tt.req.GitSHA)
			}

			// Verify release was created in repository
			createdRelease, err := repos.Releases.GetByID(resp.Release.ID)
			if err != nil {
				t.Errorf("BuildService() release not found in repository: %v", err)
			}
			if createdRelease.Status != types.ReleaseStatusBuilding {
				t.Errorf("BuildService() persisted release status = %v, want building", createdRelease.Status)
			}
		})
	}
}

func TestDeploymentService_DeployService(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	// Create service, release, and environment
	svc := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	readyRelease := &types.Release{
		ID:        uuid.New(),
		ServiceID: svc.ID,
		Version:   "v1.0.0",
		ImageURI:  "registry.io/image:tag",
		GitSHA:    "abc123",
		Status:    types.ReleaseStatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Releases.Create(readyRelease)

	buildingRelease := &types.Release{
		ID:        uuid.New(),
		ServiceID: svc.ID,
		Version:   "v1.0.1",
		ImageURI:  "registry.io/image:tag2",
		GitSHA:    "def456",
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Releases.Create(buildingRelease)

	env := &types.Environment{
		ID:            uuid.New(),
		ProjectID:     svc.ProjectID,
		Name:          "production",
		KubeNamespace: "prod",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Environments.Create(env)

	tests := []struct {
		name    string
		req     *DeployServiceRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid deployment",
			req: &DeployServiceRequest{
				ServiceID:     svc.ID,
				ReleaseID:     readyRelease.ID,
				EnvironmentID: env.ID,
				Replicas:      3,
				UserID:        uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "non-existent release",
			req: &DeployServiceRequest{
				ServiceID:     svc.ID,
				ReleaseID:     uuid.New(),
				EnvironmentID: env.ID,
				Replicas:      3,
				UserID:        uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrReleaseNotFound,
		},
		{
			name: "release not ready",
			req: &DeployServiceRequest{
				ServiceID:     svc.ID,
				ReleaseID:     buildingRelease.ID,
				EnvironmentID: env.ID,
				Replicas:      3,
				UserID:        uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrBuildFailed,
		},
		{
			name: "non-existent environment",
			req: &DeployServiceRequest{
				ServiceID:     svc.ID,
				ReleaseID:     readyRelease.ID,
				EnvironmentID: uuid.New(),
				Replicas:      3,
				UserID:        uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.DeployService(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeployService() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("DeployService() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("DeployService() unexpected error: %v", err)
				return
			}

			if resp.Deployment == nil {
				t.Error("DeployService() deployment is nil")
				return
			}

			if resp.Deployment.ReleaseID != tt.req.ReleaseID {
				t.Errorf("DeployService() release ID = %v, want %v", resp.Deployment.ReleaseID, tt.req.ReleaseID)
			}

			if resp.Deployment.EnvironmentID != tt.req.EnvironmentID {
				t.Errorf("DeployService() environment ID = %v, want %v", resp.Deployment.EnvironmentID, tt.req.EnvironmentID)
			}

			if resp.Deployment.Replicas != tt.req.Replicas {
				t.Errorf("DeployService() replicas = %v, want %v", resp.Deployment.Replicas, tt.req.Replicas)
			}

			if resp.Deployment.Status != types.DeploymentStatusPending {
				t.Errorf("DeployService() status = %v, want pending", resp.Deployment.Status)
			}
		})
	}
}

func TestDeploymentService_Rollback(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	// Create service
	svc := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	// Create two releases (current and previous)
	previousRelease := &types.Release{
		ID:        uuid.New(),
		ServiceID: svc.ID,
		Version:   "v1.0.0",
		ImageURI:  "registry.io/image:v1",
		GitSHA:    "abc123",
		Status:    types.ReleaseStatusReady,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	}
	repos.Releases.Create(previousRelease)

	currentRelease := &types.Release{
		ID:        uuid.New(),
		ServiceID: svc.ID,
		Version:   "v2.0.0",
		ImageURI:  "registry.io/image:v2",
		GitSHA:    "def456",
		Status:    types.ReleaseStatusReady,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}
	repos.Releases.Create(currentRelease)

	// Create environment
	env := &types.Environment{
		ID:            uuid.New(),
		ProjectID:     svc.ProjectID,
		Name:          "production",
		KubeNamespace: "prod",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Environments.Create(env)

	// Create current deployment
	currentDeployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     currentRelease.ID,
		EnvironmentID: env.ID,
		Replicas:      3,
		Status:        types.DeploymentStatusRunning,
		Health:        types.HealthStatusHealthy,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Deployments.Create(currentDeployment)

	tests := []struct {
		name    string
		req     *RollbackRequest
		wantErr bool
		errType *errors.AppError
	}{
		{
			name: "valid rollback",
			req: &RollbackRequest{
				DeploymentID: currentDeployment.ID,
				UserID:       uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "non-existent deployment",
			req: &RollbackRequest{
				DeploymentID: uuid.New(),
				UserID:       uuid.New(),
			},
			wantErr: true,
			errType: errors.ErrDeploymentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.Rollback(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Rollback() expected error, got nil")
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("Rollback() error = %v, want error type %v", err, tt.errType.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("Rollback() unexpected error: %v", err)
				return
			}

			if resp.NewDeployment == nil {
				t.Error("Rollback() new deployment is nil")
				return
			}

			// Verify rolled back to previous release
			if resp.NewDeployment.ReleaseID != previousRelease.ID {
				t.Errorf("Rollback() release ID = %v, want previous release %v", resp.NewDeployment.ReleaseID, previousRelease.ID)
			}

			if resp.NewDeployment.Status != types.DeploymentStatusPending {
				t.Errorf("Rollback() status = %v, want pending", resp.NewDeployment.Status)
			}
		})
	}
}

func TestDeploymentService_Rollback_NoValidPreviousRelease(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	// Create service with only one release
	svc := &types.Service{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	currentRelease := &types.Release{
		ID:        uuid.New(),
		ServiceID: svc.ID,
		Version:   "v1.0.0",
		ImageURI:  "registry.io/image:v1",
		GitSHA:    "abc123",
		Status:    types.ReleaseStatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Releases.Create(currentRelease)

	env := &types.Environment{
		ID:            uuid.New(),
		ProjectID:     svc.ProjectID,
		Name:          "production",
		KubeNamespace: "prod",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Environments.Create(env)

	currentDeployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     currentRelease.ID,
		EnvironmentID: env.ID,
		Replicas:      3,
		Status:        types.DeploymentStatusRunning,
		Health:        types.HealthStatusHealthy,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Deployments.Create(currentDeployment)

	req := &RollbackRequest{
		DeploymentID: currentDeployment.ID,
		UserID:       uuid.New(),
	}

	_, err := service.Rollback(ctx, req)

	if err == nil {
		t.Error("Rollback() expected error for no previous release, got nil")
		return
	}

	if !errors.Is(err, errors.ErrRollbackFailed) {
		t.Errorf("Rollback() error = %v, want ErrRollbackFailed", err)
	}
}

func TestDeploymentService_GetDeploymentStatus(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	deployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     uuid.New(),
		EnvironmentID: uuid.New(),
		Replicas:      3,
		Status:        types.DeploymentStatusRunning,
		Health:        types.HealthStatusHealthy,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	repos.Deployments.Create(deployment)

	tests := []struct {
		name         string
		deploymentID uuid.UUID
		wantErr      bool
	}{
		{
			name:         "existing deployment",
			deploymentID: deployment.ID,
			wantErr:      false,
		},
		{
			name:         "non-existent deployment",
			deploymentID: uuid.New(),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetDeploymentStatus(ctx, tt.deploymentID)

			if tt.wantErr {
				if err == nil {
					t.Error("GetDeploymentStatus() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetDeploymentStatus() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("GetDeploymentStatus() result is nil")
				return
			}

			if result.ID != tt.deploymentID {
				t.Errorf("GetDeploymentStatus() ID = %v, want %v", result.ID, tt.deploymentID)
			}
		})
	}
}

func TestDeploymentService_ListServiceDeployments(t *testing.T) {
	service, repos := newTestDeploymentService()
	ctx := context.Background()

	serviceID := uuid.New()

	// Create service
	svc := &types.Service{
		ID:        serviceID,
		ProjectID: uuid.New(),
		Name:      "test-service",
		GitRepo:   "https://github.com/user/repo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repos.Services.Create(svc)

	// Create releases and deployments
	for i := 0; i < 3; i++ {
		release := &types.Release{
			ID:        uuid.New(),
			ServiceID: serviceID,
			Version:   "v1.0." + string(rune('0'+i)),
			ImageURI:  "registry.io/image:tag",
			GitSHA:    "abc123",
			Status:    types.ReleaseStatusReady,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repos.Releases.Create(release)

		deployment := &types.Deployment{
			ID:            uuid.New(),
			ReleaseID:     release.ID,
			EnvironmentID: uuid.New(),
			Replicas:      3,
			Status:        types.DeploymentStatusRunning,
			Health:        types.HealthStatusHealthy,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		repos.Deployments.Create(deployment)
	}

	deployments, err := service.ListServiceDeployments(ctx, serviceID)

	if err != nil {
		t.Errorf("ListServiceDeployments() unexpected error: %v", err)
		return
	}

	if len(deployments) != 3 {
		t.Errorf("ListServiceDeployments() count = %d, want 3", len(deployments))
	}
}
