package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// DeploymentGroupService handles deployment group business logic
// including multi-service coordinated deployments with topological ordering
type DeploymentGroupService struct {
	repos             *db.Repositories
	deploymentService *DeploymentService
	logger            *logrus.Logger
}

// NewDeploymentGroupService creates a new deployment group service
func NewDeploymentGroupService(
	repos *db.Repositories,
	deploymentService *DeploymentService,
	logger *logrus.Logger,
) *DeploymentGroupService {
	return &DeploymentGroupService{
		repos:             repos,
		deploymentService: deploymentService,
		logger:            logger,
	}
}

// CreateGroupDeploymentRequest represents a request to create a group deployment
type CreateGroupDeploymentRequest struct {
	ProjectID     string
	EnvironmentID string
	ServiceIDs    []string // Services to deploy (all if empty)
	Strategy      string   // "parallel", "dependency_ordered", "sequential"
	GitSHA        string
	PRURL         string
	TriggeredBy   string
	UserID        string
	UserEmail     string
	UserRole      string
}

// CreateGroupDeploymentResponse represents the response from creating a group deployment
type CreateGroupDeploymentResponse struct {
	Group           *db.DeploymentGroup
	DeploymentOrder [][]uuid.UUID // Layers of services to deploy in order
}

// CreateGroupDeployment creates a new coordinated deployment group
func (s *DeploymentGroupService) CreateGroupDeployment(ctx context.Context, req *CreateGroupDeploymentRequest) (*CreateGroupDeploymentResponse, error) {
	// Parse UUIDs
	projectID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}
	environmentID, err := uuid.Parse(req.EnvironmentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	// Validate project exists
	project, err := s.repos.Projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	// Get services to deploy
	var serviceIDs []uuid.UUID
	if len(req.ServiceIDs) == 0 {
		// Deploy all project services
		services, err := s.repos.Services.ListByProject(projectID)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrDatabaseError)
		}
		for _, svc := range services {
			serviceIDs = append(serviceIDs, svc.ID)
		}
	} else {
		for _, id := range req.ServiceIDs {
			svcID, err := uuid.Parse(id)
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrInvalidInput)
			}
			serviceIDs = append(serviceIDs, svcID)
		}
	}

	if len(serviceIDs) == 0 {
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"reason": "No services to deploy",
		})
	}

	// Parse strategy
	strategy := db.DeploymentGroupStrategyDependencyOrdered
	switch req.Strategy {
	case "parallel":
		strategy = db.DeploymentGroupStrategyParallel
	case "sequential":
		strategy = db.DeploymentGroupStrategySequential
	case "dependency_ordered", "":
		strategy = db.DeploymentGroupStrategyDependencyOrdered
	default:
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"field":  "strategy",
			"reason": "Invalid strategy: must be parallel, sequential, or dependency_ordered",
		})
	}

	s.logger.WithFields(logrus.Fields{
		"project_id":     req.ProjectID,
		"environment_id": req.EnvironmentID,
		"services_count": len(serviceIDs),
		"strategy":       strategy,
	}).Info("Creating deployment group")

	// Calculate deployment order using topological sort
	deploymentOrder, err := s.TopologicalSort(ctx, serviceIDs)
	if err != nil {
		return nil, err
	}

	// Create deployment group
	name := fmt.Sprintf("deploy-%s-%s", project.Slug, time.Now().Format("20060102-150405"))
	group := &db.DeploymentGroup{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		Name:          &name,
		Status:        db.DeploymentGroupStatusPending,
		Strategy:      strategy,
		TriggeredBy:   &req.TriggeredBy,
		GitSHA:        &req.GitSHA,
	}
	if req.PRURL != "" {
		group.PRURL = &req.PRURL
	}

	if err := s.repos.DeploymentGroups.Create(ctx, group); err != nil {
		s.logger.Error("Failed to create deployment group", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      nil,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "deployment_group_created",
		ResourceType: "deployment_group",
		ResourceID:   group.ID.String(),
		ResourceName: name,
		Outcome:      "success",
		Context: map[string]interface{}{
			"project_id":     req.ProjectID,
			"environment_id": req.EnvironmentID,
			"services_count": len(serviceIDs),
			"strategy":       string(strategy),
			"layers_count":   len(deploymentOrder),
		},
	})

	return &CreateGroupDeploymentResponse{
		Group:           group,
		DeploymentOrder: deploymentOrder,
	}, nil
}

