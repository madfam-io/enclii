package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cloudflare"
)

// Error mapping for cloudflare.dns-apply.
//
// Every failure below this handler used to answer HTTP 502, including the ones
// Cloudflare had already explained. An operator applying an MX record got a
// bare gateway error with no message: nothing said the record was rejected,
// nothing said why, and nothing said whether the write had landed. It read as
// a platform outage, and the operator's only recourse was the Cloudflare
// dashboard (enclii#530).
//
// A record Cloudflare refuses is a request problem, so it gets a 4xx carrying
// Cloudflare's own message. 502 is kept for what it actually means: the
// provider was unreachable or answered something we could not parse.

// cloudflareDNSApplyStatusForError maps a Cloudflare client error to the HTTP
// status the operator contract should answer with.
func cloudflareDNSApplyStatusForError(err error) int {
	if err == nil {
		return http.StatusAccepted
	}
	var apiErr *cloudflare.APIError
	if !errors.As(err, &apiErr) {
		// Transport failure, timeout, or an unparseable body: the provider
		// really is the problem.
		return http.StatusBadGateway
	}
	switch apiErr.Code {
	case 10000: // Authentication error
		return http.StatusFailedDependency
	case 9109: // Zone not found / not accessible with these credentials
		return http.StatusFailedDependency
	case 81057, 81058: // Record already exists / would conflict
		return http.StatusConflict
	default:
		// Everything else Cloudflare bothered to name is a rejected record:
		// bad content, missing or out-of-range priority, disallowed proxy
		// setting. The caller can fix all of those.
		return http.StatusUnprocessableEntity
	}
}

// cloudflareDNSApplyNextForError turns a Cloudflare rejection into the next
// step an operator can actually take, instead of leaving them to infer it.
func cloudflareDNSApplyNextForError(err error) []string {
	var apiErr *cloudflare.APIError
	if !errors.As(err, &apiErr) {
		return []string{
			"the provider did not answer cleanly — re-read the zone before retrying, because a timeout does not say whether the write landed",
			"run providers.cloudflare.dns to read the record's live state",
		}
	}
	switch apiErr.Code {
	case 81057, 81058:
		return []string{
			"a record with this content already exists at this name — re-run the dry-run to see the live set",
			"pass replace=\"true\" only if you intend to overwrite an existing record's value",
		}
	case 10000, 9109:
		return []string{
			"the Cloudflare credentials cannot see this zone — check the API token's zone scope",
		}
	default:
		return []string{
			"Cloudflare rejected the record as submitted; the message above is Cloudflare's own",
			"for MX/SRV, pass priority explicitly (e.g. priority=\"10\") rather than inside content",
		}
	}
}

// cloudflareDNSApplyPriorityArg renders the intent's priority for the client's
// int-taking create call, where a negative value means "unspecified".
func cloudflareDNSApplyPriorityArg(intent cloudflareDNSApplyIntent) int {
	if intent.Priority == nil {
		return -1
	}
	return *intent.Priority
}

// cloudflareDNSApplyDecorateIntent puts the parsed intent on the response so
// the operator can see what the server understood — chiefly that an MX
// priority typed inside content was split back out.
func cloudflareDNSApplyDecorateIntent(data map[string]any, intent cloudflareDNSApplyIntent) {
	if intent.Priority != nil {
		data["priority"] = *intent.Priority
	} else if cloudflare.RecordTypeRequiresPriority(intent.RecordType) {
		data["priority"] = cloudflare.RecordTypeDefaultPriority
		data["priority_defaulted"] = true
	}
	if intent.PriorityFromContent {
		data["priority_source"] = "content"
	} else if intent.Priority != nil {
		data["priority_source"] = "argument"
	}
}

// cloudflareDNSApplyDecoratePlan puts the plan on the response.
//
// The existing records at this name are named explicitly: an operator adding a
// second TXT needs to see the one already there, and the old response — which
// showed a single `existingRecord` and a bare mutation verb — is what let a
// destructive update pass for an additive create.
func cloudflareDNSApplyDecoratePlan(data map[string]any, plan cloudflareDNSPlan) {
	data["mutation"] = plan.Mutation
	if plan.Match != nil {
		data["existingRecord"] = plan.Match
	}
	if len(plan.Siblings) > 0 {
		data["existingRecordsAtName"] = plan.Siblings
		data["existingRecordCountAtName"] = len(plan.Siblings)
	}
}

// cloudflareDNSApplyInvalidRequest is the response for an argument the server
// refused to guess at.
func cloudflareDNSApplyInvalidRequest(operationID, operation string, dryRun bool, detail string, data map[string]any, steps []operatorOperationStep) operatorOperationResponse {
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "invalid_request",
		DryRun:      dryRun,
		Summary:     fmt.Sprintf("cloudflare.dns-apply rejected the request: %s", detail),
		Data:        data,
		Steps:       steps,
		Warnings:    []string{detail},
		Next:        []string{strings.TrimSpace("correct the argument and re-run the dry-run")},
	}
}
