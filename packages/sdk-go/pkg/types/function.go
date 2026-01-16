package types

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// SERVERLESS FUNCTION TYPES
// Enclii Functions - Scale-to-Zero Serverless Platform
// =============================================================================

// FunctionRuntime represents the runtime for a serverless function
type FunctionRuntime string

const (
	FunctionRuntimeGo     FunctionRuntime = "go"
	FunctionRuntimePython FunctionRuntime = "python"
	FunctionRuntimeNode   FunctionRuntime = "node"
	FunctionRuntimeRust   FunctionRuntime = "rust"
)

// FunctionStatus represents the lifecycle status of a function
type FunctionStatus string

const (
	FunctionStatusPending   FunctionStatus = "pending"
	FunctionStatusBuilding  FunctionStatus = "building"
	FunctionStatusDeploying FunctionStatus = "deploying"
	FunctionStatusReady     FunctionStatus = "ready"
	FunctionStatusFailed    FunctionStatus = "failed"
	FunctionStatusDeleting  FunctionStatus = "deleting"
)

// FunctionConfig contains the configuration for a serverless function
type FunctionConfig struct {
	// Runtime specifies the function runtime (go, python, node, rust)
	Runtime FunctionRuntime `json:"runtime" yaml:"runtime"`

	// Handler specifies the entry point (e.g., "main.Handler", "handler.main")
	Handler string `json:"handler" yaml:"handler"`

	// Memory is the memory limit for the function container (e.g., "128Mi", "256Mi")
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`

	// CPU is the CPU limit for the function container (e.g., "100m", "500m")
	CPU string `json:"cpu,omitempty" yaml:"cpu,omitempty"`

	// Timeout is the function execution timeout in seconds
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// MinReplicas is the minimum number of replicas (0 = scale-to-zero)
	MinReplicas int `json:"min_replicas" yaml:"minReplicas"`

	// MaxReplicas is the maximum number of replicas for autoscaling
	MaxReplicas int `json:"max_replicas" yaml:"maxReplicas"`

	// CooldownPeriod is seconds to wait before scaling down after last request
	CooldownPeriod int `json:"cooldown_period,omitempty" yaml:"cooldownPeriod,omitempty"`

	// Concurrency is the target concurrent requests per replica for scaling
	Concurrency int `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`

	// Env contains environment variables for the function
	Env []EnvVar `json:"env,omitempty" yaml:"env,omitempty"`
}

// Function represents a serverless function in the system
type Function struct {
	ID        uuid.UUID      `json:"id" db:"id"`
	ProjectID uuid.UUID      `json:"project_id" db:"project_id"`
	Name      string         `json:"name" db:"name"`
	Config    FunctionConfig `json:"config" db:"config"`
	Status    FunctionStatus `json:"status" db:"status"`

	// StatusMessage provides details about the current status
	StatusMessage string `json:"status_message,omitempty" db:"status_message"`

	// Kubernetes resources
	K8sNamespace    string `json:"k8s_namespace,omitempty" db:"k8s_namespace"`
	K8sResourceName string `json:"k8s_resource_name,omitempty" db:"k8s_resource_name"`

	// Image and endpoint
	ImageURI string `json:"image_uri,omitempty" db:"image_uri"`
	Endpoint string `json:"endpoint,omitempty" db:"endpoint"`

	// Runtime metrics
	AvailableReplicas int     `json:"available_replicas" db:"available_replicas"`
	InvocationCount   int64   `json:"invocation_count" db:"invocation_count"`
	AvgDurationMs     float64 `json:"avg_duration_ms" db:"avg_duration_ms"`
	LastInvokedAt     *time.Time `json:"last_invoked_at,omitempty" db:"last_invoked_at"`

	// Audit fields
	CreatedBy      *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
	CreatedByEmail string     `json:"created_by_email,omitempty" db:"created_by_email"`

	// Timestamps
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	DeployedAt *time.Time `json:"deployed_at,omitempty" db:"deployed_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// FunctionInvocation represents a single invocation of a function
type FunctionInvocation struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	FunctionID uuid.UUID  `json:"function_id" db:"function_id"`
	StartedAt  time.Time  `json:"started_at" db:"started_at"`
	DurationMs *int64     `json:"duration_ms,omitempty" db:"duration_ms"`
	StatusCode *int       `json:"status_code,omitempty" db:"status_code"`
	ColdStart  bool       `json:"cold_start" db:"cold_start"`
	ErrorType  *string    `json:"error_type,omitempty" db:"error_type"`
	RequestID  string     `json:"request_id,omitempty" db:"request_id"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// FunctionCreateRequest is the API request for creating a function
type FunctionCreateRequest struct {
	Name    string         `json:"name" binding:"required"`
	Config  FunctionConfig `json:"config" binding:"required"`
}

// FunctionUpdateRequest is the API request for updating a function
type FunctionUpdateRequest struct {
	Config *FunctionConfig `json:"config,omitempty"`
}

// FunctionInvokeRequest is the API request for invoking a function
type FunctionInvokeRequest struct {
	Body    []byte            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Method  string            `json:"method,omitempty"` // Default: POST
}

// FunctionInvokeResponse is the API response from a function invocation
type FunctionInvokeResponse struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	DurationMs int64             `json:"duration_ms"`
	ColdStart  bool              `json:"cold_start"`
	RequestID  string            `json:"request_id"`
}

// FunctionLogsRequest is the API request for fetching function logs
type FunctionLogsRequest struct {
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	Limit     int        `json:"limit,omitempty"` // Default: 100, Max: 1000
	Follow    bool       `json:"follow,omitempty"`
	RequestID string     `json:"request_id,omitempty"` // Filter by specific invocation
}

// FunctionLogEntry represents a single log entry from a function
type FunctionLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
	RequestID string    `json:"request_id,omitempty"`
	Source    string    `json:"source"` // user, system
}

// FunctionMetrics contains aggregated metrics for a function
type FunctionMetrics struct {
	FunctionID       uuid.UUID `json:"function_id"`
	FunctionName     string    `json:"function_name"`
	TotalInvocations int64     `json:"total_invocations"`
	SuccessCount     int64     `json:"success_count"`
	ErrorCount       int64     `json:"error_count"`
	ColdStartCount   int64     `json:"cold_start_count"`
	AvgDurationMs    float64   `json:"avg_duration_ms"`
	P50DurationMs    float64   `json:"p50_duration_ms"`
	P95DurationMs    float64   `json:"p95_duration_ms"`
	P99DurationMs    float64   `json:"p99_duration_ms"`
	Period           string    `json:"period"` // hourly, daily, weekly
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
}

// FunctionListResponse is the API response for listing functions
type FunctionListResponse struct {
	Functions []Function `json:"functions"`
	Total     int        `json:"total"`
}

// FunctionDeploymentInfo contains deployment information for the UI
type FunctionDeploymentInfo struct {
	Function         Function          `json:"function"`
	ScaledObjectName string            `json:"scaled_object_name,omitempty"`
	ServiceName      string            `json:"service_name,omitempty"`
	DeploymentName   string            `json:"deployment_name,omitempty"`
	PodCount         int               `json:"pod_count"`
	LastBuildLogs    string            `json:"last_build_logs,omitempty"`
}

// FunctionDefaults provides default values for function configuration
var FunctionDefaults = FunctionConfig{
	Memory:         "128Mi",
	CPU:            "100m",
	Timeout:        30,
	MinReplicas:    0,  // Enable scale-to-zero by default
	MaxReplicas:    10,
	CooldownPeriod: 300, // 5 minutes
	Concurrency:    100,
}

// FunctionRuntimeDefaults provides default handlers per runtime
var FunctionRuntimeDefaults = map[FunctionRuntime]string{
	FunctionRuntimeGo:     "main.Handler",
	FunctionRuntimePython: "handler.main",
	FunctionRuntimeNode:   "handler.main",
	FunctionRuntimeRust:   "handler",
}

// FunctionRuntimeBaseImages provides base images per runtime for cold start optimization
var FunctionRuntimeBaseImages = map[FunctionRuntime]string{
	FunctionRuntimeGo:     "gcr.io/distroless/static-debian12",
	FunctionRuntimePython: "python:3.12-slim",
	FunctionRuntimeNode:   "node:20-alpine",
	FunctionRuntimeRust:   "gcr.io/distroless/cc-debian12",
}

// ColdStartTargets defines target cold start times per runtime
var ColdStartTargets = map[FunctionRuntime]string{
	FunctionRuntimeGo:     "<500ms",
	FunctionRuntimePython: "<3s",
	FunctionRuntimeNode:   "<2s",
	FunctionRuntimeRust:   "<500ms",
}