// TopologicalSort returns services in dependency order as layers
// Services in the same layer have no dependencies on each other and can be deployed in parallel
// Layers are returned in order: [[no-deps], [depends-on-layer-0], [depends-on-layer-1], ...]
func (s *DeploymentGroupService) TopologicalSort(ctx context.Context, serviceIDs []uuid.UUID) ([][]uuid.UUID, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	// Build a set for quick lookup of which services are in scope
	inScope := make(map[uuid.UUID]bool)
	for _, id := range serviceIDs {
		inScope[id] = true
	}

	// Build in-degree map and adjacency list (only for services in scope)
	inDegree := make(map[uuid.UUID]int)
	dependents := make(map[uuid.UUID][]uuid.UUID) // service -> services that depend on it

	for _, id := range serviceIDs {
		inDegree[id] = 0
	}

	// Get dependencies for each service
	for _, serviceID := range serviceIDs {
		deps, err := s.repos.ServiceDependencies.GetByService(ctx, serviceID)
		if err != nil {
			s.logger.Warn("Failed to get dependencies for service", "service_id", serviceID, "error", err)
			continue
		}

		for _, dep := range deps {
			// Only count dependencies that are within our scope
			if inScope[dep.DependsOnServiceID] {
				inDegree[serviceID]++
				dependents[dep.DependsOnServiceID] = append(dependents[dep.DependsOnServiceID], serviceID)
			}
		}
	}

	// Kahn's algorithm for topological sort with layers
	var layers [][]uuid.UUID

	for len(inDegree) > 0 {
		// Find all nodes with in-degree 0
		var currentLayer []uuid.UUID
		for id, degree := range inDegree {
			if degree == 0 {
				currentLayer = append(currentLayer, id)
			}
		}

		if len(currentLayer) == 0 {
			// Cycle detected - no nodes with in-degree 0 but graph not empty
			return nil, errors.ErrValidation.WithDetails(map[string]any{
				"reason":   "Circular dependency detected in service graph",
				"services": getRemainingServiceIDs(inDegree),
			})
		}

		// Remove current layer from graph and update in-degrees
		for _, id := range currentLayer {
			delete(inDegree, id)
			for _, dependent := range dependents[id] {
				if _, exists := inDegree[dependent]; exists {
					inDegree[dependent]--
				}
			}
		}

		layers = append(layers, currentLayer)
	}

	s.logger.WithFields(logrus.Fields{
		"total_services": len(serviceIDs),
		"layers_count":   len(layers),
	}).Debug("Computed deployment order using topological sort")

	return layers, nil
}

// getRemainingServiceIDs extracts service IDs from in-degree map for error reporting
func getRemainingServiceIDs(inDegree map[uuid.UUID]int) []string {
	var ids []string
	for id := range inDegree {
		ids = append(ids, id.String())
	}
	return ids
}

// ExecuteGroupDeploymentRequest represents a request to execute a deployment group
type ExecuteGroupDeploymentRequest struct {
	GroupID   string
	UserID    string
	UserEmail string
	UserRole  string
}

// ExecuteGroupDeploymentResponse represents the result of executing a deployment group
type ExecuteGroupDeploymentResponse struct {
	Group       *db.DeploymentGroup
	Deployments []*types.Deployment
	Errors      []error
}

