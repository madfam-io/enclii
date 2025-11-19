package lockbox

import (
	"time"

	"github.com/google/uuid"
)

// SecretProvider represents the backend secret store
type SecretProvider string

const (
	ProviderVault       SecretProvider = "vault"
	ProviderOnePassword SecretProvider = "1password"
	ProviderKubernetes  SecretProvider = "kubernetes"
)

// Secret represents a secret stored in the secret backend
type Secret struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`        // Secret name (e.g., "DATABASE_URL")
	Path        string         `json:"path"`        // Backend path (e.g., "secret/data/myapp/db")
	Provider    SecretProvider `json:"provider"`    // "vault", "1password", "kubernetes"
	Version     int            `json:"version"`     // Secret version number
	Value       string         `json:"value"`       // Encrypted value (never stored in DB)
	ServiceID   string         `json:"service_id"`  // Service using this secret
	Environment string         `json:"environment"` // "production", "staging", "dev"
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	RotatedAt   *time.Time     `json:"rotated_at,omitempty"` // Last rotation timestamp
}

// SecretMetadata contains metadata about a secret without the actual value
type SecretMetadata struct {
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	Provider    SecretProvider `json:"provider"`
	Version     int            `json:"version"`
	ServiceID   string         `json:"service_id"`
	Environment string         `json:"environment"`
	LastRotated *time.Time     `json:"last_rotated,omitempty"`
}

// SecretChangeEvent represents a detected change in a secret
type SecretChangeEvent struct {
	ID          uuid.UUID      `json:"id"`
	SecretPath  string         `json:"secret_path"`
	SecretName  string         `json:"secret_name"`
	Provider    SecretProvider `json:"provider"`
	OldVersion  int            `json:"old_version"`
	NewVersion  int            `json:"new_version"`
	ServiceID   string         `json:"service_id"`
	Environment string         `json:"environment"`
	DetectedAt  time.Time      `json:"detected_at"`
	ProcessedAt *time.Time     `json:"processed_at,omitempty"`
	Status      RotationStatus `json:"status"`
	Error       string         `json:"error,omitempty"`
	TriggeredBy string         `json:"triggered_by"`         // "watcher", "webhook", "manual"
	RolloutID   string         `json:"rollout_id,omitempty"` // K8s rollout tracking
}

// RotationStatus represents the state of a secret rotation
type RotationStatus string

const (
	RotationPending    RotationStatus = "pending"
	RotationInProgress RotationStatus = "in_progress"
	RotationCompleted  RotationStatus = "completed"
	RotationFailed     RotationStatus = "failed"
	RotationRolledBack RotationStatus = "rolled_back"
)

// RotationAuditLog records secret rotation activity
type RotationAuditLog struct {
	ID              uuid.UUID      `json:"id"`
	EventID         uuid.UUID      `json:"event_id"`
	ServiceID       string         `json:"service_id"`
	ServiceName     string         `json:"service_name"`
	Environment     string         `json:"environment"`
	SecretName      string         `json:"secret_name"`
	SecretPath      string         `json:"secret_path"`
	OldVersion      int            `json:"old_version"`
	NewVersion      int            `json:"new_version"`
	Status          RotationStatus `json:"status"`
	StartedAt       time.Time      `json:"started_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	Duration        time.Duration  `json:"duration,omitempty"`
	RolloutStrategy string         `json:"rollout_strategy"` // "rolling", "recreate", "blue-green"
	PodsRestarted   int            `json:"pods_restarted"`
	Error           string         `json:"error,omitempty"`
	ChangedBy       string         `json:"changed_by,omitempty"` // Who changed the secret
	TriggeredBy     string         `json:"triggered_by"`         // How rotation was triggered
}

// VaultConfig holds Vault connection configuration
type VaultConfig struct {
	Address      string        // Vault server address
	Token        string        // Vault token
	Namespace    string        // Vault namespace (optional)
	PollInterval time.Duration // How often to poll for changes
	Enabled      bool          // Enable Vault integration
}

// SecretReference links a service to secrets it uses
type SecretReference struct {
	ServiceID   string    `json:"service_id"`
	Environment string    `json:"environment"`
	SecretName  string    `json:"secret_name"` // Environment variable name
	SecretPath  string    `json:"secret_path"` // Vault/1Password path
	Required    bool      `json:"required"`    // Is this secret required for startup?
	CreatedAt   time.Time `json:"created_at"`
}
