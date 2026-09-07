package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/cloudflare"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/services"
)

type cloudflareDNSApplyIntent struct {
	Target     string
	RecordType string
	Content    string
	Proxied    bool
	// Priority is the MX/SRV preference, nil when the caller specified none.
	// A pointer because 0 is a legal preference and "unset" must stay
	// distinguishable from it.
	Priority *int
	// Replace turns an add into an overwrite of an existing record with
	// different content at the same name+type. Off by default: overwriting a
	// record the operator did not name is exactly the bug this guards
	// (enclii#530).
	Replace bool
	// PriorityFromContent records that the priority was parsed off the front
	// of --content (the pre-#530 "10 mail.example.com" form) rather than
	// given explicitly, so the response can say so.
	PriorityFromContent bool
	// ParseError is a malformed argument, surfaced as a 400 rather than
	// silently defaulted.
	ParseError string
}

func (h *Handler) handleProviderCloudflareDNSApplyDryRun(ctx context.Context, operation string, req operatorOperationRequest) operatorOperationResponse {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := cloudflareDNSApplyIntentFromRequest(req, cloudflareDNSDefaultContent(h))
	allowPending := cloudflareDNSApplyAllowPending(req)
	data := map[string]any{
		"target":             intent.Target,
		"type":               intent.RecordType,
		"content":            intent.Content,
		"proxied":            intent.Proxied,
		"project":            strings.TrimSpace(req.Scope["project"]),
		"service":            strings.TrimSpace(req.Scope["service"]),
		"can_apply":          false,
		"zone_owned":         false,
		"allow_pending_zone": allowPending,
		"replace":            intent.Replace,
	}
	cloudflareDNSApplyDecorateIntent(data, intent)
	steps := []operatorOperationStep{
		{Name: "authorize", Status: "planned", Detail: "check caller RBAC and require reason on apply"},
		{Name: "load-state", Status: "planned", Detail: "load Cloudflare zone and DNS record through Enclii"},
		{Name: "diff", Status: "planned", Detail: "compare desired DNS record with live Cloudflare state"},
		{Name: "audit", Status: "planned", Detail: "record operation reason and idempotency key before mutation"},
	}
	if intent.Target == "" {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "invalid_request",
			DryRun:      true,
			Summary:     "cloudflare.dns-apply requires a target domain",
			Data:        data,
			Steps:       steps,
			Warnings:    []string{"missing args.target or scope.target"},
		}
	}
	if intent.ParseError != "" {
		return cloudflareDNSApplyInvalidRequest(operationID, operation, true, intent.ParseError, data, steps)
	}

	cfClient := h.cloudflareDNSApplyClient()
	if cfClient == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      true,
			Summary:     "cloudflare.dns-apply cannot run until the Cloudflare domain sync client is configured",
			Data:        data,
			Steps:       steps,
			Warnings:    []string{"cloudflare domain sync service is not configured"},
			Next:        []string{"set ENCLII_CLOUDFLARE_API_TOKEN, ENCLII_CLOUDFLARE_ACCOUNT_ID, ENCLII_CLOUDFLARE_ZONE_ID, and ENCLII_CLOUDFLARE_TUNNEL_ID on switchyard-api"},
		}
	}

	zone, err := cloudflareDNSApplyFindZone(ctx, cfClient, intent.Target, allowPending)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "blocked_by_dns_authority",
			DryRun:      true,
			Summary:     fmt.Sprintf("Cloudflare zone authority is not available for %s", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
			Next: []string{
				"delegate or import the apex zone into the Enclii-managed Cloudflare account",
				"pass args.allow_pending_zone=\"true\" to deliberately stage records into a created-but-not-yet-delegated zone",
				"configure the Enclii Porkbun adapter when registrar nameserver changes are required",
				"rerun this dry-run before applying DNS",
			},
		}
	}

	data["zoneID"] = zone.ID
	data["zoneName"] = zone.Name
	data["zone_status"] = zone.Status
	data["zone_owned"] = true
	pendingWarnings := cloudflareDNSApplyPendingWarnings(zone)
	live, err := cfClient.ListDNSRecordsByTypeInZone(ctx, zone.ID, intent.Target, intent.RecordType)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_read_failed",
			DryRun:      true,
			Summary:     fmt.Sprintf("failed to read Cloudflare DNS record for %s", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
		}
	}

	// The dry-run and the apply derive the plan from the same function. They
	// did not before, and that is precisely how a plan that said `create`
	// executed as an `update` that destroyed a record (enclii#530).
	plan := planCloudflareDNSApply(intent, live)
	cloudflareDNSApplyDecoratePlan(data, plan)
	data["can_apply"] = true
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "ready_to_apply",
		DryRun:      true,
		Summary: fmt.Sprintf("cloudflare.dns-apply dry-run completed for %s: %s",
			intent.Target, plan.Mutation),
		Data:     data,
		Steps:    steps,
		Warnings: append(append([]string{}, pendingWarnings...), plan.Warnings...),
		Next: []string{
			"rerun with --apply and a reason to execute the DNS mutation through Enclii",
			"poll providers.cloudflare.dns and the public DNS resolver until the record converges",
		},
	}
}

