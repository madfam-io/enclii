package topology

import (
	"time"
)

// ServiceNode represents a service in the topology graph
type ServiceNode struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ProjectID   string            `json:"project_id"`
	ProjectName string            `json:"project_name"`
	Environment string            `json:"environment"`
	Type        ServiceType       `json:"type"`               // "http", "grpc", "database", "queue", "cache"
	Status      HealthStatus      `json:"status"`             // "healthy", "degraded", "unhealthy", "unknown"
	Metadata    map[string]string `json:"metadata"`           // Additional service metadata
	Position    *GraphPosition    `json:"position,omitempty"` // Visual position for frontend

	// Health metrics
	Replicas          int     `json:"replicas"`
	AvailableReplicas int     `json:"available_replicas"`
	ErrorRate         float64 `json:"error_rate,omitempty"`    // 0.0 to 1.0
	ResponseTime      float64 `json:"response_time,omitempty"` // Average response time in ms

	// Deployment info
	Version   string    `json:"version,omitempty"`
	ImageURI  string    `json:"image_uri,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceType represents the type of service
type ServiceType string

const (
	ServiceTypeHTTP     ServiceType = "http"
	ServiceTypeGRPC     ServiceType = "grpc"
	ServiceTypeDatabase ServiceType = "database"
	ServiceTypeQueue    ServiceType = "queue"
	ServiceTypeCache    ServiceType = "cache"
	ServiceTypeStorage  ServiceType = "storage"
	ServiceTypeUnknown  ServiceType = "unknown"
)

// HealthStatus represents the health state of a service
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// GraphPosition represents visual position in the topology graph
type GraphPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// DependencyEdge represents a dependency between two services
type DependencyEdge struct {
	ID        string            `json:"id"`
	SourceID  string            `json:"source_id"`          // Service that depends on target
	TargetID  string            `json:"target_id"`          // Service being depended on
	Type      DependencyType    `json:"type"`               // "sync", "async", "storage"
	Protocol  string            `json:"protocol,omitempty"` // "http", "grpc", "tcp", "postgres", etc.
	Required  bool              `json:"required"`           // Is this dependency required for startup?
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// DependencyType represents the type of dependency
type DependencyType string

const (
	DependencyTypeSync    DependencyType = "sync"    // Synchronous dependency (HTTP, gRPC)
	DependencyTypeAsync   DependencyType = "async"   // Asynchronous dependency (message queue)
	DependencyTypeStorage DependencyType = "storage" // Storage dependency (database, cache)
)

// TopologyGraph represents the complete service topology
type TopologyGraph struct {
	Nodes       []*ServiceNode    `json:"nodes"`
	Edges       []*DependencyEdge `json:"edges"`
	Environment string            `json:"environment"`
	GeneratedAt time.Time         `json:"generated_at"`
	Stats       *TopologyStats    `json:"stats"`
}

// TopologyStats provides summary statistics about the topology
type TopologyStats struct {
	TotalServices     int                       `json:"total_services"`
	HealthyServices   int                       `json:"healthy_services"`
	DegradedServices  int                       `json:"degraded_services"`
	UnhealthyServices int                       `json:"unhealthy_services"`
	TotalDependencies int                       `json:"total_dependencies"`
	ServicesByType    map[ServiceType]int       `json:"services_by_type"`
	HealthByProject   map[string]*ProjectHealth `json:"health_by_project"`
}

// ProjectHealth tracks health metrics for a project
type ProjectHealth struct {
	ProjectID         string  `json:"project_id"`
	ProjectName       string  `json:"project_name"`
	TotalServices     int     `json:"total_services"`
	HealthyServices   int     `json:"healthy_services"`
	DegradedServices  int     `json:"degraded_services"`
	UnhealthyServices int     `json:"unhealthy_services"`
	HealthScore       float64 `json:"health_score"` // 0.0 to 1.0
}

// ImpactAnalysis represents the impact of a service failure
type ImpactAnalysis struct {
	ServiceID          string         `json:"service_id"`
	ServiceName        string         `json:"service_name"`
	DirectDependents   []string       `json:"direct_dependents"`   // Services that directly depend on this
	IndirectDependents []string       `json:"indirect_dependents"` // Services indirectly affected
	TotalImpact        int            `json:"total_impact"`        // Total number of affected services
	CriticalPath       bool           `json:"critical_path"`       // Is this on the critical path?
	Severity           ImpactSeverity `json:"severity"`
}

// ImpactSeverity represents how severe the impact would be
type ImpactSeverity string

const (
	ImpactSeverityLow      ImpactSeverity = "low"      // Affects 1-2 services
	ImpactSeverityModerate ImpactSeverity = "moderate" // Affects 3-5 services
	ImpactSeverityHigh     ImpactSeverity = "high"     // Affects 6-10 services
	ImpactSeverityCritical ImpactSeverity = "critical" // Affects 10+ services
)

// DependencyPath represents a path from one service to another
type DependencyPath struct {
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Path     []string `json:"path"`     // Service IDs along the path
	Distance int      `json:"distance"` // Number of hops
	Type     string   `json:"type"`     // "upstream" or "downstream"
}

// ServiceDependencies contains all dependencies for a service
type ServiceDependencies struct {
	ServiceID       string            `json:"service_id"`
	ServiceName     string            `json:"service_name"`
	Upstream        []*DependencyEdge `json:"upstream"`   // Services this depends on
	Downstream      []*DependencyEdge `json:"downstream"` // Services that depend on this
	UpstreamCount   int               `json:"upstream_count"`
	DownstreamCount int               `json:"downstream_count"`
}
