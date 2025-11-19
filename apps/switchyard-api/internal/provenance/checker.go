package provenance

import (
	"context"
	"fmt"
	"time"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Checker verifies PR approvals before allowing deployments
type Checker struct {
	githubClient *GitHubClient
	policy       *EnvironmentPolicy
}

// NewChecker creates a new approval checker
func NewChecker(githubToken string, policy *EnvironmentPolicy) *Checker {
	if policy == nil {
		policy = GetDefaultPolicy()
	}

	return &Checker{
		githubClient: NewGitHubClient(githubToken),
		policy:       policy,
	}
}

// ApprovalResult represents the result of an approval check
type ApprovalResult struct {
	// Approved indicates if the deployment is approved
	Approved bool

	// Violations contains any policy violations found
	Violations PolicyViolations

	// Receipt is the compliance receipt (generated even if not approved)
	Receipt *ComplianceReceipt

	// PR metadata
	PRNumber      int
	PRURL         string
	ApproverEmail string
	ApproverName  string
	ApprovedAt    time.Time
	CIStatus      string
}

// CheckDeploymentApproval verifies that a deployment meets approval requirements
// This is the main entry point for pre-deployment checks
func (c *Checker) CheckDeploymentApproval(
	ctx context.Context,
	deployment *types.Deployment,
	release *types.Release,
	service *types.Service,
	environmentName string,
	changeTicketURL string,
) (*ApprovalResult, error) {
	// Get the appropriate policy for this environment
	policy := c.policy.GetPolicyForEnvironment(environmentName)

	// If development environment with no approval requirements, skip checks
	if policy.MinApprovals == 0 && !policy.RequireCIPassing && !policy.RequireMerged {
		return &ApprovalResult{
			Approved:   true,
			Violations: nil,
			CIStatus:   "skipped",
		}, nil
	}

	// Extract GitHub repository info from git_repo URL
	owner, repo, err := parseGitRepoURL(service.GitRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse git repository URL: %w", err)
	}

	// Find the PR for this commit
	pr, err := c.githubClient.FindPRByCommit(ctx, owner, repo, release.GitSHA)
	if err != nil {
		// If no PR found and policy requires it, fail
		if policy.RequireMerged {
			return &ApprovalResult{
				Approved: false,
				Violations: PolicyViolations{
					{Rule: "pr_required", Message: fmt.Sprintf("no PR found for commit %s", release.GitSHA)},
				},
			}, nil
		}

		// Otherwise, allow deployment without PR
		return &ApprovalResult{
			Approved:   true,
			Violations: nil,
			CIStatus:   "no_pr",
		}, nil
	}

	// Get PR reviews
	reviews, err := c.githubClient.GetPRReviews(ctx, owner, repo, pr.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR reviews: %w", err)
	}

	// Get CI check status
	status, err := c.githubClient.GetCheckStatus(ctx, owner, repo, release.GitSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CI status: %w", err)
	}

	// Run policy validation
	violations := policy.Validate(pr, reviews, status, changeTicketURL)

	// Determine if approved
	approved := len(violations) == 0

	// Generate compliance receipt
	policyChecks := map[string]interface{}{
		"min_approvals":         policy.MinApprovals,
		"require_ci_passing":    policy.RequireCIPassing,
		"require_merged":        policy.RequireMerged,
		"require_change_ticket": policy.RequireChangeTicket,
		"environment":           environmentName,
	}

	receipt, err := GenerateReceipt(
		deployment.ID,
		service.Name,
		environmentName,
		release.Version,
		release.GitSHA,
		release.ImageURI,
		pr,
		reviews,
		status,
		changeTicketURL,
		approved,
		policyChecks,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate compliance receipt: %w", err)
	}

	// Extract approver information (first approver)
	var approverEmail, approverName string
	var approvedAt time.Time
	if len(reviews) > 0 {
		for _, review := range reviews {
			if review.State == "APPROVED" {
				approverEmail = review.User.Email
				approverName = review.User.Name
				approvedAt = review.SubmittedAt
				break
			}
		}
	}

	return &ApprovalResult{
		Approved:      approved,
		Violations:    violations,
		Receipt:       receipt,
		PRNumber:      pr.Number,
		PRURL:         pr.HTMLURL,
		ApproverEmail: approverEmail,
		ApproverName:  approverName,
		ApprovedAt:    approvedAt,
		CIStatus:      status.State,
	}, nil
}

// parseGitRepoURL extracts owner and repo from a GitHub repository URL
// Supports formats:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - git@github.com:owner/repo.git
func parseGitRepoURL(repoURL string) (owner, repo string, err error) {
	// Simple regex-based parsing (in production, use a proper git URL parser)
	// For now, assume HTTPS format: https://github.com/owner/repo
	var url string
	if len(repoURL) > 19 && repoURL[:19] == "https://github.com/" {
		url = repoURL[19:]
	} else if len(repoURL) > 15 && repoURL[:15] == "git@github.com:" {
		url = repoURL[15:]
	} else {
		return "", "", fmt.Errorf("unsupported git URL format: %s", repoURL)
	}

	// Remove .git suffix if present
	if len(url) > 4 && url[len(url)-4:] == ".git" {
		url = url[:len(url)-4]
	}

	// Split by /
	parts := make([]string, 0)
	current := ""
	for _, c := range url {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid git URL format: %s", repoURL)
	}

	return parts[0], parts[1], nil
}

// VerifyReceipt checks if a compliance receipt signature is valid
func VerifyReceipt(receiptJSON string) (bool, error) {
	receipt, err := FromJSON(receiptJSON)
	if err != nil {
		return false, fmt.Errorf("failed to parse receipt: %w", err)
	}

	return receipt.Verify()
}
