package builder

import (
	"context"

	"github.com/google/uuid"
	"github.com/madfam-org/enclii/apps/roundhouse/internal/queue"
)

// BuildMode specifies the container build backend
type BuildMode string

const (
	// BuildModeDocker uses local Docker daemon (requires Docker socket)
	// WARNING: Security risk - allows container escape
	BuildModeDocker BuildMode = "docker"

	// BuildModeKaniko uses Kaniko in Kubernetes (rootless, secure)
	// Recommended for production
	BuildModeKaniko BuildMode = "kaniko"
)

// Builder is the interface for container image builders
type Builder interface {
	// Execute runs a build job and returns the result
	Execute(ctx context.Context, job *queue.BuildJob) (*queue.BuildResult, error)
}

// LogFunc is a callback for streaming build logs
type LogFunc func(jobID uuid.UUID, line string)
