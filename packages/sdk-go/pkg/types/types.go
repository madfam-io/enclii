package types

import (
	"time"

	"github.com/google/uuid"
)

// Project represents a collection of services
type Project struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Environment represents a deployment target (dev, staging, prod, preview-*)
type Environment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	ProjectID     uuid.UUID `json:"project_id" db:"project_id"`
	Name          string    `json:"name" db:"name"`
	KubeNamespace string    `json:"kube_namespace" db:"kube_namespace"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// Service represents a deployable application
type Service struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	ProjectID   uuid.UUID   `json:"project_id" db:"project_id"`
	Name        string      `json:"name" db:"name"`
	GitRepo     string      `json:"git_repo" db:"git_repo"`
	BuildConfig BuildConfig `json:"build_config" db:"build_config"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// BuildConfig defines how to build a service
type BuildConfig struct {
	Type       BuildType `json:"type"`
	Dockerfile string    `json:"dockerfile,omitempty"`
	Buildpack  string    `json:"buildpack,omitempty"`
}

type BuildType string

const (
	BuildTypeAuto       BuildType = "auto"
	BuildTypeDockerfile BuildType = "dockerfile"
	BuildTypeBuildpack  BuildType = "buildpack"
)

// Release represents a built and immutable version of a service
type Release struct {
	ID       uuid.UUID     `json:"id" db:"id"`
	ServiceID uuid.UUID    `json:"service_id" db:"service_id"`
	Version   string       `json:"version" db:"version"`
	ImageURI  string       `json:"image_uri" db:"image_uri"`
	GitSHA    string       `json:"git_sha" db:"git_sha"`
	Status    ReleaseStatus `json:"status" db:"status"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
}

type ReleaseStatus string

const (
	ReleaseStatusBuilding ReleaseStatus = "building"
	ReleaseStatusReady    ReleaseStatus = "ready"
	ReleaseStatusFailed   ReleaseStatus = "failed"
)

// Deployment represents a running instance of a release in an environment
type Deployment struct {
	ID            uuid.UUID        `json:"id" db:"id"`
	ReleaseID     uuid.UUID        `json:"release_id" db:"release_id"`
	EnvironmentID uuid.UUID        `json:"environment_id" db:"environment_id"`
	Replicas      int              `json:"replicas" db:"replicas"`
	Status        DeploymentStatus `json:"status" db:"status"`
	Health        HealthStatus     `json:"health" db:"health"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
}

type DeploymentStatus string

const (
	DeploymentStatusPending DeploymentStatus = "pending"
	DeploymentStatusRunning DeploymentStatus = "running"
	DeploymentStatusFailed  DeploymentStatus = "failed"
)

type HealthStatus string

const (
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ServiceSpec represents the desired configuration for a service
type ServiceSpec struct {
	APIVersion string            `yaml:"apiVersion" json:"api_version"`
	Kind       string            `yaml:"kind" json:"kind"`
	Metadata   ServiceMetadata   `yaml:"metadata" json:"metadata"`
	Spec       ServiceSpecConfig `yaml:"spec" json:"spec"`
}

type ServiceMetadata struct {
	Name    string `yaml:"name" json:"name"`
	Project string `yaml:"project" json:"project"`
}

type ServiceSpecConfig struct {
	Build   BuildSpec   `yaml:"build" json:"build"`
	Runtime RuntimeSpec `yaml:"runtime" json:"runtime"`
	Env     []EnvVar    `yaml:"env,omitempty" json:"env,omitempty"`
}

type BuildSpec struct {
	Type       string `yaml:"type" json:"type"`
	Dockerfile string `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`
}

type RuntimeSpec struct {
	Port        int    `yaml:"port" json:"port"`
	Replicas    int    `yaml:"replicas" json:"replicas"`
	HealthCheck string `yaml:"healthCheck" json:"health_check"`
}

type EnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// Role represents a user's role in the system
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
)

// User represents a user account in the system
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"` // Never expose password hash in JSON
	Name         string    `json:"name" db:"name"`
	OIDCSub      string    `json:"oidc_sub,omitempty" db:"oidc_sub"` // OIDC subject identifier
	Active       bool      `json:"active" db:"active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}

// Team represents a group of users
type Team struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ProjectAccess represents a user's access to a project with environment-specific permissions
type ProjectAccess struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	ProjectID     uuid.UUID  `json:"project_id" db:"project_id"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty" db:"environment_id"` // nil = all environments
	Role          Role       `json:"role" db:"role"`
	GrantedBy     uuid.UUID  `json:"granted_by" db:"granted_by"`
	GrantedAt     time.Time  `json:"granted_at" db:"granted_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" db:"expires_at"` // for temporary access
}

// AuditLog represents an immutable audit trail entry
type AuditLog struct {
	ID           uuid.UUID         `json:"id" db:"id"`
	Timestamp    time.Time         `json:"timestamp" db:"timestamp"`
	ActorID      uuid.UUID         `json:"actor_id" db:"actor_id"`
	ActorEmail   string            `json:"actor_email" db:"actor_email"`
	ActorRole    Role              `json:"actor_role" db:"actor_role"`
	Action       string            `json:"action" db:"action"` // 'deploy', 'scale', 'delete', 'access_logs'
	ResourceType string            `json:"resource_type" db:"resource_type"` // 'service', 'environment', 'secret'
	ResourceID   string            `json:"resource_id" db:"resource_id"`
	ResourceName string            `json:"resource_name" db:"resource_name"`
	ProjectID    *uuid.UUID        `json:"project_id,omitempty" db:"project_id"`
	EnvironmentID *uuid.UUID       `json:"environment_id,omitempty" db:"environment_id"`
	IPAddress    string            `json:"ip_address" db:"ip_address"`
	UserAgent    string            `json:"user_agent" db:"user_agent"`
	Outcome      string            `json:"outcome" db:"outcome"` // 'success', 'failure', 'denied'
	Context      map[string]interface{} `json:"context" db:"context"` // {pr_url, commit_sha, approver, change_ticket}
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}