package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

type Repositories struct {
	db                  *sql.DB // Keep reference for transaction support
	Projects            *ProjectRepository
	Environments        *EnvironmentRepository
	Services            *ServiceRepository
	Releases            *ReleaseRepository
	Deployments         *DeploymentRepository
	Users               *UserRepository
	ProjectAccess       *ProjectAccessRepository
	AuditLogs           *AuditLogRepository
	ApprovalRecords     *ApprovalRecordRepository
	RotationAuditLogs   *RotationAuditLogRepository
	CustomDomains       *CustomDomainRepository
	Routes              *RouteRepository
	DeploymentGroups    *DeploymentGroupRepository
	ServiceDependencies *ServiceDependencyRepository
	EnvVars             *EnvVarRepository
	PreviewEnvironments *PreviewEnvironmentRepository
	PreviewComments     *PreviewCommentRepository
	PreviewAccessLogs   *PreviewAccessLogRepository
	Teams               *TeamRepository
	TeamMembers         *TeamMemberRepository
	TeamInvitations     *TeamInvitationRepository
	APITokens           *APITokenRepository
	DatabaseAddons      *DatabaseAddonRepository
}

// WithTransaction executes the given function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func (r *Repositories) WithTransaction(ctx context.Context, fn func(txRepos *Repositories) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create transaction-scoped repositories
	txRepos := &Repositories{
		db:                  r.db, // Keep original db for nested transaction prevention
		Projects:            &ProjectRepository{db: tx},
		Environments:        &EnvironmentRepository{db: tx},
		Services:            &ServiceRepository{db: tx},
		Releases:            &ReleaseRepository{db: tx},
		Deployments:         &DeploymentRepository{db: tx},
		Users:               &UserRepository{db: tx},
		ProjectAccess:       &ProjectAccessRepository{db: tx},
		AuditLogs:           &AuditLogRepository{db: tx},
		ApprovalRecords:     &ApprovalRecordRepository{db: tx},
		RotationAuditLogs:   &RotationAuditLogRepository{db: tx},
		CustomDomains:       NewCustomDomainRepositoryWithTx(tx),
		Routes:              NewRouteRepositoryWithTx(tx),
		DeploymentGroups:    NewDeploymentGroupRepositoryWithTx(tx),
		ServiceDependencies: NewServiceDependencyRepositoryWithTx(tx),
		EnvVars:             NewEnvVarRepositoryWithTx(tx),
		PreviewEnvironments: NewPreviewEnvironmentRepositoryWithTx(tx),
		PreviewComments:     NewPreviewCommentRepositoryWithTx(tx),
		PreviewAccessLogs:   NewPreviewAccessLogRepositoryWithTx(tx),
		Teams:               NewTeamRepositoryWithTx(tx),
		TeamMembers:         NewTeamMemberRepositoryWithTx(tx),
		TeamInvitations:     NewTeamInvitationRepositoryWithTx(tx),
		APITokens:           NewAPITokenRepositoryWithTx(tx),
		DatabaseAddons:      NewDatabaseAddonRepositoryWithTx(tx),
	}

	// Execute the function with transaction repositories
	if err := fn(txRepos); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		db:                  db,
		Projects:            NewProjectRepository(db),
		Environments:        NewEnvironmentRepository(db),
		Services:            NewServiceRepository(db),
		Releases:            NewReleaseRepository(db),
		Deployments:         NewDeploymentRepository(db),
		Users:               NewUserRepository(db),
		ProjectAccess:       NewProjectAccessRepository(db),
		AuditLogs:           NewAuditLogRepository(db),
		ApprovalRecords:     NewApprovalRecordRepository(db),
		RotationAuditLogs:   NewRotationAuditLogRepository(db),
		CustomDomains:       NewCustomDomainRepository(db),
		Routes:              NewRouteRepository(db),
		DeploymentGroups:    NewDeploymentGroupRepository(db),
		ServiceDependencies: NewServiceDependencyRepository(db),
		EnvVars:             NewEnvVarRepository(db),
		PreviewEnvironments: NewPreviewEnvironmentRepository(db),
		PreviewComments:     NewPreviewCommentRepository(db),
		PreviewAccessLogs:   NewPreviewAccessLogRepository(db),
		Teams:               NewTeamRepository(db),
		TeamMembers:         NewTeamMemberRepository(db),
		TeamInvitations:     NewTeamInvitationRepository(db),
		APITokens:           NewAPITokenRepository(db),
		DatabaseAddons:      NewDatabaseAddonRepository(db),
	}
}

