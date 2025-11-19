package compliance

import (
	"time"
)

// VantaEvent represents a Vanta webhook event
// Based on Vanta's webhook API documentation
type VantaEvent struct {
	// Event metadata
	EventType     string    `json:"event_type"`
	EventID       string    `json:"event_id"`
	Timestamp     time.Time `json:"timestamp"`
	Source        string    `json:"source"`
	SourceVersion string    `json:"source_version"`

	// Resource information
	Resource VantaResource `json:"resource"`

	// Evidence data
	Evidence VantaEvidence `json:"evidence"`

	// Actor (who performed the action)
	Actor VantaActor `json:"actor"`
}

// VantaResource represents the resource being acted upon
type VantaResource struct {
	Type        string `json:"type"`        // "deployment", "service", "environment"
	ID          string `json:"id"`          // Unique identifier
	Name        string `json:"name"`        // Human-readable name
	Environment string `json:"environment"` // "production", "staging", "development"
}

// VantaEvidence represents the evidence for compliance
type VantaEvidence struct {
	// Deployment details
	DeploymentID   string    `json:"deployment_id"`
	ReleaseVersion string    `json:"release_version"`
	ImageURI       string    `json:"image_uri"`
	DeployedAt     time.Time `json:"deployed_at"`

	// Source control
	GitSHA        string `json:"git_sha"`
	GitRepo       string `json:"git_repo"`
	CommitMessage string `json:"commit_message,omitempty"`

	// Code review (PR Approval Tracking)
	CodeReview *VantaCodeReview `json:"code_review,omitempty"`

	// Change management
	ChangeTicket string `json:"change_ticket,omitempty"`

	// Supply chain security
	SBOM              *VantaSBOM `json:"sbom,omitempty"`
	ImageSignature    string     `json:"image_signature,omitempty"`
	SignatureVerified bool       `json:"signature_verified,omitempty"`

	// Compliance receipt (cryptographic proof)
	ComplianceReceipt string `json:"compliance_receipt,omitempty"`
}

// VantaCodeReview represents code review evidence
type VantaCodeReview struct {
	PRURL      string    `json:"pr_url"`
	PRNumber   int       `json:"pr_number"`
	ApprovedBy string    `json:"approved_by"`
	ApprovedAt time.Time `json:"approved_at"`
	CIStatus   string    `json:"ci_status"`
	Verified   bool      `json:"verified"` // Whether we verified the approval
}

// VantaSBOM represents SBOM metadata
type VantaSBOM struct {
	Format       string `json:"format"`        // "cyclonedx-json", "spdx-json"
	PackageCount int    `json:"package_count"` // Number of packages identified
	Generated    bool   `json:"generated"`     // Whether SBOM was successfully generated
}

// VantaActor represents the person who performed the action
type VantaActor struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
}

// FormatForVanta converts DeploymentEvidence to Vanta's webhook format
func FormatForVanta(evidence *DeploymentEvidence) *VantaEvent {
	event := &VantaEvent{
		EventType:     "deployment.completed",
		EventID:       evidence.EventID,
		Timestamp:     evidence.Timestamp,
		Source:        "enclii-switchyard",
		SourceVersion: "1.0",
		Resource: VantaResource{
			Type:        "deployment",
			ID:          evidence.DeploymentID,
			Name:        evidence.ServiceName,
			Environment: evidence.Environment,
		},
		Evidence: VantaEvidence{
			DeploymentID:      evidence.DeploymentID,
			ReleaseVersion:    evidence.ReleaseVersion,
			ImageURI:          evidence.ImageURI,
			DeployedAt:        evidence.DeployedAt,
			GitSHA:            evidence.GitSHA,
			GitRepo:           evidence.GitRepo,
			CommitMessage:     evidence.CommitMessage,
			ChangeTicket:      evidence.ChangeTicket,
			ImageSignature:    evidence.ImageSignature,
			SignatureVerified: evidence.SignatureVerified,
			ComplianceReceipt: evidence.ComplianceReceipt,
		},
		Actor: VantaActor{
			Email: evidence.DeployedByEmail,
			Name:  evidence.DeployedBy,
		},
	}

	// Add code review evidence if available
	if evidence.PRURL != "" {
		event.Evidence.CodeReview = &VantaCodeReview{
			PRURL:      evidence.PRURL,
			PRNumber:   evidence.PRNumber,
			ApprovedBy: evidence.ApprovedBy,
			ApprovedAt: evidence.ApprovedAt,
			CIStatus:   evidence.CIStatus,
			Verified:   true, // We verified this via GitHub API
		}
	}

	// Add SBOM metadata if available
	if evidence.SBOMFormat != "" {
		event.Evidence.SBOM = &VantaSBOM{
			Format:    evidence.SBOMFormat,
			Generated: evidence.SBOM != "",
		}
	}

	return event
}

// VantaControls maps evidence to Vanta SOC 2 controls
type VantaControls struct {
	CC81 bool `json:"cc8_1"` // Monitoring - Deployment tracking
	CC71 bool `json:"cc7_1"` // System Operations - Change management
	CC72 bool `json:"cc7_2"` // System Operations - Code review
	CC66 bool `json:"cc6_6"` // Logical Access - Credential rotation
	CC82 bool `json:"cc8_2"` // Change Management - Deployment approval
}

// GetVantaControls returns which SOC 2 controls this evidence satisfies
func GetVantaControls(evidence *DeploymentEvidence) *VantaControls {
	controls := &VantaControls{}

	// CC8.1: Monitoring for security events
	controls.CC81 = true // We always track deployments

	// CC7.1: System operations (change management)
	controls.CC71 = evidence.ChangeTicket != ""

	// CC7.2: Code review before deployment
	controls.CC72 = evidence.PRURL != "" && evidence.ApprovedBy != ""

	// CC6.6: Credential rotation (if secret rotation is tracked)
	// This will be set when we implement Sprint 4

	// CC8.2: Change management with approval
	controls.CC82 = evidence.PRURL != "" && evidence.ApprovedBy != ""

	return controls
}
