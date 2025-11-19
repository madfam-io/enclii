package compliance

import (
	"time"
)

// DrataEvent represents a Drata webhook event
// Based on Drata's webhook API documentation
type DrataEvent struct {
	// Event metadata
	EventType   string    `json:"event_type"`
	EventID     string    `json:"event_id"`
	Timestamp   time.Time `json:"timestamp"`
	Integration string    `json:"integration"`

	// Entity information
	Entity DrataEntity `json:"entity"`

	// Attributes (evidence data)
	Attributes DrataAttributes `json:"attributes"`

	// Personnel (who performed the action)
	Personnel DrataPersonnel `json:"personnel"`
}

// DrataEntity represents the entity being monitored
type DrataEntity struct {
	Type        string            `json:"type"`        // "deployment"
	ID          string            `json:"id"`          // Unique identifier
	Name        string            `json:"name"`        // Service name
	Environment string            `json:"environment"` // "production", "staging"
	Tags        map[string]string `json:"tags,omitempty"`
}

// DrataAttributes represents the evidence attributes
type DrataAttributes struct {
	// Deployment metadata
	DeploymentID   string    `json:"deployment_id"`
	ReleaseVersion string    `json:"release_version"`
	ImageURI       string    `json:"image_uri"`
	DeployedAt     time.Time `json:"deployed_at"`
	Status         string    `json:"status"` // "success", "failed"

	// Source control
	Repository    string `json:"repository"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message,omitempty"`
	Branch        string `json:"branch,omitempty"`

	// Change management
	ChangeRequest *DrataChangeRequest `json:"change_request,omitempty"`

	// Code review
	PullRequest *DrataPullRequest `json:"pull_request,omitempty"`

	// Security & compliance
	Security *DrataSecurity `json:"security,omitempty"`

	// Evidence
	Evidence *DrataEvidenceData `json:"evidence,omitempty"`
}

