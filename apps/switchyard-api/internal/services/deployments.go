package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/audit"
	"github.com/madfam/enclii/apps/switchyard-api/internal/builder"
	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/apps/switchyard-api/internal/sbom"
	"github.com/madfam/enclii/apps/switchyard-api/internal/signing"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// DeploymentService handles deployment business logic
type DeploymentService struct {
	repos        *db.Repositories
	builder      *builder.BuildpacksBuilder
	k8sClient    *k8s.Client
	sbomGen      *sbom.Generator
	signer       *signing.Signer
	auditLogger  *audit.AsyncLogger
	logger       *logrus.Logger
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(
	repos *db.Repositories,
	builder *builder.BuildpacksBuilder,
	k8sClient *k8s.Client,
	sbomGen *sbom.Generator,
	signer *signing.Signer,
	auditLogger *audit.AsyncLogger,
	logger *logrus.Logger,
) *DeploymentService {
	return &DeploymentService{
		repos:       repos,
		builder:     builder,
		k8sClient:   k8sClient,
		sbomGen:     sbomGen,
		signer:      signer,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

// BuildServiceRequest represents a request to build a service
type BuildServiceRequest struct {
	ServiceID uuid.UUID
	GitSHA    string
	UserID    uuid.UUID
}

// BuildServiceResponse represents the result of a build operation
type BuildServiceResponse struct {
	Release *types.Release
	Logs    []string
}

// BuildService builds a new release for a service
func (s *DeploymentService) BuildService(ctx context.Context, req *BuildServiceRequest) (*BuildServiceResponse, error) {
	// Get service
	service, err := s.repos.Services.GetByID(req.ServiceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrServiceNotFound)
	}

	s.logger.WithFields(logrus.Fields{
		"service_id": req.ServiceID,
		"git_sha":    req.GitSHA,
	}).Info("Starting service build")

	// Create release record
	release := &types.Release{
		ID:        uuid.New(),
		ServiceID: req.ServiceID,
		Version:   fmt.Sprintf("v%s-%s", time.Now().Format("20060102-150405"), req.GitSHA[:7]),
		GitSHA:    req.GitSHA,
		Status:    types.ReleaseStatusBuilding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repos.Releases.Create(release); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "build_started",
		ResourceType: "release",
		ResourceID:   release.ID.String(),
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
	ServiceID     uuid.UUID
	ReleaseID     uuid.UUID
	EnvironmentID uuid.UUID
	Replicas      int
	UserID        uuid.UUID
}

// DeployServiceResponse represents the result of a deployment
type DeployServiceResponse struct {
	Deployment *types.Deployment
}

// DeployService deploys a release to an environment
func (s *DeploymentService) DeployService(ctx context.Context, req *DeployServiceRequest) (*DeployServiceResponse, error) {
	// Validate release exists and is ready
	release, err := s.repos.Releases.GetByID(req.ReleaseID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrReleaseNotFound)
	}

	if release.Status != types.ReleaseStatusReady {
		return nil, errors.ErrBuildFailed.WithDetails(map[string]any{
			"status": release.Status,
			"reason": "Release is not ready for deployment",
		})
	}

	// Validate environment exists
	_, err = s.repos.Environments.GetByID(ctx, req.EnvironmentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrNotFound.WithDetails("environment"))
	}

	s.logger.WithFields(logrus.Fields{
		"service_id":     req.ServiceID,
		"release_id":     req.ReleaseID,
		"environment_id": req.EnvironmentID,
	}).Info("Starting deployment")

	// Create deployment record
	deployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     req.ReleaseID,
		EnvironmentID: req.EnvironmentID,
		Replicas:      req.Replicas,
		Status:        types.DeploymentStatusPending,
		Health:        types.HealthStatusUnknown,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repos.Deployments.Create(deployment); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "deployment_created",
		ResourceType: "deployment",
		ResourceID:   deployment.ID.String(),
		Outcome:      "success",
	})

	// Deployment will be picked up by reconciler
	return &DeployServiceResponse{
		Deployment: deployment,
	}, nil
}

// RollbackRequest represents a request to rollback a deployment
type RollbackRequest struct {
	DeploymentID uuid.UUID
	UserID       uuid.UUID
}

// RollbackResponse represents the result of a rollback
type RollbackResponse struct {
	NewDeployment *types.Deployment
}

// Rollback rolls back to a previous deployment
func (s *DeploymentService) Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	// Get current deployment
	deployment, err := s.repos.Deployments.GetByID(ctx, req.DeploymentID.String())
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	// Get release
	release, err := s.repos.Releases.GetByID(deployment.ReleaseID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrReleaseNotFound)
	}

	// Get previous release for the service
	releases, err := s.repos.Releases.ListByService(release.ServiceID)
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
		ID:            uuid.New(),
		ReleaseID:     previousRelease.ID,
		EnvironmentID: deployment.EnvironmentID,
		Replicas:      deployment.Replicas,
		Status:        types.DeploymentStatusPending,
		Health:        types.HealthStatusUnknown,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repos.Deployments.Create(newDeployment); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.auditLogger.LogAction(ctx, &audit.AuditLogEntry{
		Actor:        req.UserID.String(),
		Action:       "deployment_rollback",
		ResourceType: "deployment",
		ResourceID:   newDeployment.ID.String(),
		Outcome:      "success",
		Context: map[string]interface{}{
			"previous_deployment": deployment.ID.String(),
			"previous_release":    release.ID.String(),
			"rolled_back_to":      previousRelease.ID.String(),
		},
	})

	return &RollbackResponse{
		NewDeployment: newDeployment,
	}, nil
}

// GetDeploymentStatus retrieves the current status of a deployment
func (s *DeploymentService) GetDeploymentStatus(ctx context.Context, deploymentID uuid.UUID) (*types.Deployment, error) {
	deployment, err := s.repos.Deployments.GetByID(ctx, deploymentID.String())
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	return deployment, nil
}

// ListServiceDeployments lists all deployments for a service
func (s *DeploymentService) ListServiceDeployments(ctx context.Context, serviceID uuid.UUID) ([]*types.Deployment, error) {
	// Get all releases for the service
	releases, err := s.repos.Releases.ListByService(serviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	var allDeployments []*types.Deployment
	for _, release := range releases {
		deployments, err := s.repos.Deployments.ListByRelease(ctx, release.ID.String())
		if err != nil {
			continue // Skip on error
		}
		allDeployments = append(allDeployments, deployments...)
	}

	return allDeployments, nil
}