func (h *Handler) handleProviderCloudflareDNSApply(ctx context.Context, operation string, req operatorOperationRequest) (operatorOperationResponse, int) {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := cloudflareDNSApplyIntentFromRequest(req, cloudflareDNSDefaultContent(h))
	allowPending := cloudflareDNSApplyAllowPending(req)
	data := map[string]any{
		"target":             intent.Target,
		"type":               intent.RecordType,
		"content":            intent.Content,
		"proxied":            intent.Proxied,
		"project":            strings.TrimSpace(req.Scope["project"]),
		"service":            strings.TrimSpace(req.Scope["service"]),
		"allow_pending_zone": allowPending,
		"replace":            intent.Replace,
	}
	cloudflareDNSApplyDecorateIntent(data, intent)
	steps := []operatorOperationStep{
		{Name: "authorize", Status: "completed", Detail: "reason supplied and caller passed endpoint authorization"},
		{Name: "load-state", Status: "planned", Detail: "load Cloudflare zone and DNS record through Enclii"},
		{Name: "diff", Status: "planned", Detail: "compare desired DNS record with live Cloudflare state"},
		{Name: "audit", Status: "planned", Detail: "record operation reason and idempotency key before mutation"},
	}
	if intent.Target == "" {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "invalid_request",
			DryRun:      false,
			Summary:     "cloudflare.dns-apply requires a target domain",
			Data:        data,
			Steps:       steps,
			Warnings:    []string{"missing args.target or scope.target"},
		}, http.StatusBadRequest
	}
	if intent.ParseError != "" {
		return cloudflareDNSApplyInvalidRequest(operationID, operation, false, intent.ParseError, data, steps), http.StatusBadRequest
	}

	cfClient := h.cloudflareDNSApplyClient()
	if cfClient == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      false,
			Summary:     "cloudflare.dns-apply cannot run until the Cloudflare domain sync client is configured",
			Data:        data,
			Steps:       steps,
			Warnings:    []string{"cloudflare domain sync service is not configured"},
			Next:        []string{"configure the Cloudflare provider environment on switchyard-api, then retry through Enclii"},
		}, http.StatusServiceUnavailable
	}

	zone, err := cloudflareDNSApplyFindZone(ctx, cfClient, intent.Target, allowPending)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "blocked_by_dns_authority",
			DryRun:      false,
			Summary:     fmt.Sprintf("Cloudflare zone authority is not available for %s", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
			Next: []string{
				"delegate or import the apex zone into the Enclii-managed Cloudflare account",
				"pass args.allow_pending_zone=\"true\" to deliberately stage records into a created-but-not-yet-delegated zone",
				"configure and apply the Enclii Porkbun adapter if registrar nameservers must change",
			},
		}, http.StatusFailedDependency
	}

	data["zoneID"] = zone.ID
	data["zoneName"] = zone.Name
	data["zone_status"] = zone.Status
	steps[1].Status = "completed"
	pendingWarnings := cloudflareDNSApplyPendingWarnings(zone)
	live, err := cfClient.ListDNSRecordsByTypeInZone(ctx, zone.ID, intent.Target, intent.RecordType)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_read_failed",
			DryRun:      false,
			Summary:     fmt.Sprintf("failed to read Cloudflare DNS record for %s", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
		}, cloudflareDNSApplyStatusForError(err)
	}

	plan := planCloudflareDNSApply(intent, live)
	cloudflareDNSApplyDecoratePlan(data, plan)
	planWarnings := append(append([]string{}, pendingWarnings...), plan.Warnings...)

	if plan.Mutation == "noop" {
		data["record"] = plan.Match
		steps[2].Status = "completed"
		steps[2].Detail = "live Cloudflare DNS already matches desired state"
		steps[3].Status = "completed"
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "noop",
			DryRun:      false,
			Summary:     fmt.Sprintf("Cloudflare DNS for %s already matches desired Enclii state", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    planWarnings,
			Next:        []string{"poll public DNS and the service health check until status converges"},
		}, http.StatusOK
	}

	var changed any
	mutation := plan.Mutation
	if mutation == "update" {
		changed, err = cfClient.UpdateDNSRecordInZoneWithPriority(ctx, zone.ID, *plan.Match, intent.Content, intent.Proxied, intent.Priority)
	} else {
		changed, err = cfClient.CreateDNSRecordInZoneWithPriority(ctx, zone.ID, intent.Target, intent.RecordType, intent.Content, intent.Proxied, cloudflareDNSApplyPriorityArg(intent))
	}
	if err != nil {
		// A rejected record is the caller's problem, not the platform's: an
		// MX with an impossible priority or a malformed TXT is a 4xx with
		// Cloudflare's own message, never an opaque 5xx the operator has to
		// guess at (enclii#530).
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_apply_failed",
			DryRun:      false,
			Summary:     fmt.Sprintf("failed to %s Cloudflare DNS record for %s: %s", mutation, intent.Target, err.Error()),
			Data:        data,
			Steps:       steps,
			Warnings:    append(planWarnings, err.Error()),
			Next:        cloudflareDNSApplyNextForError(err),
		}, cloudflareDNSApplyStatusForError(err)
	}

	data["record"] = changed
	steps[2].Status = "completed"
	steps[2].Detail = fmt.Sprintf("%s %s record through Cloudflare", mutation, intent.RecordType)
	steps[3].Status = "completed"
	next := []string{
		"poll providers.cloudflare.dns until the record is visible",
		"poll the public service endpoint until status.madfam.io converges",
	}
	// Only a non-active ZONE makes the write inert. Plan warnings (a create
	// joining existing records, or a replace overwriting one) are about the
	// record set, not about delegation, and must not be mistaken for it.
	if len(pendingWarnings) > 0 {
		next = []string{
			"the record is staged only — it serves nothing until the registrar delegates to this zone's nameservers",
			"at cutover time: apply the registrar nameserver change, then poll public DNS until the staged records serve",
		}
	}
	summary := fmt.Sprintf("%sd Cloudflare DNS record for %s through Enclii", mutation, intent.Target)
	if mutation == "create" && len(plan.Siblings) > 0 {
		summary = fmt.Sprintf("added a %s record at %s through Enclii alongside %d existing record(s)",
			intent.RecordType, intent.Target, len(plan.Siblings))
	}
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "succeeded",
		DryRun:      false,
		Summary:     summary,
		Data:        data,
		Steps:       steps,
		Warnings:    planWarnings,
		Next:        next,
	}, http.StatusAccepted
}

