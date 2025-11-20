package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// DeploymentService handles deployment business logic
// Note: This is a simplified version focused on deployment orchestration.
// Builder, SBOM, and signing functionality will be added in future iterations.
type DeploymentService struct {
	repos  *db.Repositories
	logger *logrus.Logger
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(
	repos *db.Repositories,
	logger *logrus.Logger,
) *DeploymentService {
	return &DeploymentService{
		repos:  repos,
		logger: logger,
	}
}

// BuildServiceRequest represents a request to build a service
type BuildServiceRequest struct {
	ServiceID string
	GitSHA    string
	UserID    string
	UserEmail string
	UserRole  string
}

// BuildServiceResponse represents the result of a build operation
type BuildServiceResponse struct {
	Release *types.Release
	Logs    []string
}

// BuildService builds a new release for a service
func (s *DeploymentService) BuildService(ctx context.Context, req *BuildServiceRequest) (*BuildServiceResponse, error) {
	// Get service
	service, err := s.repos.Service.GetByID(ctx, req.ServiceID)
	if err != nil {
		s.logger.Error("Failed to get service", "service_id", req.ServiceID, "error", err)
		return nil, errors.Wrap(err, errors.ErrServiceNotFound)
	}

	s.logger.WithFields(logrus.Fields{
		"service_id": req.ServiceID,
		"git_sha":    req.GitSHA,
	}).Info("Starting service build")

	// Create release record
	release := &types.Release{
		ID:        uuid.New().String(),
		ServiceID: req.ServiceID,
		Version:   fmt.Sprintf("v%s-%s", time.Now().Format("20060102-150405"), req.GitSHA[:7]),
		GitSHA:    req.GitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repos.Release.Create(ctx, release); err != nil {
		s.logger.Error("Failed to create release", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "build_started",
		ResourceType: "release",
		ResourceID:   release.ID,
		ResourceName: fmt.Sprintf("%s@%s", service.Name, req.GitSHA[:7]),
		Outcome:      "success",
	})

	// Perform build asynchronously (in real implementation)
	// For now, return the building release
	return &BuildServiceResponse{
		Release: release,
		Logs:    []string{"Build started..."},
	}, nil
}

// DeployServiceRequest represents a request to deploy a service
type DeployServiceRequest struct {
	ServiceID     string
	ReleaseID     string
	EnvironmentID string
	Replicas      int
	UserID        string
	UserEmail     string
	UserRole      string
}

// DeployServiceResponse represents the result of a deployment
type DeployServiceResponse struct {
	Deployment *types.Deployment
}

// DeployService deploys a release to an environment
func (s *DeploymentService) DeployService(ctx context.Context, req *DeployServiceRequest) (*DeployServiceResponse, error) {
	// Validate release exists and is ready
	release, err := s.repos.Release.GetByID(ctx, req.ReleaseID)
	if err != nil {
		s.logger.Error("Failed to get release", "release_id", req.ReleaseID, "error", err)
		return nil, errors.Wrap(err, errors.ErrReleaseNotFound)
	}

	if release.Status != types.ReleaseStatusReady {
		return nil, errors.ErrBuildFailed.WithDetails(map[string]any{
			"status": release.Status,
			"reason": "Release is not ready for deployment",
		})
	}

	s.logger.WithFields(logrus.Fields{
		"service_id":     req.ServiceID,
		"release_id":     req.ReleaseID,
		"environment_id": req.EnvironmentID,
	}).Info("Starting deployment")

	// Create deployment record
	deployment := &types.Deployment{
		ID:        uuid.New().String(),
		ServiceID: req.ServiceID,
		ReleaseID: req.ReleaseID,
		Replicas:  req.Replicas,
		Status:    types.DeploymentStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repos.Deployment.Create(ctx, deployment); err != nil {
		s.logger.Error("Failed to create deployment", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "deployment_created",
		ResourceType: "deployment",
		ResourceID:   deployment.ID,
		ResourceName: fmt.Sprintf("deployment-%s", deployment.ID[:8]),
		Outcome:      "success",
		Context: map[string]interface{}{
			"release_id":     req.ReleaseID,
			"environment_id": req.EnvironmentID,
		},
	})

	// Deployment will be picked up by reconciler
	return &DeployServiceResponse{
		Deployment: deployment,
	}, nil
}

// RollbackRequest represents a request to rollback a deployment
type RollbackRequest struct {
	DeploymentID string
	UserID       string
	UserEmail    string
	UserRole     string
}

// RollbackResponse represents the result of a rollback
type RollbackResponse struct {
	NewDeployment *types.Deployment
}

// Rollback rolls back to a previous deployment
func (s *DeploymentService) Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	// Get current deployment
	deployment, err := s.repos.Deployment.GetByID(ctx, req.DeploymentID)
	if err != nil {
		s.logger.Error("Failed to get deployment", "deployment_id", req.DeploymentID, "error", err)
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	// Get release
	release, err := s.repos.Release.GetByID(ctx, deployment.ReleaseID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrReleaseNotFound)
	}

	// Get previous release for the service
	releases, err := s.repos.Release.ListByService(ctx, release.ServiceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	if len(releases) < 2 {
		return nil, errors.ErrRollbackFailed.WithDetails("No previous release available")
	}

	// Find previous release (second in list, assuming sorted by created_at DESC)
	var previousRelease *types.Release
	for _, r := range releases {
		if r.ID != release.ID && r.Status == types.ReleaseStatusReady {
			previousRelease = r
			break
		}
	}

	if previousRelease == nil {
		return nil, errors.ErrRollbackFailed.WithDetails("No valid previous release found")
	}

	s.logger.WithFields(logrus.Fields{
		"current_release":  release.ID,
		"previous_release": previousRelease.ID,
		"deployment_id":    deployment.ID,
	}).Info("Rolling back deployment")

	// Create new deployment with previous release
	newDeployment := &types.Deployment{
		ID:        uuid.New().String(),
		ServiceID: deployment.ServiceID,
		ReleaseID: previousRelease.ID,
		Replicas:  deployment.Replicas,
		Status:    types.DeploymentStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repos.Deployment.Create(ctx, newDeployment); err != nil {
		s.logger.Error("Failed to create rollback deployment", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      req.UserID,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "deployment_rollback",
		ResourceType: "deployment",
		ResourceID:   newDeployment.ID,
		ResourceName: fmt.Sprintf("rollback-%s", newDeployment.ID[:8]),
		Outcome:      "success",
		Context: map[string]interface{}{
			"previous_deployment": deployment.ID,
			"previous_release":    release.ID,
			"rolled_back_to":      previousRelease.ID,
		},
	})

	return &RollbackResponse{
		NewDeployment: newDeployment,
	}, nil
}

// GetDeploymentStatus retrieves the current status of a deployment
func (s *DeploymentService) GetDeploymentStatus(ctx context.Context, deploymentID string) (*types.Deployment, error) {
	deployment, err := s.repos.Deployment.GetByID(ctx, deploymentID)
	if err != nil {
		s.logger.Error("Failed to get deployment", "deployment_id", deploymentID, "error", err)
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	return deployment, nil
}

// ListServiceDeployments lists all deployments for a service
func (s *DeploymentService) ListServiceDeployments(ctx context.Context, serviceID string) ([]*types.Deployment, error) {
	// Get all releases for the service
	releases, err := s.repos.Release.ListByService(ctx, serviceID)
	if err != nil {
		s.logger.Error("Failed to list releases", "service_id", serviceID, "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	var allDeployments []*types.Deployment
	for _, release := range releases {
		deployments, err := s.repos.Deployment.ListByRelease(ctx, release.ID)
		if err != nil {
			s.logger.Warn("Failed to list deployments for release", "release_id", release.ID, "error", err)
			continue // Skip on error
		}
		allDeployments = append(allDeployments, deployments...)
	}

	return allDeployments, nil
}

// ListReleases lists all releases for a service
func (s *DeploymentService) ListReleases(ctx context.Context, serviceID string) ([]*types.Release, error) {
	releases, err := s.repos.Release.ListByService(ctx, serviceID)
	if err != nil {
		s.logger.Error("Failed to list releases", "service_id", serviceID, "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return releases, nil
}