// ExecuteGroupDeployment orchestrates the actual deployment of a group
func (s *DeploymentGroupService) ExecuteGroupDeployment(ctx context.Context, req *ExecuteGroupDeploymentRequest) (*ExecuteGroupDeploymentResponse, error) {
	groupID, err := uuid.Parse(req.GroupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	// Get the deployment group
	group, err := s.repos.DeploymentGroups.GetByID(ctx, groupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	// Validate group is in pending status
	if group.Status != db.DeploymentGroupStatusPending {
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"reason": fmt.Sprintf("Deployment group is not pending, current status: %s", group.Status),
		})
	}

	// Mark group as started
	if err := s.repos.DeploymentGroups.UpdateStarted(ctx, groupID); err != nil {
		s.logger.Error("Failed to update group started status", "error", err)
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	s.logger.WithFields(logrus.Fields{
		"group_id": req.GroupID,
		"strategy": group.Strategy,
	}).Info("Executing deployment group")

	// Get services for this project
	services, err := s.repos.Services.ListByProject(group.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	serviceIDs := make([]uuid.UUID, len(services))
	for i, svc := range services {
		serviceIDs[i] = svc.ID
	}

	// Get deployment order
	layers, err := s.TopologicalSort(ctx, serviceIDs)
	if err != nil {
		errMsg := err.Error()
		if updateErr := s.repos.DeploymentGroups.UpdateCompleted(ctx, groupID, db.DeploymentGroupStatusFailed, &errMsg); updateErr != nil {
			s.logger.Error("Failed to update group failed status", "error", updateErr)
		}
		return nil, err
	}

	var allDeployments []*types.Deployment
	var allErrors []error

	// Execute deployments based on strategy
	switch group.Strategy {
	case db.DeploymentGroupStrategyParallel:
		allDeployments, allErrors = s.executeParallel(ctx, group, layers, req)
	case db.DeploymentGroupStrategySequential:
		allDeployments, allErrors = s.executeSequential(ctx, group, layers, req)
	case db.DeploymentGroupStrategyDependencyOrdered:
		allDeployments, allErrors = s.executeDependencyOrdered(ctx, group, layers, req)
	}

	// Update group status based on results
	var finalStatus db.DeploymentGroupStatus
	var errMsg *string
	if len(allErrors) > 0 {
		finalStatus = db.DeploymentGroupStatusFailed
		errMsgStr := fmt.Sprintf("%d deployments failed", len(allErrors))
		errMsg = &errMsgStr
	} else {
		finalStatus = db.DeploymentGroupStatusSucceeded
	}

	if err := s.repos.DeploymentGroups.UpdateCompleted(ctx, groupID, finalStatus, errMsg); err != nil {
		s.logger.Error("Failed to update group completed status", "error", err)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      nil,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "deployment_group_executed",
		ResourceType: "deployment_group",
		ResourceID:   group.ID.String(),
		ResourceName: *group.Name,
		Outcome:      string(finalStatus),
		Context: map[string]interface{}{
			"deployments_count": len(allDeployments),
			"errors_count":      len(allErrors),
		},
	})

	// Refresh group to get updated timestamps
	group, _ = s.repos.DeploymentGroups.GetByID(ctx, groupID)

	return &ExecuteGroupDeploymentResponse{
		Group:       group,
		Deployments: allDeployments,
		Errors:      allErrors,
	}, nil
}

// executeParallel deploys all services simultaneously
func (s *DeploymentGroupService) executeParallel(
	ctx context.Context,
	group *db.DeploymentGroup,
	layers [][]uuid.UUID,
	req *ExecuteGroupDeploymentRequest,
) ([]*types.Deployment, []error) {
	// Flatten all layers into single parallel deployment
	var allServiceIDs []uuid.UUID
	for _, layer := range layers {
		allServiceIDs = append(allServiceIDs, layer...)
	}

	return s.deployServicesInParallel(ctx, group, allServiceIDs, req, 0)
}

// executeSequential deploys services one at a time in order
func (s *DeploymentGroupService) executeSequential(
	ctx context.Context,
	group *db.DeploymentGroup,
	layers [][]uuid.UUID,
	req *ExecuteGroupDeploymentRequest,
) ([]*types.Deployment, []error) {
	var allDeployments []*types.Deployment
	var allErrors []error
	deployOrder := 0

	for _, layer := range layers {
		for _, serviceID := range layer {
			deployment, err := s.deployService(ctx, group, serviceID, req, deployOrder)
			deployOrder++
			if err != nil {
				allErrors = append(allErrors, err)
				// Continue to next service in sequential mode
			} else if deployment != nil {
				allDeployments = append(allDeployments, deployment)
			}
		}
	}

	return allDeployments, allErrors
}

// executeDependencyOrdered deploys services layer by layer, parallel within each layer
func (s *DeploymentGroupService) executeDependencyOrdered(
	ctx context.Context,
	group *db.DeploymentGroup,
	layers [][]uuid.UUID,
	req *ExecuteGroupDeploymentRequest,
) ([]*types.Deployment, []error) {
	var allDeployments []*types.Deployment
	var allErrors []error
	deployOrder := 0

	for layerIdx, layer := range layers {
		s.logger.WithFields(logrus.Fields{
			"group_id":      group.ID,
			"layer":         layerIdx,
			"services_count": len(layer),
		}).Debug("Deploying layer")

		// Deploy all services in this layer in parallel
		deployments, errors := s.deployServicesInParallel(ctx, group, layer, req, deployOrder)
		allDeployments = append(allDeployments, deployments...)
		allErrors = append(allErrors, errors...)
		deployOrder += len(layer)

		// If any errors in this layer, stop (dependencies for next layer may fail)
		if len(errors) > 0 {
			s.logger.WithFields(logrus.Fields{
				"group_id":     group.ID,
				"layer":        layerIdx,
				"errors_count": len(errors),
			}).Warn("Layer deployment failed, stopping group deployment")
			break
		}
	}

	return allDeployments, allErrors
}

// deployServicesInParallel deploys multiple services concurrently
func (s *DeploymentGroupService) deployServicesInParallel(
	ctx context.Context,
	group *db.DeploymentGroup,
	serviceIDs []uuid.UUID,
	req *ExecuteGroupDeploymentRequest,
	startingOrder int,
) ([]*types.Deployment, []error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var deployments []*types.Deployment
	var errors []error

	for i, serviceID := range serviceIDs {
		wg.Add(1)
		go func(svcID uuid.UUID, order int) {
			defer wg.Done()

			deployment, err := s.deployService(ctx, group, svcID, req, order)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, err)
			} else if deployment != nil {
				deployments = append(deployments, deployment)
			}
		}(serviceID, startingOrder+i)
	}

	wg.Wait()
	return deployments, errors
}

