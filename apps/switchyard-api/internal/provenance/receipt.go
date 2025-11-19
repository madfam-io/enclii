package provenance

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// ComplianceReceipt represents a signed audit trail for deployment provenance
// This receipt can be provided to auditors (SOC 2, ISO 27001, etc.) as proof that:
// 1. Code was reviewed and approved before deployment
// 2. CI checks passed
// 3. The deployment followed established change control procedures
type ComplianceReceipt struct {
	// Version of the receipt format (for future compatibility)
	Version string `json:"version"`

	// Timestamp when the receipt was generated
	GeneratedAt time.Time `json:"generated_at"`

	// Deployment information
	DeploymentID string `json:"deployment_id"`
	ServiceName  string `json:"service_name"`
	Environment  string `json:"environment"`

	// Release information
	ReleaseVersion string `json:"release_version"`
	GitCommitSHA   string `json:"git_commit_sha"`
	ImageURI       string `json:"image_uri"`

	// Pull request provenance
	PullRequest PullRequestEvidence `json:"pull_request"`

	// Approval evidence
	Approvals []ApprovalEvidence `json:"approvals"`

	// CI/CD evidence
	CIStatus CIEvidence `json:"ci_status"`

	// Change management
	ChangeTicket string `json:"change_ticket,omitempty"`

	// Policy compliance
	PolicyCompliant bool                   `json:"policy_compliant"`
	PolicyChecks    map[string]interface{} `json:"policy_checks"`

	// Cryptographic signature (SHA256 hash of receipt)
	Signature string `json:"signature"`
}

// PullRequestEvidence captures PR metadata
type PullRequestEvidence struct {
	URL          string    `json:"url"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	MergedAt     time.Time `json:"merged_at"`
	MergeCommit  string    `json:"merge_commit_sha"`
	Repository   string    `json:"repository"`
	BaseBranch   string    `json:"base_branch"`
	HeadCommitSHA string   `json:"head_commit_sha"`
}

// ApprovalEvidence captures who approved the PR
type ApprovalEvidence struct {
	Approver    string    `json:"approver"`
	ApproverEmail string  `json:"approver_email,omitempty"`
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// CIEvidence captures CI check results
type CIEvidence struct {
	State       string            `json:"state"`
	TotalChecks int               `json:"total_checks"`
	Checks      map[string]string `json:"checks"`
}

// GenerateReceipt creates a compliance receipt from PR approval data
func GenerateReceipt(
	deploymentID, serviceName, environment string,
	releaseVersion, gitCommitSHA, imageURI string,
	pr *PullRequest,
	reviews []Review,
	status *CheckStatus,
	changeTicketURL string,
	policyCompliant bool,
	policyChecks map[string]interface{},
) (*ComplianceReceipt, error) {
	receipt := &ComplianceReceipt{
		Version:        "1.0",
		GeneratedAt:    time.Now().UTC(),
		DeploymentID:   deploymentID,
		ServiceName:    serviceName,
		Environment:    environment,
		ReleaseVersion: releaseVersion,
		GitCommitSHA:   gitCommitSHA,
		ImageURI:       imageURI,
		PullRequest: PullRequestEvidence{
			URL:          pr.HTMLURL,
			Number:       pr.Number,
			Title:        pr.Title,
			State:        pr.State,
			MergedAt:     pr.MergedAt,
			MergeCommit:  pr.MergeCommit,
			Repository:   fmt.Sprintf("%s/%s", pr.Base.Repo.Owner.Login, pr.Base.Repo.Name),
			BaseBranch:   pr.Base.Ref,
			HeadCommitSHA: pr.Head.SHA,
		},
		Approvals:       []ApprovalEvidence{},
		ChangeTicket:    changeTicketURL,
		PolicyCompliant: policyCompliant,
		PolicyChecks:    policyChecks,
	}

	// Add approval evidence
	for _, review := range reviews {
		if review.State == "APPROVED" {
			receipt.Approvals = append(receipt.Approvals, ApprovalEvidence{
				Approver:      review.User.Login,
				ApproverEmail: review.User.Email,
				State:         review.State,
				SubmittedAt:   review.SubmittedAt,
			})
		}
	}

	// Add CI evidence
	checks := make(map[string]string)
	for _, check := range status.Statuses {
		checks[check.Context] = check.State
	}

	receipt.CIStatus = CIEvidence{
		State:       status.State,
		TotalChecks: status.TotalCount,
		Checks:      checks,
	}

	// Generate cryptographic signature
	signature, err := receipt.generateSignature()
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

	receipt.Signature = signature

	return receipt, nil
}

// generateSignature creates a SHA256 hash of the receipt data
// This provides tamper-evidence (any modification will change the signature)
func (r *ComplianceReceipt) generateSignature() (string, error) {
	// Create a copy without the signature field
	copy := *r
	copy.Signature = ""

	// Serialize to JSON (canonical format)
	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipt: %w", err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256(data)

	// Encode as base64 for storage
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// Verify checks if the receipt signature is valid
func (r *ComplianceReceipt) Verify() (bool, error) {
	expectedSignature, err := r.generateSignature()
	if err != nil {
		return false, err
	}

	return r.Signature == expectedSignature, nil
}

// ToJSON serializes the receipt to JSON format
func (r *ComplianceReceipt) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipt: %w", err)
	}

	return string(data), nil
}

// FromJSON deserializes a receipt from JSON format
func FromJSON(data string) (*ComplianceReceipt, error) {
	var receipt ComplianceReceipt
	if err := json.Unmarshal([]byte(data), &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}

	return &receipt, nil
}

// GetApproverEmails returns a list of all approver emails
func (r *ComplianceReceipt) GetApproverEmails() []string {
	emails := make([]string, 0, len(r.Approvals))
	for _, approval := range r.Approvals {
		if approval.ApproverEmail != "" {
			emails = append(emails, approval.ApproverEmail)
		}
	}
	return emails
}

// GetApprovalSummary returns a human-readable summary of approvals
func (r *ComplianceReceipt) GetApprovalSummary() string {
	if len(r.Approvals) == 0 {
		return "No approvals"
	}

	if len(r.Approvals) == 1 {
		return fmt.Sprintf("Approved by %s", r.Approvals[0].Approver)
	}

	approvers := make([]string, len(r.Approvals))
	for i, approval := range r.Approvals {
		approvers[i] = approval.Approver
	}

	return fmt.Sprintf("Approved by %d reviewers: %v", len(r.Approvals), approvers)
}