// cloudflareDNSApplyAllowPending reads the strict opt-in for writing into a
// created-but-not-yet-delegated zone. Only the literal string "true"
// (case-insensitive) enables it — pre-staging records must be an explicit,
// auditable choice, never a truthy accident.
func cloudflareDNSApplyAllowPending(req operatorOperationRequest) bool {
	return strings.EqualFold(strings.TrimSpace(req.Args["allow_pending_zone"]), "true")
}

// cloudflareDNSApplyFindZone picks the zone lookup for the operation:
// the strict active-only authority check by default, or the
// pending-tolerant lookup when the caller explicitly opted in.
func cloudflareDNSApplyFindZone(ctx context.Context, cfClient *cloudflare.Client, target string, allowPending bool) (*cloudflare.Zone, error) {
	if allowPending {
		return cfClient.FindZoneForDomainIncludingPending(ctx, target)
	}
	return cfClient.FindZoneForDomain(ctx, target)
}

// cloudflareDNSApplyPendingWarnings makes a non-active zone impossible to
// miss in the response: a staged write that reads like live DNS is how a
// cutover gets called done while nothing serves.
func cloudflareDNSApplyPendingWarnings(zone *cloudflare.Zone) []string {
	if zone == nil || strings.EqualFold(zone.Status, "active") {
		return nil
	}
	return []string{fmt.Sprintf(
		"zone %s is %q, not active: this record is INERT until the registrar delegates to the zone's nameservers",
		zone.Name, zone.Status,
	)}
}