// deployService deploys a single service within a group
func (s *DeploymentGroupService) deployService(
	ctx context.Context,
	group *db.DeploymentGroup,
	serviceID uuid.UUID,
	req *ExecuteGroupDeploymentRequest,
	deployOrder int,
) (*types.Deployment, error) {
	// Get latest release for the service
	releases, err := s.repos.Releases.ListByService(serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases for service %s: %w", serviceID, err)
	}

	// Find latest ready release
	var latestRelease *types.Release
	for _, r := range releases {
		if r.Status == types.ReleaseStatusReady {
			latestRelease = r
			break
		}
	}

	if latestRelease == nil {
		return nil, fmt.Errorf("no ready release found for service %s", serviceID)
	}

	// Create deployment
	deployment := &types.Deployment{
		ID:            uuid.New(),
		ReleaseID:     latestRelease.ID,
		EnvironmentID: group.EnvironmentID,
		GroupID:       &group.ID,
		DeployOrder:   deployOrder,
		Replicas:      1, // Default replicas
		Status:        types.DeploymentStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repos.Deployments.Create(deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment for service %s: %w", serviceID, err)
	}

	s.logger.WithFields(logrus.Fields{
		"deployment_id": deployment.ID,
		"service_id":    serviceID,
		"release_id":    latestRelease.ID,
		"group_id":      group.ID,
		"deploy_order":  deployOrder,
	}).Debug("Created deployment for service in group")

	return deployment, nil
}

// RollbackGroupRequest represents a request to rollback a deployment group
type RollbackGroupRequest struct {
	GroupID   string
	UserID    string
	UserEmail string
	UserRole  string
}

// RollbackGroupResponse represents the result of rolling back a group
type RollbackGroupResponse struct {
	Group        *db.DeploymentGroup
	RolledBack   int
	FailedToRoll int
	Errors       []error
}

// RollbackGroup rolls back all deployments in a group
func (s *DeploymentGroupService) RollbackGroup(ctx context.Context, req *RollbackGroupRequest) (*RollbackGroupResponse, error) {
	groupID, err := uuid.Parse(req.GroupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	// Get the deployment group
	group, err := s.repos.DeploymentGroups.GetByID(ctx, groupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	s.logger.WithFields(logrus.Fields{
		"group_id": req.GroupID,
		"status":   group.Status,
	}).Info("Rolling back deployment group")

	// Get all deployments in this group
	deployments, err := s.repos.Deployments.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	var rolledBack int
	var failedToRoll int
	var rollbackErrors []error

	// Rollback each deployment (in reverse order)
	for i := len(deployments) - 1; i >= 0; i-- {
		dep := deployments[i]
		_, err := s.deploymentService.Rollback(ctx, &RollbackRequest{
			DeploymentID: dep.ID.String(),
			UserID:       req.UserID,
			UserEmail:    req.UserEmail,
			UserRole:     req.UserRole,
		})
		if err != nil {
			failedToRoll++
			rollbackErrors = append(rollbackErrors, fmt.Errorf("failed to rollback deployment %s: %w", dep.ID, err))
			s.logger.Warn("Failed to rollback deployment", "deployment_id", dep.ID, "error", err)
		} else {
			rolledBack++
		}
	}

	// Update group status
	var finalStatus db.DeploymentGroupStatus
	var errMsg *string
	if failedToRoll > 0 {
		finalStatus = db.DeploymentGroupStatusFailed
		errMsgStr := fmt.Sprintf("Rollback partially failed: %d succeeded, %d failed", rolledBack, failedToRoll)
		errMsg = &errMsgStr
	} else {
		finalStatus = db.DeploymentGroupStatusRolledBack
	}

	if err := s.repos.DeploymentGroups.UpdateStatus(ctx, groupID, finalStatus, errMsg); err != nil {
		s.logger.Error("Failed to update group rollback status", "error", err)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorID:      nil,
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "deployment_group_rollback",
		ResourceType: "deployment_group",
		ResourceID:   group.ID.String(),
		ResourceName: *group.Name,
		Outcome:      string(finalStatus),
		Context: map[string]interface{}{
			"rolled_back":    rolledBack,
			"failed_to_roll": failedToRoll,
		},
	})

	// Refresh group
	group, _ = s.repos.DeploymentGroups.GetByID(ctx, groupID)

	return &RollbackGroupResponse{
		Group:        group,
		RolledBack:   rolledBack,
		FailedToRoll: failedToRoll,
		Errors:       rollbackErrors,
	}, nil
}

// GetGroupDeployment retrieves a deployment group by ID
func (s *DeploymentGroupService) GetGroupDeployment(ctx context.Context, groupID string) (*db.DeploymentGroup, error) {
	id, err := uuid.Parse(groupID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	group, err := s.repos.DeploymentGroups.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDeploymentNotFound)
	}

	return group, nil
}

// ListGroupDeployments lists deployment groups for a project
func (s *DeploymentGroupService) ListGroupDeployments(ctx context.Context, projectSlug string, limit, offset int) ([]*db.DeploymentGroup, error) {
	project, err := s.repos.Projects.GetBySlug(projectSlug)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrProjectNotFound)
	}

	groups, err := s.repos.DeploymentGroups.ListByProject(ctx, project.ID, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return groups, nil
}

