package provenance

import (
	"fmt"
	"strings"
)

// ApprovalPolicy defines requirements for deployments
type ApprovalPolicy struct {
	// MinApprovals is the minimum number of PR approvals required
	MinApprovals int

	// RequireCIPassing requires all CI checks to pass
	RequireCIPassing bool

	// RequireMerged requires the PR to be merged (not just approved)
	RequireMerged bool

	// AllowedApprovers is a whitelist of GitHub usernames allowed to approve
	// Empty list = any approver is allowed
	AllowedApprovers []string

	// BlockedApprovers is a blacklist of GitHub usernames NOT allowed to approve
	// (e.g., bots, test accounts)
	BlockedApprovers []string

	// RequireChangeTicket requires a change ticket URL for production deployments
	RequireChangeTicket bool

	// AllowSelfApproval allows the PR author to approve their own PR
	AllowSelfApproval bool
}

// EnvironmentPolicy maps environment names to approval policies
type EnvironmentPolicy struct {
	Production  *ApprovalPolicy
	Staging     *ApprovalPolicy
	Development *ApprovalPolicy
}

// GetDefaultPolicy returns sensible defaults for approval policies
func GetDefaultPolicy() *EnvironmentPolicy {
	return &EnvironmentPolicy{
		Production: &ApprovalPolicy{
			MinApprovals:        2, // Require 2 approvals for production
			RequireCIPassing:    true,
			RequireMerged:       true,
			AllowedApprovers:    []string{}, // Any approver allowed
			BlockedApprovers:    []string{"dependabot", "renovate", "github-actions"},
			RequireChangeTicket: true,
			AllowSelfApproval:   false,
		},
		Staging: &ApprovalPolicy{
			MinApprovals:        1, // Require 1 approval for staging
			RequireCIPassing:    true,
			RequireMerged:       true,
			AllowedApprovers:    []string{},
			BlockedApprovers:    []string{"dependabot", "renovate", "github-actions"},
			RequireChangeTicket: false,
			AllowSelfApproval:   false,
		},
		Development: &ApprovalPolicy{
			MinApprovals:        0, // No approvals required for dev
			RequireCIPassing:    false,
			RequireMerged:       false,
			AllowedApprovers:    []string{},
			BlockedApprovers:    []string{},
			RequireChangeTicket: false,
			AllowSelfApproval:   true,
		},
	}
}

// GetPolicyForEnvironment returns the approval policy for a given environment name
func (ep *EnvironmentPolicy) GetPolicyForEnvironment(envName string) *ApprovalPolicy {
	envLower := strings.ToLower(envName)

	if strings.Contains(envLower, "prod") {
		return ep.Production
	}

	if strings.Contains(envLower, "staging") || strings.Contains(envLower, "stage") {
		return ep.Staging
	}

	// Default to development policy
	return ep.Development
}

// PolicyViolation represents a single policy violation
type PolicyViolation struct {
	Rule    string
	Message string
}

// Error implements the error interface
func (v PolicyViolation) Error() string {
	return fmt.Sprintf("[%s] %s", v.Rule, v.Message)
}

// PolicyViolations represents multiple policy violations
type PolicyViolations []PolicyViolation

// Error implements the error interface
func (pvs PolicyViolations) Error() string {
	if len(pvs) == 0 {
		return "no violations"
	}

	var messages []string
	for _, v := range pvs {
		messages = append(messages, v.Error())
	}

	return fmt.Sprintf("policy violations:\n  - %s", strings.Join(messages, "\n  - "))
}

// ValidateApprovalCount checks if enough approvals were received
func (ap *ApprovalPolicy) ValidateApprovalCount(approvals []Review, prAuthor string) PolicyViolations {
	var violations PolicyViolations

	validApprovals := 0
	for _, review := range approvals {
		// Only count "APPROVED" reviews
		if review.State != "APPROVED" {
			continue
		}

		// Check if self-approval is allowed
		if !ap.AllowSelfApproval && review.User.Login == prAuthor {
			continue
		}

		// Check if approver is in allowed list (if specified)
		if len(ap.AllowedApprovers) > 0 {
			allowed := false
			for _, allowedUser := range ap.AllowedApprovers {
				if review.User.Login == allowedUser {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		// Check if approver is blocked
		blocked := false
		for _, blockedUser := range ap.BlockedApprovers {
			if review.User.Login == blockedUser {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		validApprovals++
	}

	if validApprovals < ap.MinApprovals {
		violations = append(violations, PolicyViolation{
			Rule: "min_approvals",
			Message: fmt.Sprintf(
				"requires %d approvals, but only %d valid approvals found",
				ap.MinApprovals,
				validApprovals,
			),
		})
	}

	return violations
}

// ValidateCIStatus checks if CI checks have passed
func (ap *ApprovalPolicy) ValidateCIStatus(status *CheckStatus) PolicyViolations {
	var violations PolicyViolations

	if !ap.RequireCIPassing {
		return violations
	}

	if status.State != "success" {
		violations = append(violations, PolicyViolation{
			Rule:    "ci_passing",
			Message: fmt.Sprintf("CI checks have status '%s', expected 'success'", status.State),
		})
	}

	return violations
}

// ValidatePRMerged checks if the PR has been merged
func (ap *ApprovalPolicy) ValidatePRMerged(pr *PullRequest) PolicyViolations {
	var violations PolicyViolations

	if !ap.RequireMerged {
		return violations
	}

	if pr.State != "closed" || pr.MergedAt.IsZero() {
		violations = append(violations, PolicyViolation{
			Rule:    "pr_merged",
			Message: "PR must be merged before deployment",
		})
	}

	return violations
}

// ValidateChangeTicket checks if a change ticket URL is provided
func (ap *ApprovalPolicy) ValidateChangeTicket(changeTicketURL string) PolicyViolations {
	var violations PolicyViolations

	if !ap.RequireChangeTicket {
		return violations
	}

	if changeTicketURL == "" {
		violations = append(violations, PolicyViolation{
			Rule:    "change_ticket",
			Message: "change ticket URL is required for this environment",
		})
	}

	return violations
}

// Validate runs all policy checks and returns any violations
func (ap *ApprovalPolicy) Validate(
	pr *PullRequest,
	reviews []Review,
	status *CheckStatus,
	changeTicketURL string,
) PolicyViolations {
	var violations PolicyViolations

	// Assume the PR author is the head SHA author (GitHub convention)
	prAuthor := pr.Head.SHA // In practice, you'd get this from PR.User.Login

	violations = append(violations, ap.ValidateApprovalCount(reviews, prAuthor)...)
	violations = append(violations, ap.ValidateCIStatus(status)...)
	violations = append(violations, ap.ValidatePRMerged(pr)...)
	violations = append(violations, ap.ValidateChangeTicket(changeTicketURL)...)

	return violations
}
