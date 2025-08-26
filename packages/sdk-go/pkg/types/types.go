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