package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

type Repositories struct {
	Projects     *ProjectRepository
	Environments *EnvironmentRepository
	Services     *ServiceRepository
	Releases     *ReleaseRepository
	Deployments  *DeploymentRepository
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Projects:     NewProjectRepository(db),
		Environments: NewEnvironmentRepository(db),
		Services:     NewServiceRepository(db),
		Releases:     NewReleaseRepository(db),
		Deployments:  NewDeploymentRepository(db),
	}
}

// ProjectRepository handles project CRUD operations
type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(project *types.Project) error {
	project.ID = uuid.New()
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()

	query := `
		INSERT INTO projects (id, name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(query, project.ID, project.Name, project.Slug, project.CreatedAt, project.UpdatedAt)
	return err
}

func (r *ProjectRepository) GetBySlug(slug string) (*types.Project, error) {
	project := &types.Project{}
	query := `SELECT id, name, slug, created_at, updated_at FROM projects WHERE slug = $1`
	
	err := r.db.QueryRow(query, slug).Scan(
		&project.ID, &project.Name, &project.Slug,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return project, nil
}

func (r *ProjectRepository) List() ([]*types.Project, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM projects ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*types.Project
	for rows.Next() {
		project := &types.Project{}
		err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// ServiceRepository handles service CRUD operations
type ServiceRepository struct {
	db *sql.DB
}

func NewServiceRepository(db *sql.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) Create(service *types.Service) error {
	service.ID = uuid.New()
	service.CreatedAt = time.Now()
	service.UpdatedAt = time.Now()

	buildConfigJSON, err := json.Marshal(service.BuildConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal build config: %w", err)
	}

	query := `
		INSERT INTO services (id, project_id, name, git_repo, build_config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.Exec(query, service.ID, service.ProjectID, service.Name, service.GitRepo, buildConfigJSON, service.CreatedAt, service.UpdatedAt)
	return err
}

func (r *ServiceRepository) GetByID(id uuid.UUID) (*types.Service, error) {
	service := &types.Service{}
	var buildConfigJSON []byte

	query := `SELECT id, project_id, name, git_repo, build_config, created_at, updated_at FROM services WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
		&buildConfigJSON, &service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
	}
	
	return service, nil
}

func (r *ServiceRepository) ListByProject(projectID uuid.UUID) ([]*types.Service, error) {
	query := `SELECT id, project_id, name, git_repo, build_config, created_at, updated_at FROM services WHERE project_id = $1 ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*types.Service
	for rows.Next() {
		service := &types.Service{}
		var buildConfigJSON []byte
		
		err := rows.Scan(&service.ID, &service.ProjectID, &service.Name, &service.GitRepo, &buildConfigJSON, &service.CreatedAt, &service.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
		}

		services = append(services, service)
	}

	return services, nil
}

// Environment Repository
type EnvironmentRepository struct {
	db *sql.DB
}

func NewEnvironmentRepository(db *sql.DB) *EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

func (r *EnvironmentRepository) Create(env *types.Environment) error {
	env.ID = uuid.New()
	env.CreatedAt = time.Now()
	env.UpdatedAt = time.Now()

	query := `
		INSERT INTO environments (id, project_id, name, kube_namespace, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(query, env.ID, env.ProjectID, env.Name, env.KubeNamespace, env.CreatedAt, env.UpdatedAt)
	return err
}

func (r *EnvironmentRepository) GetByProjectAndName(projectID uuid.UUID, name string) (*types.Environment, error) {
	env := &types.Environment{}
	query := `SELECT id, project_id, name, kube_namespace, created_at, updated_at FROM environments WHERE project_id = $1 AND name = $2`
	
	err := r.db.QueryRow(query, projectID, name).Scan(
		&env.ID, &env.ProjectID, &env.Name, &env.KubeNamespace,
		&env.CreatedAt, &env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return env, nil
}

// Release Repository
type ReleaseRepository struct {
	db *sql.DB
}

func NewReleaseRepository(db *sql.DB) *ReleaseRepository {
	return &ReleaseRepository{db: db}
}

func (r *ReleaseRepository) Create(release *types.Release) error {
	release.ID = uuid.New()
	release.CreatedAt = time.Now()
	release.UpdatedAt = time.Now()

	query := `
		INSERT INTO releases (id, service_id, version, image_uri, git_sha, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(query, release.ID, release.ServiceID, release.Version, release.ImageURI, release.GitSHA, release.Status, release.CreatedAt, release.UpdatedAt)
	return err
}

func (r *ReleaseRepository) UpdateStatus(id uuid.UUID, status types.ReleaseStatus) error {
	query := `UPDATE releases SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(query, status, id)
	return err
}

func (r *ReleaseRepository) GetByID(id uuid.UUID) (*types.Release, error) {
	release := &types.Release{}
	query := `SELECT id, service_id, version, image_uri, git_sha, status, created_at, updated_at FROM releases WHERE id = $1`
	
	err := r.db.QueryRow(query, id).Scan(
		&release.ID, &release.ServiceID, &release.Version, &release.ImageURI,
		&release.GitSHA, &release.Status, &release.CreatedAt, &release.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return release, nil
}

func (r *ReleaseRepository) ListByService(serviceID uuid.UUID) ([]*types.Release, error) {
	query := `SELECT id, service_id, version, image_uri, git_sha, status, created_at, updated_at FROM releases WHERE service_id = $1 ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []*types.Release
	for rows.Next() {
		release := &types.Release{}
		err := rows.Scan(&release.ID, &release.ServiceID, &release.Version, &release.ImageURI, &release.GitSHA, &release.Status, &release.CreatedAt, &release.UpdatedAt)
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	}

	return releases, nil
}

// Deployment Repository
type DeploymentRepository struct {
	db *sql.DB
}

func NewDeploymentRepository(db *sql.DB) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) Create(deployment *types.Deployment) error {
	deployment.ID = uuid.New()
	deployment.CreatedAt = time.Now()
	deployment.UpdatedAt = time.Now()

	query := `
		INSERT INTO deployments (id, release_id, environment_id, replicas, status, health, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(query, deployment.ID, deployment.ReleaseID, deployment.EnvironmentID, deployment.Replicas, deployment.Status, deployment.Health, deployment.CreatedAt, deployment.UpdatedAt)
	return err
}

func (r *DeploymentRepository) UpdateStatus(id uuid.UUID, status types.DeploymentStatus, health types.HealthStatus) error {
	query := `UPDATE deployments SET status = $1, health = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.Exec(query, status, health, id)
	return err
}

func (r *DeploymentRepository) GetByID(ctx context.Context, id string) (*types.Deployment, error) {
	deployment := &types.Deployment{}
	query := `SELECT id, release_id, environment_id, replicas, status, health, created_at, updated_at
	          FROM deployments WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&deployment.ID, &deployment.ReleaseID, &deployment.EnvironmentID,
		&deployment.Replicas, &deployment.Status, &deployment.Health,
		&deployment.CreatedAt, &deployment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *DeploymentRepository) ListByRelease(ctx context.Context, releaseID string) ([]*types.Deployment, error) {
	query := `SELECT id, release_id, environment_id, replicas, status, health, created_at, updated_at
	          FROM deployments WHERE release_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*types.Deployment
	for rows.Next() {
		deployment := &types.Deployment{}
		err := rows.Scan(
			&deployment.ID, &deployment.ReleaseID, &deployment.EnvironmentID,
			&deployment.Replicas, &deployment.Status, &deployment.Health,
			&deployment.CreatedAt, &deployment.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepository) GetLatestByService(ctx context.Context, serviceID string) (*types.Deployment, error) {
	deployment := &types.Deployment{}
	query := `
		SELECT d.id, d.release_id, d.environment_id, d.replicas, d.status, d.health, d.created_at, d.updated_at
		FROM deployments d
		JOIN releases r ON d.release_id = r.id
		WHERE r.service_id = $1
		ORDER BY d.created_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, serviceID).Scan(
		&deployment.ID, &deployment.ReleaseID, &deployment.EnvironmentID,
		&deployment.Replicas, &deployment.Status, &deployment.Health,
		&deployment.CreatedAt, &deployment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}