// AddServiceDependencyRequest represents a request to add a service dependency
type AddServiceDependencyRequest struct {
	ServiceID          string
	DependsOnServiceID string
	DependencyType     string // "runtime", "build", "data"
	UserEmail          string
	UserRole           string
}

// AddServiceDependency adds a dependency between two services
func (s *DeploymentGroupService) AddServiceDependency(ctx context.Context, req *AddServiceDependencyRequest) (*db.ServiceDependency, error) {
	serviceID, err := uuid.Parse(req.ServiceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}
	dependsOnID, err := uuid.Parse(req.DependsOnServiceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	// Validate services exist
	if _, err := s.repos.Services.GetByID(serviceID); err != nil {
		return nil, errors.Wrap(err, errors.ErrServiceNotFound)
	}
	if _, err := s.repos.Services.GetByID(dependsOnID); err != nil {
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"field":  "depends_on_service_id",
			"reason": "Dependency service not found",
		})
	}

	// Check for cycles
	hasCycle, err := s.repos.ServiceDependencies.HasCycle(ctx, serviceID, dependsOnID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}
	if hasCycle {
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"reason": "Adding this dependency would create a circular dependency",
		})
	}

	// Parse dependency type
	depType := db.DependencyTypeRuntime
	switch req.DependencyType {
	case "build":
		depType = db.DependencyTypeBuild
	case "data":
		depType = db.DependencyTypeData
	case "runtime", "":
		depType = db.DependencyTypeRuntime
	default:
		return nil, errors.ErrValidation.WithDetails(map[string]any{
			"field":  "dependency_type",
			"reason": "Invalid type: must be runtime, build, or data",
		})
	}

	dep := &db.ServiceDependency{
		ServiceID:          serviceID,
		DependsOnServiceID: dependsOnID,
		DependencyType:     depType,
	}

	if err := s.repos.ServiceDependencies.Create(ctx, dep); err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorEmail:   req.UserEmail,
		ActorRole:    types.Role(req.UserRole),
		Action:       "service_dependency_added",
		ResourceType: "service_dependency",
		ResourceID:   dep.ID.String(),
		Outcome:      "success",
		Context: map[string]interface{}{
			"service_id":           req.ServiceID,
			"depends_on_service_id": req.DependsOnServiceID,
			"dependency_type":      string(depType),
		},
	})

	return dep, nil
}