func (h *Handler) cloudflareDNSApplyClient() *cloudflare.Client {
	if h == nil || h.domainSyncService == nil {
		return nil
	}
	return h.domainSyncService.GetCloudflareClient()
}

func cloudflareDNSDefaultContent(h *Handler) string {
	if h != nil && h.domainSyncService != nil {
		return h.domainSyncService.TunnelCNAME()
	}
	return services.DefaultTunnelCNAME
}

func cloudflareDNSApplyIntentFromRequest(req operatorOperationRequest, defaultContent string) cloudflareDNSApplyIntent {
	recordType := strings.ToUpper(strings.TrimSpace(req.Args["type"]))
	if recordType == "" {
		recordType = "CNAME"
	}
	content := strings.TrimSpace(req.Args["content"])
	if content == "" {
		content = strings.TrimSpace(req.Args["cname"])
	}
	if content == "" {
		content = strings.TrimSpace(req.Args["record_content"])
	}
	if content == "" {
		content = defaultContent
	}
	intent := cloudflareDNSApplyIntent{
		Target:     operationTarget(req),
		RecordType: recordType,
		Content:    content,
		Proxied:    cloudflareDNSApplyProxied(req, recordType),
		Replace:    cloudflareDNSApplyReplace(req),
	}

	priority, err := cloudflareDNSParsePriority(req.Args["priority"])
	if err != nil {
		intent.ParseError = err.Error()
		return intent
	}
	intent.Priority = priority

	// Back-compat: before --priority existed the only way to express an MX
	// preference was "10 mail.example.com" in --content, and the runbooks
	// still say so. Parse it off the front unless an explicit priority was
	// given, in which case the explicit flag wins and the content is used
	// verbatim.
	if intent.Priority == nil {
		if rest, parsed, ok := cloudflareDNSSplitPriority(recordType, content); ok {
			intent.Content = rest
			value := parsed
			intent.Priority = &value
			intent.PriorityFromContent = true
		}
	}
	return intent
}

// cloudflareDNSApplyReplace reads the strict opt-in for overwriting an
// existing record that has DIFFERENT content at the same name+type.
//
// Strict for the same reason allow_pending_zone is: this is the flag that
// re-enables the destructive behaviour of enclii#530, so it must be a
// deliberate literal "true", never a truthy accident.
func cloudflareDNSApplyReplace(req operatorOperationRequest) bool {
	return strings.EqualFold(strings.TrimSpace(req.Args["replace"]), "true")
}

func cloudflareDNSApplyProxied(req operatorOperationRequest, recordType string) bool {
	defaultProxied := recordType == "A" || recordType == "AAAA" || recordType == "CNAME"
	value := strings.ToLower(strings.TrimSpace(req.Args["proxied"]))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(req.Args["proxy"]))
	}
	switch value {
	case "true", "1", "yes", "y", "on":
		return true
	case "false", "0", "no", "n", "off":
		return false
	default:
		return defaultProxied
	}
}