// ProjectRepository handles project CRUD operations
type ProjectRepository struct {
	db DBTX
}

func NewProjectRepository(db DBTX) *ProjectRepository {
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

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*types.Project, error) {
	project := &types.Project{}
	query := `SELECT id, name, slug, created_at, updated_at FROM projects WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&project.ID, &project.Name, &project.Slug,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return project, nil
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

// Delete removes a project by ID
// Note: All related records (services, environments, etc.) are automatically
// deleted via ON DELETE CASCADE foreign key constraints
func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM projects WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ServiceRepository handles service CRUD operations
type ServiceRepository struct {
	db DBTX
}

func NewServiceRepository(db DBTX) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) Create(service *types.Service) error {
	service.ID = uuid.New()
	service.CreatedAt = time.Now()
	service.UpdatedAt = time.Now()

	// Set sensible defaults for auto-deploy if not provided
	if service.AutoDeployBranch == "" {
		service.AutoDeployBranch = "main"
	}
	if service.AutoDeployEnv == "" {
		service.AutoDeployEnv = "production"
	}

	buildConfigJSON, err := json.Marshal(service.BuildConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal build config: %w", err)
	}

	query := `
		INSERT INTO services (id, project_id, name, git_repo, app_path, build_config,
			auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.db.Exec(query, service.ID, service.ProjectID, service.Name, service.GitRepo,
		service.AppPath, buildConfigJSON, service.AutoDeploy, service.AutoDeployBranch,
		service.AutoDeployEnv, service.CreatedAt, service.UpdatedAt)
	return err
}

func (r *ServiceRepository) GetByID(id uuid.UUID) (*types.Service, error) {
	service := &types.Service{}
	var buildConfigJSON []byte
	var appPath sql.NullString

	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services WHERE id = $1`

	err := r.db.QueryRow(query, id).Scan(
		&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
		&appPath, &buildConfigJSON, &service.AutoDeploy, &service.AutoDeployBranch,
		&service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if appPath.Valid {
		service.AppPath = appPath.String
	}

	if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
	}

	return service, nil
}

// GetByName retrieves a service by its name (used for K8s→DB reconciliation)
func (r *ServiceRepository) GetByName(name string) (*types.Service, error) {
	service := &types.Service{}
	var buildConfigJSON []byte
	var appPath sql.NullString

	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services WHERE name = $1`

	err := r.db.QueryRow(query, name).Scan(
		&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
		&appPath, &buildConfigJSON, &service.AutoDeploy, &service.AutoDeployBranch,
		&service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if appPath.Valid {
		service.AppPath = appPath.String
	}

	if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
	}

	return service, nil
}

func (r *ServiceRepository) ListAll(ctx context.Context) ([]*types.Service, error) {
	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*types.Service
	for rows.Next() {
		service := &types.Service{}
		var buildConfigJSON []byte
		var appPath sql.NullString

		err := rows.Scan(
			&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
			&appPath, &buildConfigJSON, &service.AutoDeploy, &service.AutoDeployBranch,
			&service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if appPath.Valid {
			service.AppPath = appPath.String
		}

		if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
		}

		services = append(services, service)
	}

	return services, nil
}

func (r *ServiceRepository) ListByProject(projectID uuid.UUID) ([]*types.Service, error) {
	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services WHERE project_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*types.Service
	for rows.Next() {
		service := &types.Service{}
		var buildConfigJSON []byte
		var appPath sql.NullString

		err := rows.Scan(&service.ID, &service.ProjectID, &service.Name, &service.GitRepo, &appPath, &buildConfigJSON,
			&service.AutoDeploy, &service.AutoDeployBranch, &service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if appPath.Valid {
			service.AppPath = appPath.String
		}

		if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
		}

		services = append(services, service)
	}

	return services, nil
}

// GetByGitRepo retrieves a service by its git repository URL
// Used by GitHub webhooks to find the service to build when a push event is received
func (r *ServiceRepository) GetByGitRepo(gitRepoURL string) (*types.Service, error) {
	service := &types.Service{}
	var buildConfigJSON []byte
	var appPath sql.NullString

	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services WHERE git_repo = $1`

	err := r.db.QueryRow(query, gitRepoURL).Scan(
		&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
		&appPath, &buildConfigJSON, &service.AutoDeploy, &service.AutoDeployBranch,
		&service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if appPath.Valid {
		service.AppPath = appPath.String
	}

	if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
	}

	return service, nil
}

// ListByGitRepo retrieves ALL services matching a git repository URL
// Supports monorepos where multiple services share the same repo
// Normalizes URLs to handle variations like .git suffix, trailing slashes
func (r *ServiceRepository) ListByGitRepo(gitRepoURL string) ([]*types.Service, error) {
	// Normalize the input URL for matching
	normalizedURL := normalizeGitURL(gitRepoURL)

	// Query with normalized URL matching (handles .git suffix variations)
	query := `SELECT id, project_id, name, git_repo, COALESCE(app_path, '') as app_path, build_config,
		auto_deploy, auto_deploy_branch, auto_deploy_env, created_at, updated_at
		FROM services
		WHERE REPLACE(REPLACE(git_repo, '.git', ''), 'https://github.com/', '') = $1
		   OR git_repo = $2
		   OR git_repo = $3`

	// Try with and without .git suffix
	urlWithGit := normalizedURL
	if !strings.HasSuffix(normalizedURL, ".git") {
		urlWithGit = normalizedURL + ".git"
	}
	urlWithoutGit := strings.TrimSuffix(normalizedURL, ".git")

	rows, err := r.db.Query(query,
		strings.TrimPrefix(strings.TrimPrefix(urlWithoutGit, "https://github.com/"), "http://github.com/"),
		urlWithGit,
		urlWithoutGit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*types.Service
	for rows.Next() {
		service := &types.Service{}
		var buildConfigJSON []byte
		var appPath sql.NullString

		if err := rows.Scan(
			&service.ID, &service.ProjectID, &service.Name, &service.GitRepo,
			&appPath, &buildConfigJSON, &service.AutoDeploy, &service.AutoDeployBranch,
			&service.AutoDeployEnv, &service.CreatedAt, &service.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if appPath.Valid {
			service.AppPath = appPath.String
		}

		if err := json.Unmarshal(buildConfigJSON, &service.BuildConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal build config: %w", err)
		}

		services = append(services, service)
	}

	return services, nil
}

// normalizeGitURL normalizes a git repository URL for consistent matching
func normalizeGitURL(url string) string {
	// Remove trailing slashes
	url = strings.TrimSuffix(url, "/")
	// Ensure https:// prefix
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}
	return url
}

// Update updates an existing service
func (r *ServiceRepository) Update(ctx context.Context, service *types.Service) error {
	service.UpdatedAt = time.Now()

	buildConfigJSON, err := json.Marshal(service.BuildConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal build config: %w", err)
	}

	query := `
		UPDATE services
		SET name = $1, git_repo = $2, app_path = $3, build_config = $4,
		    auto_deploy = $5, auto_deploy_branch = $6, auto_deploy_env = $7, updated_at = $8
		WHERE id = $9
	`
	result, err := r.db.ExecContext(ctx, query,
		service.Name, service.GitRepo, service.AppPath, buildConfigJSON,
		service.AutoDeploy, service.AutoDeployBranch, service.AutoDeployEnv, service.UpdatedAt, service.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Delete removes a service by ID
func (r *ServiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM services WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Environment Repository
type EnvironmentRepository struct {
	db DBTX
}

func NewEnvironmentRepository(db DBTX) *EnvironmentRepository {
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

func (r *EnvironmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*types.Environment, error) {
	env := &types.Environment{}
	query := `SELECT id, project_id, name, kube_namespace, created_at, updated_at FROM environments WHERE id = $1`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&env.ID, &env.ProjectID, &env.Name, &env.KubeNamespace,
		&env.CreatedAt, &env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func (r *EnvironmentRepository) ListByProject(projectID uuid.UUID) ([]*types.Environment, error) {
	query := `SELECT id, project_id, name, kube_namespace, created_at, updated_at FROM environments WHERE project_id = $1 ORDER BY name`

	rows, err := r.db.Query(query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []*types.Environment
	for rows.Next() {
		env := &types.Environment{}
		if err := rows.Scan(&env.ID, &env.ProjectID, &env.Name, &env.KubeNamespace, &env.CreatedAt, &env.UpdatedAt); err != nil {
			return nil, err
		}
		environments = append(environments, env)
	}

	return environments, nil
}

// ListAll retrieves all environments across all projects
// Used by the reconciler to build dynamic namespace list for K8s sync
func (r *EnvironmentRepository) ListAll() ([]*types.Environment, error) {
	query := `SELECT id, project_id, name, kube_namespace, created_at, updated_at FROM environments ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []*types.Environment
	for rows.Next() {
		env := &types.Environment{}
		if err := rows.Scan(&env.ID, &env.ProjectID, &env.Name, &env.KubeNamespace, &env.CreatedAt, &env.UpdatedAt); err != nil {
			return nil, err
		}
		environments = append(environments, env)
	}

	return environments, nil
}

// GetByKubeNamespace retrieves an environment by its Kubernetes namespace (used for K8s→DB reconciliation)
func (r *EnvironmentRepository) GetByKubeNamespace(namespace string) (*types.Environment, error) {
	env := &types.Environment{}
	query := `SELECT id, project_id, name, kube_namespace, created_at, updated_at FROM environments WHERE kube_namespace = $1`

	err := r.db.QueryRow(query, namespace).Scan(
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
	db DBTX
}

func NewReleaseRepository(db DBTX) *ReleaseRepository {
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

func (r *ReleaseRepository) UpdateImageURI(id uuid.UUID, imageURI string) error {
	query := `UPDATE releases SET image_uri = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(query, imageURI, id)
	return err
}

func (r *ReleaseRepository) UpdateSBOM(ctx context.Context, id uuid.UUID, sbom, sbomFormat string) error {
	query := `UPDATE releases SET sbom = $1, sbom_format = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, sbom, sbomFormat, id)
	return err
}

func (r *ReleaseRepository) UpdateSignature(ctx context.Context, id uuid.UUID, signature string) error {
	query := `UPDATE releases SET image_signature = $1, signature_verified_at = NOW(), updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, signature, id)
	return err
}

func (r *ReleaseRepository) GetByID(id uuid.UUID) (*types.Release, error) {
	release := &types.Release{}
	query := `SELECT id, service_id, version, image_uri, git_sha, status, sbom, sbom_format, image_signature, signature_verified_at, created_at, updated_at FROM releases WHERE id = $1`

	var sbom, sbomFormat, imageSignature sql.NullString
	var signatureVerifiedAt sql.NullTime
	err := r.db.QueryRow(query, id).Scan(
		&release.ID, &release.ServiceID, &release.Version, &release.ImageURI,
		&release.GitSHA, &release.Status, &sbom, &sbomFormat, &imageSignature, &signatureVerifiedAt, &release.CreatedAt, &release.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable SBOM fields
	if sbom.Valid {
		release.SBOM = sbom.String
	}
	if sbomFormat.Valid {
		release.SBOMFormat = sbomFormat.String
	}

	// Handle nullable signature fields
	if imageSignature.Valid {
		release.ImageSignature = imageSignature.String
	}
	if signatureVerifiedAt.Valid {
		release.SignatureVerifiedAt = &signatureVerifiedAt.Time
	}

	return release, nil
}

func (r *ReleaseRepository) ListByService(serviceID uuid.UUID) ([]*types.Release, error) {
	query := `SELECT id, service_id, version, image_uri, git_sha, status, sbom, sbom_format, image_signature, signature_verified_at, created_at, updated_at FROM releases WHERE service_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(query, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var releases []*types.Release
	for rows.Next() {
		release := &types.Release{}
		var sbom, sbomFormat, imageSignature sql.NullString
		var signatureVerifiedAt sql.NullTime

		err := rows.Scan(&release.ID, &release.ServiceID, &release.Version, &release.ImageURI, &release.GitSHA, &release.Status, &sbom, &sbomFormat, &imageSignature, &signatureVerifiedAt, &release.CreatedAt, &release.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Handle nullable SBOM fields
		if sbom.Valid {
			release.SBOM = sbom.String
		}
		if sbomFormat.Valid {
			release.SBOMFormat = sbomFormat.String
		}

		// Handle nullable signature fields
		if imageSignature.Valid {
			release.ImageSignature = imageSignature.String
		}
		if signatureVerifiedAt.Valid {
			release.SignatureVerifiedAt = &signatureVerifiedAt.Time
		}

		releases = append(releases, release)
	}

	return releases, nil
}

// Deployment Repository
type DeploymentRepository struct {
	db DBTX
}

func NewDeploymentRepository(db DBTX) *DeploymentRepository {
	return &DeploymentRepository{db: db}
}

func (r *DeploymentRepository) Create(deployment *types.Deployment) error {
	deployment.ID = uuid.New()
	deployment.CreatedAt = time.Now()
	deployment.UpdatedAt = time.Now()

	// Note: group_id and deploy_order columns don't exist in the database yet
	// They're part of the deployment group feature that hasn't been migrated
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

func (r *DeploymentRepository) GetByStatus(ctx context.Context, status types.DeploymentStatus) ([]*types.Deployment, error) {
	// Note: group_id and deploy_order columns don't exist in the database yet
	// They're part of the deployment group feature that hasn't been migrated
	query := `SELECT id, release_id, environment_id, replicas, status, health, created_at, updated_at
	          FROM deployments WHERE status = $1 ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, status)
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
		// GroupID and DeployOrder default to nil/0 until feature is migrated
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

// ListByGroup retrieves all deployments for a deployment group
// Note: This feature is not yet implemented - group_id and deploy_order columns
// don't exist in the database. Returns empty slice until feature is migrated.
func (r *DeploymentRepository) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]*types.Deployment, error) {
	// Deployment groups feature not yet migrated - return empty slice
	return []*types.Deployment{}, nil
}

// UserRepository handles user CRUD operations
type UserRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *types.User) error {
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	query := `
		INSERT INTO users (id, email, password_hash, name, role, oidc_subject, oidc_issuer, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role,
		user.OIDCSubject, user.OIDCIssuer, user.Active, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*types.User, error) {
	user := &types.User{}
	query := `
		SELECT id, email, password_hash, name, role, oidc_subject, oidc_issuer, active, created_at, updated_at, last_login_at
		FROM users WHERE email = $1
	`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
		&user.OIDCSubject, &user.OIDCIssuer, &user.Active, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetByOIDCIdentity retrieves a user by their OIDC issuer and subject
func (r *UserRepository) GetByOIDCIdentity(ctx context.Context, issuer string, subject string) (*types.User, error) {
	user := &types.User{}
	query := `
		SELECT id, email, password_hash, name, role, oidc_subject, oidc_issuer, active, created_at, updated_at, last_login_at
		FROM users WHERE oidc_issuer = $1 AND oidc_subject = $2
	`

	err := r.db.QueryRowContext(ctx, query, issuer, subject).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
		&user.OIDCSubject, &user.OIDCIssuer, &user.Active, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*types.User, error) {
	user := &types.User{}
	query := `
		SELECT id, email, password_hash, name, role, oidc_subject, oidc_issuer, active, created_at, updated_at, last_login_at
		FROM users WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
		&user.OIDCSubject, &user.OIDCIssuer, &user.Active, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()

	query := `
		UPDATE users
		SET email = $1, password_hash = $2, name = $3, role = $4, oidc_subject = $5, oidc_issuer = $6, active = $7, updated_at = $8, last_login_at = $9
		WHERE id = $10
	`
	_, err := r.db.ExecContext(ctx, query,
		user.Email, user.PasswordHash, user.Name, user.Role,
		user.OIDCSubject, user.OIDCIssuer, user.Active, user.UpdatedAt, user.LastLoginAt, user.ID,
	)
	return err
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *UserRepository) List(ctx context.Context) ([]*types.User, error) {
	query := `
		SELECT id, email, password_hash, name, role, oidc_subject, oidc_issuer, active, created_at, updated_at, last_login_at
		FROM users ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*types.User
	for rows.Next() {
		user := &types.User{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
			&user.OIDCSubject, &user.OIDCIssuer, &user.Active, &user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// ProjectAccessRepository handles project access control
type ProjectAccessRepository struct {
	db DBTX
}

func NewProjectAccessRepository(db DBTX) *ProjectAccessRepository {
	return &ProjectAccessRepository{db: db}
}

func (r *ProjectAccessRepository) Grant(ctx context.Context, access *types.ProjectAccess) error {
	access.ID = uuid.New()
	access.GrantedAt = time.Now()

	query := `
		INSERT INTO project_access (id, user_id, project_id, environment_id, role, granted_by, granted_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, project_id, environment_id) DO UPDATE
		SET role = EXCLUDED.role, granted_by = EXCLUDED.granted_by, granted_at = EXCLUDED.granted_at, expires_at = EXCLUDED.expires_at
	`
	_, err := r.db.ExecContext(ctx, query,
		access.ID, access.UserID, access.ProjectID, access.EnvironmentID,
		access.Role, access.GrantedBy, access.GrantedAt, access.ExpiresAt,
	)
	return err
}

func (r *ProjectAccessRepository) Revoke(ctx context.Context, userID, projectID uuid.UUID, environmentID *uuid.UUID) error {
	query := `
		DELETE FROM project_access
		WHERE user_id = $1 AND project_id = $2 AND (environment_id = $3 OR ($3 IS NULL AND environment_id IS NULL))
	`
	_, err := r.db.ExecContext(ctx, query, userID, projectID, environmentID)
	return err
}

func (r *ProjectAccessRepository) UserHasAccess(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM project_access
		WHERE user_id = $1 AND project_id = $2
		AND (expires_at IS NULL OR expires_at > NOW())
	`
	err := r.db.QueryRowContext(ctx, query, userID, projectID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasAccess checks if a user has access to a project/environment with the required role
func (r *ProjectAccessRepository) HasAccess(ctx context.Context, userID, projectID uuid.UUID, environmentID *uuid.UUID, requiredRole types.Role) (bool, error) {
	userRole, err := r.GetUserRole(ctx, userID, projectID, environmentID)
	if err != nil {
		// If no access record found, return false
		return false, nil
	}

	// Role hierarchy: admin > developer > viewer
	roleLevel := map[types.Role]int{
		types.RoleAdmin:     3,
		types.RoleDeveloper: 2,
		types.RoleViewer:    1,
	}

	return roleLevel[userRole] >= roleLevel[requiredRole], nil
}

func (r *ProjectAccessRepository) GetUserRole(ctx context.Context, userID, projectID uuid.UUID, environmentID *uuid.UUID) (types.Role, error) {
	var role types.Role
	query := `
		SELECT role FROM project_access
		WHERE user_id = $1 AND project_id = $2
		AND (environment_id = $3 OR environment_id IS NULL)
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY environment_id NULLS LAST
		LIMIT 1
	`
	err := r.db.QueryRowContext(ctx, query, userID, projectID, environmentID).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func (r *ProjectAccessRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*types.ProjectAccess, error) {
	query := `
		SELECT id, user_id, project_id, environment_id, role, granted_by, granted_at, expires_at
		FROM project_access
		WHERE user_id = $1 AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accesses []*types.ProjectAccess
	for rows.Next() {
		access := &types.ProjectAccess{}
		err := rows.Scan(
			&access.ID, &access.UserID, &access.ProjectID, &access.EnvironmentID,
			&access.Role, &access.GrantedBy, &access.GrantedAt, &access.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		accesses = append(accesses, access)
	}

	return accesses, nil
}

func (r *ProjectAccessRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*types.ProjectAccess, error) {
	query := `
		SELECT id, user_id, project_id, environment_id, role, granted_by, granted_at, expires_at
		FROM project_access
		WHERE project_id = $1 AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accesses []*types.ProjectAccess
	for rows.Next() {
		access := &types.ProjectAccess{}
		err := rows.Scan(
			&access.ID, &access.UserID, &access.ProjectID, &access.EnvironmentID,
			&access.Role, &access.GrantedBy, &access.GrantedAt, &access.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		accesses = append(accesses, access)
	}

	return accesses, nil
}

// AuditLogRepository handles audit log operations (immutable)
type AuditLogRepository struct {
	db DBTX
}

func NewAuditLogRepository(db DBTX) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Log(ctx context.Context, log *types.AuditLog) error {
	log.ID = uuid.New()
	log.Timestamp = time.Now()

	contextJSON, err := json.Marshal(log.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit_logs (
			id, timestamp, actor_id, actor_email, actor_role, action,
			resource_type, resource_id, resource_name,
			project_id, environment_id, ip_address, user_agent, outcome, context, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err = r.db.ExecContext(ctx, query,
		log.ID, log.Timestamp, log.ActorID, log.ActorEmail, log.ActorRole, log.Action,
		log.ResourceType, log.ResourceID, log.ResourceName,
		log.ProjectID, log.EnvironmentID, log.IPAddress, log.UserAgent, log.Outcome,
		contextJSON, metadataJSON,
	)
	return err
}

func (r *AuditLogRepository) Query(ctx context.Context, filters map[string]interface{}, limit int, offset int) ([]*types.AuditLog, error) {
	query := `
		SELECT id, timestamp, actor_id, actor_email, actor_role, action,
		       resource_type, resource_id, resource_name,
		       project_id, environment_id, ip_address, user_agent, outcome, context, metadata
		FROM audit_logs
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	// Add filters dynamically
	if actorID, ok := filters["actor_id"].(uuid.UUID); ok {
		query += fmt.Sprintf(" AND actor_id = $%d", argCount)
		args = append(args, actorID)
		argCount++
	}
	if action, ok := filters["action"].(string); ok {
		query += fmt.Sprintf(" AND action = $%d", argCount)
		args = append(args, action)
		argCount++
	}
	if resourceType, ok := filters["resource_type"].(string); ok {
		query += fmt.Sprintf(" AND resource_type = $%d", argCount)
		args = append(args, resourceType)
		argCount++
	}
	if projectID, ok := filters["project_id"].(uuid.UUID); ok {
		query += fmt.Sprintf(" AND project_id = $%d", argCount)
		args = append(args, projectID)
		argCount++
	}

	query += " ORDER BY timestamp DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*types.AuditLog
	for rows.Next() {
		log := &types.AuditLog{}
		var contextJSON, metadataJSON []byte

		err := rows.Scan(
			&log.ID, &log.Timestamp, &log.ActorID, &log.ActorEmail, &log.ActorRole, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.ResourceName,
			&log.ProjectID, &log.EnvironmentID, &log.IPAddress, &log.UserAgent, &log.Outcome,
			&contextJSON, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(contextJSON, &log.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// ApprovalRecordRepository handles approval record operations
type ApprovalRecordRepository struct {
	db DBTX
}

func NewApprovalRecordRepository(db DBTX) *ApprovalRecordRepository {
	return &ApprovalRecordRepository{db: db}
}

// Create inserts a new approval record
func (r *ApprovalRecordRepository) Create(ctx context.Context, record *types.ApprovalRecord) error {
	record.ID = uuid.New()
	record.CreatedAt = time.Now()

	query := `
		INSERT INTO approval_records (
			id, deployment_id, pr_url, pr_number, approver_email, approver_name,
			approved_at, ci_status, change_ticket_url, compliance_receipt, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		record.ID,
		record.DeploymentID,
		record.PRURL,
		record.PRNumber,
		record.ApproverEmail,
		record.ApproverName,
		record.ApprovedAt,
		record.CIStatus,
		record.ChangeTicketURL,
		record.ComplianceReceipt,
		record.CreatedAt,
	)

	return err
}

// GetByDeploymentID retrieves the approval record for a deployment
func (r *ApprovalRecordRepository) GetByDeploymentID(ctx context.Context, deploymentID uuid.UUID) (*types.ApprovalRecord, error) {
	record := &types.ApprovalRecord{}

	query := `
		SELECT id, deployment_id, pr_url, pr_number, approver_email, approver_name,
		       approved_at, ci_status, change_ticket_url, compliance_receipt, created_at
		FROM approval_records
		WHERE deployment_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, deploymentID).Scan(
		&record.ID,
		&record.DeploymentID,
		&record.PRURL,
		&record.PRNumber,
		&record.ApproverEmail,
		&record.ApproverName,
		&record.ApprovedAt,
		&record.CIStatus,
		&record.ChangeTicketURL,
		&record.ComplianceReceipt,
		&record.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No approval record found (OK for dev deployments)
	}

	if err != nil {
		return nil, err
	}

	return record, nil
}

// List retrieves approval records with optional filtering
func (r *ApprovalRecordRepository) List(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*types.ApprovalRecord, error) {
	query := `
		SELECT id, deployment_id, pr_url, pr_number, approver_email, approver_name,
		       approved_at, ci_status, change_ticket_url, compliance_receipt, created_at
		FROM approval_records
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	// Add filters dynamically
	if deploymentID, ok := filters["deployment_id"].(uuid.UUID); ok {
		query += fmt.Sprintf(" AND deployment_id = $%d", argCount)
		args = append(args, deploymentID)
		argCount++
	}

	if approverEmail, ok := filters["approver_email"].(string); ok {
		query += fmt.Sprintf(" AND approver_email = $%d", argCount)
		args = append(args, approverEmail)
		argCount++
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*types.ApprovalRecord
	for rows.Next() {
		record := &types.ApprovalRecord{}

		err := rows.Scan(
			&record.ID,
			&record.DeploymentID,
			&record.PRURL,
			&record.PRNumber,
			&record.ApproverEmail,
			&record.ApproverName,
			&record.ApprovedAt,
			&record.CIStatus,
			&record.ChangeTicketURL,
			&record.ComplianceReceipt,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

// RotationAuditLogRepository handles rotation audit log operations
type RotationAuditLogRepository struct {
	db DBTX
}

func NewRotationAuditLogRepository(db DBTX) *RotationAuditLogRepository {
	return &RotationAuditLogRepository{db: db}
}

// Create inserts a new rotation audit log
func (r *RotationAuditLogRepository) Create(ctx context.Context, log interface{}) error {
	// Import lockbox package types
	// We accept interface{} to avoid circular dependency but cast to proper type
	type rotationLog struct {
		ID              uuid.UUID
		EventID         uuid.UUID
		ServiceID       string
		ServiceName     string
		Environment     string
		SecretName      string
		SecretPath      string
		OldVersion      int
		NewVersion      int
		Status          string
		StartedAt       time.Time
		CompletedAt     *time.Time
		Duration        time.Duration
		RolloutStrategy string
		PodsRestarted   int
		Error           string
		ChangedBy       string
		TriggeredBy     string
	}

	// Type assertion
	auditLog, ok := log.(*rotationLog)
	if !ok {
		return fmt.Errorf("invalid log type")
	}

	// Convert duration to milliseconds for database storage
	var durationMs *int64
	if auditLog.Duration > 0 {
		ms := auditLog.Duration.Milliseconds()
		durationMs = &ms
	}

	query := `
		INSERT INTO rotation_audit_logs (
			id, event_id, service_id, service_name, environment,
			secret_name, secret_path, old_version, new_version, status,
			started_at, completed_at, duration_ms, rollout_strategy,
			pods_restarted, error, changed_by, triggered_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err := r.db.ExecContext(ctx, query,
		auditLog.ID,
		auditLog.EventID,
		auditLog.ServiceID,
		auditLog.ServiceName,
		auditLog.Environment,
		auditLog.SecretName,
		auditLog.SecretPath,
		auditLog.OldVersion,
		auditLog.NewVersion,
		auditLog.Status,
		auditLog.StartedAt,
		auditLog.CompletedAt,
		durationMs,
		auditLog.RolloutStrategy,
		auditLog.PodsRestarted,
		auditLog.Error,
		auditLog.ChangedBy,
		auditLog.TriggeredBy,
	)

	return err
}

// GetByServiceID retrieves rotation audit logs for a specific service
func (r *RotationAuditLogRepository) GetByServiceID(ctx context.Context, serviceID uuid.UUID, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, event_id, service_id, service_name, environment,
		       secret_name, secret_path, old_version, new_version, status,
		       started_at, completed_at, duration_ms, rollout_strategy,
		       pods_restarted, error, changed_by, triggered_by, created_at
		FROM rotation_audit_logs
		WHERE service_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []interface{}
	for rows.Next() {
		type rotationLog struct {
			ID              uuid.UUID
			EventID         uuid.UUID
			ServiceID       uuid.UUID
			ServiceName     string
			Environment     string
			SecretName      string
			SecretPath      string
			OldVersion      int
			NewVersion      int
			Status          string
			StartedAt       time.Time
			CompletedAt     *time.Time
			DurationMs      *int64
			RolloutStrategy string
			PodsRestarted   int
			Error           string
			ChangedBy       string
			TriggeredBy     string
			CreatedAt       time.Time
		}

		log := &rotationLog{}
		var durationMs sql.NullInt64
		var rolloutStrategy, errorMsg, changedBy sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.EventID,
			&log.ServiceID,
			&log.ServiceName,
			&log.Environment,
			&log.SecretName,
			&log.SecretPath,
			&log.OldVersion,
			&log.NewVersion,
			&log.Status,
			&log.StartedAt,
			&log.CompletedAt,
			&durationMs,
			&rolloutStrategy,
			&log.PodsRestarted,
			&errorMsg,
			&changedBy,
			&log.TriggeredBy,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if durationMs.Valid {
			log.DurationMs = &durationMs.Int64
		}
		if rolloutStrategy.Valid {
			log.RolloutStrategy = rolloutStrategy.String
		}
		if errorMsg.Valid {
			log.Error = errorMsg.String
		}
		if changedBy.Valid {
			log.ChangedBy = changedBy.String
		}

		logs = append(logs, log)
	}

	return logs, nil
}