// DrataChangeRequest represents change ticket information
type DrataChangeRequest struct {
	TicketURL string    `json:"ticket_url"`
	TicketID  string    `json:"ticket_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// DrataPullRequest represents PR approval information
type DrataPullRequest struct {
	URL         string    `json:"url"`
	Number      int       `json:"number"`
	Title       string    `json:"title,omitempty"`
	State       string    `json:"state"`
	MergedAt    time.Time `json:"merged_at,omitempty"`
	ApprovedBy  string    `json:"approved_by"`
	ApprovedAt  time.Time `json:"approved_at"`
	CIStatus    string    `json:"ci_status"`
	ReviewCount int       `json:"review_count,omitempty"`
}

// DrataSecurity represents security evidence
type DrataSecurity struct {
	ImageSigned       bool   `json:"image_signed"`
	SignatureVerified bool   `json:"signature_verified"`
	SBOMGenerated     bool   `json:"sbom_generated"`
	SBOMFormat        string `json:"sbom_format,omitempty"`
	VulnerabilityScan string `json:"vulnerability_scan,omitempty"` // Future: Grype results
}

// DrataEvidenceData represents compliance evidence
type DrataEvidenceData struct {
	Type              string `json:"type"` // "deployment_approval"
	ComplianceReceipt string `json:"compliance_receipt,omitempty"`
	ReceiptSignature  string `json:"receipt_signature,omitempty"`
	VerificationURL   string `json:"verification_url,omitempty"`
}

// DrataPersonnel represents the person who performed the action
type DrataPersonnel struct {
	Email  string `json:"email"`
	Name   string `json:"name,omitempty"`
	UserID string `json:"user_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

// FormatForDrata converts DeploymentEvidence to Drata's webhook format
func FormatForDrata(evidence *DeploymentEvidence) *DrataEvent {
	event := &DrataEvent{
		EventType:   "deployment",
		EventID:     evidence.EventID,
		Timestamp:   evidence.Timestamp,
		Integration: "enclii_switchyard",
		Entity: DrataEntity{
			Type:        "deployment",
			ID:          evidence.DeploymentID,
			Name:        evidence.ServiceName,
			Environment: evidence.Environment,
			Tags: map[string]string{
				"project": evidence.ProjectName,
				"service": evidence.ServiceName,
			},
		},
		Attributes: DrataAttributes{
			DeploymentID:   evidence.DeploymentID,
			ReleaseVersion: evidence.ReleaseVersion,
			ImageURI:       evidence.ImageURI,
			DeployedAt:     evidence.DeployedAt,
			Status:         "success",
			Repository:     evidence.GitRepo,
			CommitSHA:      evidence.GitSHA,
			CommitMessage:  evidence.CommitMessage,
		},
		Personnel: DrataPersonnel{
			Email: evidence.DeployedByEmail,
			Name:  evidence.DeployedBy,
		},
	}

	// Add change request if available
	if evidence.ChangeTicket != "" {
		event.Attributes.ChangeRequest = &DrataChangeRequest{
			TicketURL: evidence.ChangeTicket,
			Status:    "approved",
		}
	}

	// Add pull request information if available
	if evidence.PRURL != "" {
		event.Attributes.PullRequest = &DrataPullRequest{
			URL:        evidence.PRURL,
			Number:     evidence.PRNumber,
			State:      "merged",
			ApprovedBy: evidence.ApprovedBy,
			ApprovedAt: evidence.ApprovedAt,
			CIStatus:   evidence.CIStatus,
		}
	}

	// Add security evidence
	event.Attributes.Security = &DrataSecurity{
		ImageSigned:       evidence.ImageSignature != "",
		SignatureVerified: evidence.SignatureVerified,
		SBOMGenerated:     evidence.SBOM != "",
		SBOMFormat:        evidence.SBOMFormat,
	}

	// Add compliance evidence
	if evidence.ComplianceReceipt != "" {
		event.Attributes.Evidence = &DrataEvidenceData{
			Type:              "deployment_approval",
			ComplianceReceipt: evidence.ComplianceReceipt,
			ReceiptSignature:  evidence.ReceiptSignature,
		}
	}

	return event
}

// DrataComplianceFrameworks represents which compliance frameworks this evidence supports
type DrataComplianceFrameworks struct {
	SOC2     []string `json:"soc2"`
	ISO27001 []string `json:"iso27001"`
	HIPAA    []string `json:"hipaa,omitempty"`
	PCI      []string `json:"pci,omitempty"`
}

// GetDrataFrameworks returns which compliance frameworks this evidence satisfies
func GetDrataFrameworks(evidence *DeploymentEvidence) *DrataComplianceFrameworks {
	frameworks := &DrataComplianceFrameworks{
		SOC2:     []string{},
		ISO27001: []string{},
		HIPAA:    []string{},
		PCI:      []string{},
	}

	// SOC 2 controls
	frameworks.SOC2 = append(frameworks.SOC2, "CC8.1") // Monitoring

	if evidence.ChangeTicket != "" {
		frameworks.SOC2 = append(frameworks.SOC2, "CC7.1") // Change management
	}

	if evidence.PRURL != "" && evidence.ApprovedBy != "" {
		frameworks.SOC2 = append(frameworks.SOC2, "CC7.2") // Code review
		frameworks.SOC2 = append(frameworks.SOC2, "CC8.2") // Approval process
	}

	// ISO 27001 controls
	if evidence.PRURL != "" {
		frameworks.ISO27001 = append(frameworks.ISO27001, "A.14.2.2") // System change control
	}

	if evidence.SBOM != "" {
		frameworks.ISO27001 = append(frameworks.ISO27001, "A.14.2.1") // Secure development policy
	}

	// HIPAA (if healthcare)
	if evidence.ImageSignature != "" {
		frameworks.HIPAA = append(frameworks.HIPAA, "164.312(c)(1)") // Integrity controls
	}

	// PCI DSS (if payment processing)
	if evidence.PRURL != "" && evidence.ApprovedBy != "" {
		frameworks.PCI = append(frameworks.PCI, "6.3.2") // Code review requirement
	}

	return frameworks
}