// RemoveServiceDependency removes a dependency between two services
func (s *DeploymentGroupService) RemoveServiceDependency(ctx context.Context, serviceID, dependsOnID string, userEmail, userRole string) error {
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInvalidInput)
	}
	depID, err := uuid.Parse(dependsOnID)
	if err != nil {
		return errors.Wrap(err, errors.ErrInvalidInput)
	}

	if err := s.repos.ServiceDependencies.Delete(ctx, svcID, depID); err != nil {
		return errors.Wrap(err, errors.ErrDatabaseError)
	}

	// Audit log
	s.repos.AuditLogs.Log(ctx, &types.AuditLog{
		ActorEmail:   userEmail,
		ActorRole:    types.Role(userRole),
		Action:       "service_dependency_removed",
		ResourceType: "service_dependency",
		Outcome:      "success",
		Context: map[string]interface{}{
			"service_id":           serviceID,
			"depends_on_service_id": dependsOnID,
		},
	})

	return nil
}

// GetServiceDependencies returns all dependencies for a service
func (s *DeploymentGroupService) GetServiceDependencies(ctx context.Context, serviceID string) ([]*db.ServiceDependency, error) {
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	deps, err := s.repos.ServiceDependencies.GetByService(ctx, svcID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return deps, nil
}

// GetServiceDependents returns all services that depend on a given service
func (s *DeploymentGroupService) GetServiceDependents(ctx context.Context, serviceID string) ([]*db.ServiceDependency, error) {
	svcID, err := uuid.Parse(serviceID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrInvalidInput)
	}

	deps, err := s.repos.ServiceDependencies.GetDependents(ctx, svcID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrDatabaseError)
	}

	return deps, nil
}
