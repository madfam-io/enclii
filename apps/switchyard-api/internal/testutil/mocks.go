package testutil

import (
	"context"
	"database/sql"
	"sync"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/errors"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// MockProjectRepository is a mock implementation of ProjectRepositoryInterface
type MockProjectRepository struct {
	mu        sync.RWMutex
	projects  map[uuid.UUID]*types.Project
	slugMap   map[string]*types.Project
	CreateFn  func(*types.Project) error
	GetByIDFn func(context.Context, uuid.UUID) (*types.Project, error)
}

func NewMockProjectRepository() *MockProjectRepository {
	return &MockProjectRepository{
		projects: make(map[uuid.UUID]*types.Project),
		slugMap:  make(map[string]*types.Project),
	}
}

func (m *MockProjectRepository) Create(project *types.Project) error {
	if m.CreateFn != nil {
		return m.CreateFn(project)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[project.ID] = project
	m.slugMap[project.Slug] = project
	return nil
}

func (m *MockProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*types.Project, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.projects[id]; ok {
		return p, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockProjectRepository) GetBySlug(slug string) (*types.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.slugMap[slug]; ok {
		return p, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockProjectRepository) List() ([]*types.Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Project, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result, nil
}

// MockServiceRepository is a mock implementation of ServiceRepositoryInterface
type MockServiceRepository struct {
	mu       sync.RWMutex
	services map[uuid.UUID]*types.Service
	CreateFn func(*types.Service) error
}

func NewMockServiceRepository() *MockServiceRepository {
	return &MockServiceRepository{
		services: make(map[uuid.UUID]*types.Service),
	}
}

func (m *MockServiceRepository) Create(service *types.Service) error {
	if m.CreateFn != nil {
		return m.CreateFn(service)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[service.ID] = service
	return nil
}

func (m *MockServiceRepository) GetByID(id uuid.UUID) (*types.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.services[id]; ok {
		return s, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockServiceRepository) ListAll(ctx context.Context) ([]*types.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Service, 0, len(m.services))
	for _, s := range m.services {
		result = append(result, s)
	}
	return result, nil
}

func (m *MockServiceRepository) ListByProject(projectID uuid.UUID) ([]*types.Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Service, 0)
	for _, s := range m.services {
		if s.ProjectID == projectID {
			result = append(result, s)
		}
	}
	return result, nil
}

// MockUserRepository is a mock implementation of UserRepositoryInterface
type MockUserRepository struct {
	mu       sync.RWMutex
	users    map[uuid.UUID]*types.User
	emailMap map[string]*types.User
	CreateFn func(*types.User) error
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:    make(map[uuid.UUID]*types.User),
		emailMap: make(map[string]*types.User),
	}
}

func (m *MockUserRepository) Create(user *types.User) error {
	if m.CreateFn != nil {
		return m.CreateFn(user)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	m.emailMap[user.Email] = user
	return nil
}

func (m *MockUserRepository) GetByID(id uuid.UUID) (*types.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockUserRepository) GetByEmail(email string) (*types.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if u, ok := m.emailMap[email]; ok {
		return u, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockUserRepository) Update(user *types.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	m.emailMap[user.Email] = user
	return nil
}

// MockReleaseRepository is a mock implementation of ReleaseRepositoryInterface
type MockReleaseRepository struct {
	mu       sync.RWMutex
	releases map[uuid.UUID]*types.Release
}

func NewMockReleaseRepository() *MockReleaseRepository {
	return &MockReleaseRepository{
		releases: make(map[uuid.UUID]*types.Release),
	}
}

func (m *MockReleaseRepository) Create(release *types.Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releases[release.ID] = release
	return nil
}

func (m *MockReleaseRepository) GetByID(id uuid.UUID) (*types.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if r, ok := m.releases[id]; ok {
		return r, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockReleaseRepository) UpdateStatus(id uuid.UUID, status types.ReleaseStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.releases[id]; ok {
		r.Status = status
		return nil
	}
	return errors.ErrNotFound
}

func (m *MockReleaseRepository) UpdateImageURI(id uuid.UUID, imageURI string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.releases[id]; ok {
		r.ImageURI = imageURI
		return nil
	}
	return errors.ErrNotFound
}

func (m *MockReleaseRepository) UpdateSBOM(ctx context.Context, id uuid.UUID, sbom, sbomFormat string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.releases[id]; ok {
		r.SBOM = sbom
		r.SBOMFormat = sbomFormat
		return nil
	}
	return errors.ErrNotFound
}

func (m *MockReleaseRepository) UpdateSignature(ctx context.Context, id uuid.UUID, signature string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.releases[id]; ok {
		r.ImageSignature = signature
		return nil
	}
	return errors.ErrNotFound
}

func (m *MockReleaseRepository) ListByService(serviceID uuid.UUID) ([]*types.Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Release, 0)
	for _, r := range m.releases {
		if r.ServiceID == serviceID {
			result = append(result, r)
		}
	}
	return result, nil
}

// MockDeploymentRepository is a mock implementation of DeploymentRepositoryInterface
type MockDeploymentRepository struct {
	mu          sync.RWMutex
	deployments map[string]*types.Deployment
}

func NewMockDeploymentRepository() *MockDeploymentRepository {
	return &MockDeploymentRepository{
		deployments: make(map[string]*types.Deployment),
	}
}

func (m *MockDeploymentRepository) Create(deployment *types.Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deployments[deployment.ID.String()] = deployment
	return nil
}

func (m *MockDeploymentRepository) GetByID(ctx context.Context, id string) (*types.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.deployments[id]; ok {
		return d, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockDeploymentRepository) UpdateStatus(id uuid.UUID, status types.DeploymentStatus, health types.HealthStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.deployments[id.String()]; ok {
		d.Status = status
		d.Health = health
		return nil
	}
	return errors.ErrNotFound
}

func (m *MockDeploymentRepository) ListByRelease(ctx context.Context, releaseID string) ([]*types.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Deployment, 0)
	for _, d := range m.deployments {
		if d.ReleaseID.String() == releaseID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (m *MockDeploymentRepository) GetLatestByService(ctx context.Context, serviceID string) (*types.Deployment, error) {
	return nil, errors.ErrNotFound
}

func (m *MockDeploymentRepository) GetByStatus(ctx context.Context, status types.DeploymentStatus) ([]*types.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Deployment, 0)
	for _, d := range m.deployments {
		if d.Status == status {
			result = append(result, d)
		}
	}
	return result, nil
}

// MockEnvironmentRepository is a mock implementation of EnvironmentRepositoryInterface
type MockEnvironmentRepository struct {
	mu           sync.RWMutex
	environments map[uuid.UUID]*types.Environment
}

func NewMockEnvironmentRepository() *MockEnvironmentRepository {
	return &MockEnvironmentRepository{
		environments: make(map[uuid.UUID]*types.Environment),
	}
}

func (m *MockEnvironmentRepository) Create(env *types.Environment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.environments[env.ID] = env
	return nil
}

func (m *MockEnvironmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*types.Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if e, ok := m.environments[id]; ok {
		return e, nil
	}
	return nil, errors.ErrNotFound
}

func (m *MockEnvironmentRepository) ListByProject(projectID uuid.UUID) ([]*types.Environment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*types.Environment, 0)
	for _, e := range m.environments {
		if e.ProjectID == projectID {
			result = append(result, e)
		}
	}
	return result, nil
}

// MockProjectAccessRepository is a mock implementation of ProjectAccessRepositoryInterface
type MockProjectAccessRepository struct {
	access map[string]bool
}

func NewMockProjectAccessRepository() *MockProjectAccessRepository {
	return &MockProjectAccessRepository{
		access: make(map[string]bool),
	}
}

func (m *MockProjectAccessRepository) Grant(access *types.ProjectAccess) error {
	return nil
}

func (m *MockProjectAccessRepository) Revoke(ctx context.Context, userID, projectID uuid.UUID, environmentID *uuid.UUID) error {
	return nil
}

func (m *MockProjectAccessRepository) GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) ([]*types.ProjectAccess, error) {
	return nil, nil
}

func (m *MockProjectAccessRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*types.ProjectAccess, error) {
	return nil, nil
}

func (m *MockProjectAccessRepository) HasAccess(ctx context.Context, userID, projectID uuid.UUID, environmentID *uuid.UUID, requiredRole types.Role) (bool, error) {
	return true, nil // Default to allow for testing
}

// MockRepositories is a struct that holds mock repository implementations
type MockRepositories struct {
	Projects      *MockProjectRepository
	Services      *MockServiceRepository
	Users         *MockUserRepository
	Releases      *MockReleaseRepository
	Deployments   *MockDeploymentRepository
	Environments  *MockEnvironmentRepository
	ProjectAccess *MockProjectAccessRepository
}

// NewMockRepositories creates a new set of mock repositories for testing
func NewMockRepositories() *MockRepositories {
	return &MockRepositories{
		Projects:      NewMockProjectRepository(),
		Services:      NewMockServiceRepository(),
		Users:         NewMockUserRepository(),
		Releases:      NewMockReleaseRepository(),
		Deployments:   NewMockDeploymentRepository(),
		Environments:  NewMockEnvironmentRepository(),
		ProjectAccess: NewMockProjectAccessRepository(),
	}
}

// ErrNotFound is a sentinel error for not found
var ErrNotFound = sql.ErrNoRows
