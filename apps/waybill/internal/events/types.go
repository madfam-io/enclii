package events

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of usage event
type EventType string

const (
	// Deployment events
	EventDeploymentStarted EventType = "deployment.started"
	EventDeploymentStopped EventType = "deployment.stopped"
	EventDeploymentScaled  EventType = "deployment.scaled"

	// Build events
	EventBuildStarted   EventType = "build.started"
	EventBuildCompleted EventType = "build.completed"
	EventBuildFailed    EventType = "build.failed"

	// Storage events
	EventVolumeCreated  EventType = "volume.created"
	EventVolumeDeleted  EventType = "volume.deleted"
	EventVolumeResized  EventType = "volume.resized"

	// Network events
	EventBandwidthUsage EventType = "bandwidth.usage"

	// Domain events
	EventDomainAdded   EventType = "domain.added"
	EventDomainRemoved EventType = "domain.removed"
)

// MetricType represents billable metric types
type MetricType string

const (
	MetricComputeGBHours   MetricType = "compute_gb_hours"
	MetricBuildMinutes     MetricType = "build_minutes"
	MetricStorageGBHours   MetricType = "storage_gb_hours"
	MetricBandwidthGB      MetricType = "bandwidth_gb"
	MetricCustomDomains    MetricType = "custom_domains"
)

// UsageEvent represents a single usage event
type UsageEvent struct {
	ID           uuid.UUID         `json:"id" db:"id"`
	ProjectID    uuid.UUID         `json:"project_id" db:"project_id"`
	TeamID       *uuid.UUID        `json:"team_id,omitempty" db:"team_id"`
	EventType    EventType         `json:"event_type" db:"event_type"`
	ResourceType string            `json:"resource_type" db:"resource_type"`
	ResourceID   uuid.UUID         `json:"resource_id" db:"resource_id"`
	ResourceName string            `json:"resource_name,omitempty" db:"resource_name"`
	Metrics      map[string]float64 `json:"metrics" db:"metrics"`
	Metadata     map[string]string `json:"metadata,omitempty" db:"metadata"`
	Timestamp    time.Time         `json:"timestamp" db:"timestamp"`
	ProcessedAt  *time.Time        `json:"processed_at,omitempty" db:"processed_at"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
}

// EventRequest is the request to record an event
type EventRequest struct {
	EventType    EventType          `json:"event_type" binding:"required"`
	ProjectID    uuid.UUID          `json:"project_id" binding:"required"`
	TeamID       *uuid.UUID         `json:"team_id,omitempty"`
	ResourceType string             `json:"resource_type" binding:"required"`
	ResourceID   uuid.UUID          `json:"resource_id" binding:"required"`
	ResourceName string             `json:"resource_name,omitempty"`
	Metrics      map[string]float64 `json:"metrics" binding:"required"`
	Metadata     map[string]string  `json:"metadata,omitempty"`
	Timestamp    *time.Time         `json:"timestamp,omitempty"`
}

// HourlyUsage represents aggregated hourly usage
type HourlyUsage struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	ProjectID  uuid.UUID  `json:"project_id" db:"project_id"`
	MetricType MetricType `json:"metric_type" db:"metric_type"`
	Value      float64    `json:"value" db:"value"`
	Hour       time.Time  `json:"hour" db:"hour"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// DailyUsage represents aggregated daily usage
type DailyUsage struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	ProjectID  uuid.UUID  `json:"project_id" db:"project_id"`
	MetricType MetricType `json:"metric_type" db:"metric_type"`
	Value      float64    `json:"value" db:"value"`
	Date       time.Time  `json:"date" db:"date"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

// UsageSummary represents a usage summary for a project
type UsageSummary struct {
	ProjectID       uuid.UUID                `json:"project_id"`
	PeriodStart     time.Time                `json:"period_start"`
	PeriodEnd       time.Time                `json:"period_end"`
	Metrics         map[MetricType]float64   `json:"metrics"`
	Costs           map[MetricType]float64   `json:"costs"`
	TotalCost       float64                  `json:"total_cost"`
	EstimatedMonthly float64                 `json:"estimated_monthly"`
}

// DeploymentMetrics captures compute resource metrics
type DeploymentMetrics struct {
	Replicas      int     `json:"replicas"`
	CPUMillicores int     `json:"cpu_millicores"`
	MemoryMB      int     `json:"memory_mb"`
	// Calculated
	GBEquivalent  float64 `json:"gb_equivalent"`
}

// BuildMetrics captures build resource metrics
type BuildMetrics struct {
	DurationSeconds float64 `json:"duration_seconds"`
	ImageSizeMB     float64 `json:"image_size_mb"`
	// Calculated
	Minutes         float64 `json:"minutes"`
}

// StorageMetrics captures storage resource metrics
type StorageMetrics struct {
	SizeGB float64 `json:"size_gb"`
}

// BandwidthMetrics captures network resource metrics
type BandwidthMetrics struct {
	EgressGB  float64 `json:"egress_gb"`
	IngressGB float64 `json:"ingress_gb"`
}

// CalculateGBEquivalent converts CPU + Memory to GB-equivalent
// Using a simplified model: 1 GB = 1 GB RAM or 1 vCPU
func (m *DeploymentMetrics) CalculateGBEquivalent() float64 {
	// Memory component (MB to GB)
	memoryGB := float64(m.MemoryMB) / 1024.0

	// CPU component (millicores to cores, 1 core = 1 GB equivalent)
	cpuGB := float64(m.CPUMillicores) / 1000.0

	// Take the larger of the two (similar to Railway's pricing)
	if memoryGB > cpuGB {
		return memoryGB * float64(m.Replicas)
	}
	return cpuGB * float64(m.Replicas)
}
