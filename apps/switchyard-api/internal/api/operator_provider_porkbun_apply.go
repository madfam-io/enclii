package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/porkbun"
)

type porkbunDNSApplyIntent struct {
	Target     string
	Domain     string
	Name       string
	RecordType string
	Content    string
	TTL        string
}

type porkbunNameserversApplyIntent struct {
	Domain      string
	Nameservers []string
}

func (h *Handler) handleProviderPorkbunDNSApplyDryRun(ctx context.Context, operation string, req operatorOperationRequest) operatorOperationResponse {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunDNSApplyIntentFromRequest(req, cloudflareDNSDefaultContent(h))
	data := porkbunDNSApplyData(req, intent)
	steps := porkbunProviderSteps("load Porkbun DNS records through Enclii", "compare desired DNS record with live Porkbun state")
	if invalid := validatePorkbunDNSApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "invalid_request",
			DryRun:      true,
			Summary:     invalid,
			Data:        data,
			Steps:       steps,
			Warnings:    []string{invalid},
		}
	}

	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      true,
			Summary:     fmt.Sprintf("porkbun.dns-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}
	}

	record, records, err := findPorkbunDNSRecord(ctx, client, intent)
	if err != nil {
		return porkbunProviderReadFailed(operationID, operation, true, "failed to read Porkbun DNS records", data, steps, err)
	}
	data["recordCount"] = len(records)
	mutation := "create"
	if record != nil {
		data["existingRecord"] = record
		if porkbunRecordContentMatches(*record, intent) {
			mutation = "noop"
		} else {
			mutation = "conflict_requires_manual_update"
		}
	}
	data["mutation"] = mutation
	data["can_apply"] = mutation == "create" || mutation == "noop"
	status := "ready_to_apply"
	if mutation == "conflict_requires_manual_update" {
		status = "blocked_by_existing_record"
	}
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      status,
		DryRun:      true,
		Summary:     fmt.Sprintf("porkbun.dns-apply dry-run completed for %s", intent.Target),
		Data:        data,
		Steps:       steps,
		Next: []string{
			"rerun with --apply and a reason to execute this DNS mutation through Enclii",
			"poll providers.porkbun.dns and the public DNS resolver until the record converges",
		},
	}
}

func (h *Handler) handleProviderPorkbunDNSApply(ctx context.Context, operation string, req operatorOperationRequest) (operatorOperationResponse, int) {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunDNSApplyIntentFromRequest(req, cloudflareDNSDefaultContent(h))
	data := porkbunDNSApplyData(req, intent)
	steps := porkbunProviderSteps("load Porkbun DNS records through Enclii", "compare desired DNS record with live Porkbun state")
	steps[0].Status = "completed"
	if invalid := validatePorkbunDNSApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "invalid_request",
			DryRun:      false,
			Summary:     invalid,
			Data:        data,
			Steps:       steps,
			Warnings:    []string{invalid},
		}, http.StatusBadRequest
	}

	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      false,
			Summary:     fmt.Sprintf("porkbun.dns-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}, http.StatusServiceUnavailable
	}

	record, records, err := findPorkbunDNSRecord(ctx, client, intent)
	if err != nil {
		resp := porkbunProviderReadFailed(operationID, operation, false, "failed to read Porkbun DNS records", data, steps, err)
		return resp, http.StatusBadGateway
	}
	steps[1].Status = "completed"
	data["recordCount"] = len(records)
	if record != nil {
		data["existingRecord"] = record
		if porkbunRecordContentMatches(*record, intent) {
			data["mutation"] = "noop"
			steps[2].Status = "completed"
			steps[2].Detail = "live Porkbun DNS already matches desired state"
			steps[3].Status = "completed"
			return operatorOperationResponse{
				OperationID: operationID,
				Operation:   operation,
				Status:      "noop",
				DryRun:      false,
				Summary:     fmt.Sprintf("Porkbun DNS for %s already matches desired Enclii state", intent.Target),
				Data:        data,
				Steps:       steps,
				Next:        []string{"poll public DNS and the service health check until status converges"},
			}, http.StatusOK
		}
		data["mutation"] = "conflict_requires_manual_update"
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "blocked_by_existing_record",
			DryRun:      false,
			Summary:     fmt.Sprintf("Porkbun DNS record for %s exists with different content", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{"Porkbun update/delete support is intentionally not applied automatically in this adapter build"},
			Next:        []string{"inspect providers.porkbun.dns for the conflicting record", "replace the record through an explicit Enclii DNS update operation once available"},
		}, http.StatusConflict
	}

	changed, err := client.CreateDNSRecord(ctx, intent.Domain, porkbun.DNSRecord{
		Name:    porkbun.FlexibleString(intent.Name),
		Type:    porkbun.FlexibleString(intent.RecordType),
		Content: porkbun.FlexibleString(intent.Content),
		TTL:     porkbun.FlexibleString(intent.TTL),
	}, req.IdempotencyKey)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_apply_failed",
			DryRun:      false,
			Summary:     fmt.Sprintf("failed to create Porkbun DNS record for %s", intent.Target),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
		}, http.StatusBadGateway
	}
	data["mutation"] = "create"
	data["record"] = changed
	steps[2].Status = "completed"
	steps[2].Detail = fmt.Sprintf("created %s record through Porkbun", intent.RecordType)
	steps[3].Status = "completed"
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "succeeded",
		DryRun:      false,
		Summary:     fmt.Sprintf("created Porkbun DNS record for %s through Enclii", intent.Target),
		Data:        data,
		Steps:       steps,
		Next:        []string{"poll providers.porkbun.dns until the record is visible", "poll the public service endpoint until status.madfam.io converges"},
	}, http.StatusAccepted
}

func (h *Handler) handleProviderPorkbunNameserversApplyDryRun(ctx context.Context, operation string, req operatorOperationRequest) operatorOperationResponse {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunNameserversApplyIntentFromRequest(req)
	data := porkbunNameserversApplyData(intent)
	steps := porkbunProviderSteps("load Porkbun nameservers through Enclii", "compare desired nameservers with live registrar state")
	if invalid := validatePorkbunNameserversApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{OperationID: operationID, Operation: operation, Status: "invalid_request", DryRun: true, Summary: invalid, Data: data, Steps: steps, Warnings: []string{invalid}}
	}
	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      true,
			Summary:     fmt.Sprintf("porkbun.nameservers-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}
	}
	current, err := client.GetNameservers(ctx, intent.Domain)
	if err != nil {
		return porkbunProviderReadFailed(operationID, operation, true, "failed to read Porkbun nameservers", data, steps, err)
	}
	data["currentNameservers"] = current.Nameservers
	mutation := "update"
	if sameStringSet(current.Nameservers, intent.Nameservers) {
		mutation = "noop"
	}
	data["mutation"] = mutation
	data["can_apply"] = true
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "ready_to_apply",
		DryRun:      true,
		Summary:     fmt.Sprintf("porkbun.nameservers-apply dry-run completed for %s", intent.Domain),
		Data:        data,
		Steps:       steps,
		Next:        []string{"rerun with --apply and a reason to execute this registrar mutation through Enclii", "poll public NS records until delegation converges"},
	}
}

func (h *Handler) handleProviderPorkbunNameserversApply(ctx context.Context, operation string, req operatorOperationRequest) (operatorOperationResponse, int) {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunNameserversApplyIntentFromRequest(req)
	data := porkbunNameserversApplyData(intent)
	steps := porkbunProviderSteps("load Porkbun nameservers through Enclii", "compare desired nameservers with live registrar state")
	steps[0].Status = "completed"
	if invalid := validatePorkbunNameserversApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{OperationID: operationID, Operation: operation, Status: "invalid_request", DryRun: false, Summary: invalid, Data: data, Steps: steps, Warnings: []string{invalid}}, http.StatusBadRequest
	}
	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      false,
			Summary:     fmt.Sprintf("porkbun.nameservers-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}, http.StatusServiceUnavailable
	}
	current, err := client.GetNameservers(ctx, intent.Domain)
	if err != nil {
		resp := porkbunProviderReadFailed(operationID, operation, false, "failed to read Porkbun nameservers", data, steps, err)
		return resp, http.StatusBadGateway
	}
	steps[1].Status = "completed"
	data["currentNameservers"] = current.Nameservers
	if sameStringSet(current.Nameservers, intent.Nameservers) {
		data["mutation"] = "noop"
		steps[2].Status = "completed"
		steps[2].Detail = "live Porkbun nameservers already match desired state"
		steps[3].Status = "completed"
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "noop",
			DryRun:      false,
			Summary:     fmt.Sprintf("Porkbun nameservers for %s already match desired Enclii state", intent.Domain),
			Data:        data,
			Steps:       steps,
			Next:        []string{"poll public NS records until delegation convergence is visible"},
		}, http.StatusOK
	}
	changed, err := client.UpdateNameservers(ctx, intent.Domain, intent.Nameservers, req.IdempotencyKey)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_apply_failed",
			DryRun:      false,
			Summary:     fmt.Sprintf("failed to update Porkbun nameservers for %s", intent.Domain),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
		}, http.StatusBadGateway
	}
	data["mutation"] = "update"
	data["result"] = changed
	steps[2].Status = "completed"
	steps[2].Detail = "updated registrar nameservers through Porkbun"
	steps[3].Status = "completed"
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "succeeded",
		DryRun:      false,
		Summary:     fmt.Sprintf("updated Porkbun nameservers for %s through Enclii", intent.Domain),
		Data:        data,
		Steps:       steps,
		Next:        []string{"poll providers.porkbun.nameservers and public NS records until delegation converges", "rerun Cloudflare dns-apply once Cloudflare zone authority is visible"},
	}, http.StatusAccepted
}

func porkbunDNSApplyIntentFromRequest(req operatorOperationRequest, defaultContent string) porkbunDNSApplyIntent {
	target := operationTarget(req)
	domain := porkbunManagedDomainFromRequest(req)
	name := strings.TrimSpace(req.Args["name"])
	if name == "" {
		name = porkbunRecordName(target, domain)
	}
	recordType := strings.ToUpper(strings.TrimSpace(req.Args["type"]))
	if recordType == "" {
		recordType = "CNAME"
	}
	content := strings.TrimSpace(req.Args["content"])
	if content == "" {
		content = strings.TrimSpace(req.Args["cname"])
	}
	if content == "" {
		content = defaultContent
	}
	ttl := strings.TrimSpace(req.Args["ttl"])
	if ttl == "" {
		ttl = "600"
	}
	return porkbunDNSApplyIntent{Target: target, Domain: domain, Name: name, RecordType: recordType, Content: content, TTL: ttl}
}

func porkbunNameserversApplyIntentFromRequest(req operatorOperationRequest) porkbunNameserversApplyIntent {
	return porkbunNameserversApplyIntent{
		Domain:      porkbunManagedDomainFromRequest(req),
		Nameservers: parseNameservers(req.Args["nameservers"]),
	}
}

func porkbunManagedDomainFromRequest(req operatorOperationRequest) string {
	if req.Args != nil {
		for _, key := range []string{"domain", "apex", "zone"} {
			if value := strings.TrimSpace(req.Args[key]); value != "" {
				return strings.Trim(value, ".")
			}
		}
	}
	return porkbunManagedDomainFromTarget(operationTarget(req))
}

func porkbunManagedDomainFromTarget(target string) string {
	labels := strings.Split(strings.Trim(strings.TrimSpace(target), "."), ".")
	if len(labels) < 2 {
		return strings.Join(labels, ".")
	}
	return strings.Join(labels[len(labels)-2:], ".")
}

func porkbunRecordName(target, domain string) string {
	target = strings.Trim(strings.TrimSpace(target), ".")
	domain = strings.Trim(strings.TrimSpace(domain), ".")
	if target == "" || domain == "" || target == domain {
		return ""
	}
	suffix := "." + domain
	if strings.HasSuffix(target, suffix) {
		return strings.TrimSuffix(target, suffix)
	}
	return target
}

func validatePorkbunDNSApplyIntent(intent porkbunDNSApplyIntent) string {
	if strings.TrimSpace(intent.Target) == "" {
		return "porkbun.dns-apply requires a target domain"
	}
	if strings.TrimSpace(intent.Domain) == "" {
		return "porkbun.dns-apply requires an apex domain"
	}
	if strings.TrimSpace(intent.RecordType) == "" {
		return "porkbun.dns-apply requires a DNS record type"
	}
	if strings.TrimSpace(intent.Content) == "" {
		return "porkbun.dns-apply requires DNS record content"
	}
	if _, err := strconv.Atoi(intent.TTL); err != nil {
		return "porkbun.dns-apply ttl must be an integer number of seconds"
	}
	return ""
}

func validatePorkbunNameserversApplyIntent(intent porkbunNameserversApplyIntent) string {
	if strings.TrimSpace(intent.Domain) == "" {
		return "porkbun.nameservers-apply requires a target domain"
	}
	if len(intent.Nameservers) == 0 {
		return "porkbun.nameservers-apply requires --nameservers"
	}
	return ""
}

func findPorkbunDNSRecord(ctx context.Context, client *porkbun.Client, intent porkbunDNSApplyIntent) (*porkbun.DNSRecord, []porkbun.DNSRecord, error) {
	records, err := client.ListDNSRecords(ctx, intent.Domain)
	if err != nil {
		return nil, nil, err
	}
	for _, record := range records.Records {
		if !strings.EqualFold(record.Type.String(), intent.RecordType) {
			continue
		}
		if porkbunRecordNameMatches(record.Name.String(), intent) {
			recordCopy := record
			return &recordCopy, records.Records, nil
		}
	}
	return nil, records.Records, nil
}

func porkbunRecordNameMatches(recordName string, intent porkbunDNSApplyIntent) bool {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(recordName)), ".")
	target := strings.Trim(strings.ToLower(strings.TrimSpace(intent.Target)), ".")
	name := strings.Trim(strings.ToLower(strings.TrimSpace(intent.Name)), ".")
	if normalized == target || normalized == name {
		return true
	}
	if name == "" && normalized == strings.Trim(strings.ToLower(strings.TrimSpace(intent.Domain)), ".") {
		return true
	}
	return false
}

func porkbunRecordContentMatches(record porkbun.DNSRecord, intent porkbunDNSApplyIntent) bool {
	return strings.EqualFold(strings.TrimSpace(record.Type.String()), intent.RecordType) &&
		strings.TrimSpace(record.Content.String()) == strings.TrimSpace(intent.Content)
}

func parseNameservers(value string) []string {
	seen := map[string]bool{}
	nameservers := []string{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	}) {
		ns := strings.Trim(strings.ToLower(strings.TrimSpace(part)), ".")
		if ns == "" || seen[ns] {
			continue
		}
		seen[ns] = true
		nameservers = append(nameservers, ns)
	}
	return nameservers
}

func sameStringSet(a, b []string) bool {
	left := normalizeStringSet(a)
	right := normalizeStringSet(b)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeStringSet(values []string) []string {
	seen := map[string]bool{}
	normalized := []string{}
	for _, value := range values {
		item := strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		normalized = append(normalized, item)
	}
	sort.Strings(normalized)
	return normalized
}

func porkbunDNSApplyData(req operatorOperationRequest, intent porkbunDNSApplyIntent) map[string]any {
	return map[string]any{
		"target":    intent.Target,
		"domain":    intent.Domain,
		"name":      intent.Name,
		"type":      intent.RecordType,
		"content":   intent.Content,
		"ttl":       intent.TTL,
		"project":   strings.TrimSpace(req.Scope["project"]),
		"service":   strings.TrimSpace(req.Scope["service"]),
		"can_apply": false,
	}
}

func porkbunNameserversApplyData(intent porkbunNameserversApplyIntent) map[string]any {
	return map[string]any{
		"domain":      intent.Domain,
		"nameservers": intent.Nameservers,
		"can_apply":   false,
	}
}

func porkbunProviderSteps(loadDetail, diffDetail string) []operatorOperationStep {
	return []operatorOperationStep{
		{Name: "authorize", Status: "planned", Detail: "check caller RBAC and require reason on apply"},
		{Name: "load-state", Status: "planned", Detail: loadDetail},
		{Name: "diff", Status: "planned", Detail: diffDetail},
		{Name: "audit", Status: "planned", Detail: "record operation reason and idempotency key before mutation"},
	}
}

func porkbunProviderReadFailed(operationID, operation string, dryRun bool, summary string, data map[string]any, steps []operatorOperationStep, err error) operatorOperationResponse {
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "provider_read_failed",
		DryRun:      dryRun,
		Summary:     summary,
		Data:        data,
		Steps:       steps,
		Warnings:    []string{err.Error()},
	}
}

// --- auto-renew ------------------------------------------------------------
//
// Auto-renew is the one registrar mutation besides nameservers that a tenant
// actually needs from the platform: a client-owned domain that silently loses
// auto-renew is an outage with a 60-day fuse. Renewal itself is deliberately
// NOT wired — Porkbun's /domain/renew spends account credit and requires the
// caller to pass the exact current price in pennies, so an automated apply
// would be a money mutation gated on a price the platform would have to guess.
// Read `providers porkbun renewals` and renew in the dashboard.

type porkbunAutoRenewApplyIntent struct {
	Domain  string
	Enabled bool
	// Requested is the raw flag as typed, so an unparseable value is reported
	// rather than silently defaulting to "off" — which would be a mutation the
	// operator never asked for.
	Requested string
}

func porkbunAutoRenewApplyIntentFromRequest(req operatorOperationRequest) porkbunAutoRenewApplyIntent {
	requested := strings.TrimSpace(operationArg(req, "auto_renew", "auto-renew", "enabled", "status"))
	return porkbunAutoRenewApplyIntent{
		Domain:    porkbunManagedDomainFromRequest(req),
		Enabled:   porkbunAutoRenewEnabled(requested),
		Requested: requested,
	}
}

func porkbunAutoRenewEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "yes", "1", "enable", "enabled":
		return true
	default:
		return false
	}
}

func validatePorkbunAutoRenewApplyIntent(intent porkbunAutoRenewApplyIntent) string {
	if strings.TrimSpace(intent.Domain) == "" {
		return "porkbun.auto-renew-apply requires a target domain"
	}
	switch strings.ToLower(strings.TrimSpace(intent.Requested)) {
	case "on", "true", "yes", "1", "enable", "enabled",
		"off", "false", "no", "0", "disable", "disabled":
		return ""
	case "":
		return "porkbun.auto-renew-apply requires --auto-renew on|off"
	default:
		return fmt.Sprintf("porkbun.auto-renew-apply --auto-renew must be on or off, got %q", intent.Requested)
	}
}

func porkbunAutoRenewApplyData(intent porkbunAutoRenewApplyIntent) map[string]any {
	return map[string]any{
		"domain":    intent.Domain,
		"autoRenew": intent.Enabled,
		"can_apply": false,
	}
}

// findPorkbunDomain returns the account's record for one domain, or nil when the
// account does not hold it. Used to diff auto-renew before mutating and to tell
// "not in this account" apart from "API access not enabled".
func findPorkbunDomain(ctx context.Context, client *porkbun.Client, domain string) (*porkbun.Domain, error) {
	domains, err := client.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	for _, candidate := range domains.Domains {
		if strings.EqualFold(strings.Trim(candidate.Domain.String(), "."), strings.Trim(domain, ".")) {
			match := candidate
			return &match, nil
		}
	}
	return nil, nil
}

func (h *Handler) handleProviderPorkbunAutoRenewApplyDryRun(ctx context.Context, operation string, req operatorOperationRequest) operatorOperationResponse {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunAutoRenewApplyIntentFromRequest(req)
	data := porkbunAutoRenewApplyData(intent)
	steps := porkbunProviderSteps("load Porkbun domain state through Enclii", "compare desired auto-renew with live registrar state")
	if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{OperationID: operationID, Operation: operation, Status: "invalid_request", DryRun: true, Summary: invalid, Data: data, Steps: steps, Warnings: []string{invalid}}
	}
	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      true,
			Summary:     fmt.Sprintf("porkbun.auto-renew-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}
	}
	current, err := findPorkbunDomain(ctx, client, intent.Domain)
	if err != nil {
		return porkbunProviderReadFailed(operationID, operation, true, "failed to read Porkbun domain state", data, steps, err)
	}
	if current == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "target_not_found",
			DryRun:      true,
			Summary:     fmt.Sprintf("%s is not held by the %s Porkbun account", intent.Domain, porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{fmt.Sprintf("domain %s was not returned by domain/listAll for this credential scope", intent.Domain)},
			Next:        []string{"confirm the --tenant/--project scope names the account that holds this domain", "run providers porkbun domains for this scope to see what it does hold"},
		}
	}
	data["currentAutoRenew"] = current.AutoRenew
	data["apiAccess"] = current.APIAccess
	mutation := "update"
	if (current.AutoRenew.Int() == 1) == intent.Enabled {
		mutation = "noop"
	}
	data["mutation"] = mutation
	data["can_apply"] = true
	warnings := []string{}
	if current.APIAccess.Int() != 1 {
		warnings = append(warnings, fmt.Sprintf("API access is not enabled for %s in the owning Porkbun dashboard; apply will be refused by Porkbun", intent.Domain))
	}
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "ready_to_apply",
		DryRun:      true,
		Summary:     fmt.Sprintf("porkbun.auto-renew-apply dry-run completed for %s", intent.Domain),
		Data:        data,
		Steps:       steps,
		Warnings:    warnings,
		Next:        []string{"rerun with --apply and a reason to execute this registrar mutation through Enclii", "poll providers.porkbun.renewals until autoRenew converges"},
	}
}

func (h *Handler) handleProviderPorkbunAutoRenewApply(ctx context.Context, operation string, req operatorOperationRequest) (operatorOperationResponse, int) {
	operationID := fmt.Sprintf("op_%d", time.Now().UTC().UnixNano())
	intent := porkbunAutoRenewApplyIntentFromRequest(req)
	data := porkbunAutoRenewApplyData(intent)
	steps := porkbunProviderSteps("load Porkbun domain state through Enclii", "compare desired auto-renew with live registrar state")
	steps[0].Status = "completed"
	if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid != "" {
		return operatorOperationResponse{OperationID: operationID, Operation: operation, Status: "invalid_request", DryRun: false, Summary: invalid, Data: data, Steps: steps, Warnings: []string{invalid}}, http.StatusBadRequest
	}
	client, scope := h.porkbunClientForRequest(ctx, req)
	data["credentialScope"] = scope.asData()
	if client == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "adapter_unconfigured",
			DryRun:      false,
			Summary:     fmt.Sprintf("porkbun.auto-renew-apply cannot run until %s Porkbun API credentials are configured", porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{porkbunScopeWarning(scope)},
			Next:        porkbunUnconfiguredNext(scope),
		}, http.StatusServiceUnavailable
	}
	current, err := findPorkbunDomain(ctx, client, intent.Domain)
	if err != nil {
		resp := porkbunProviderReadFailed(operationID, operation, false, "failed to read Porkbun domain state", data, steps, err)
		return resp, http.StatusBadGateway
	}
	steps[1].Status = "completed"
	if current == nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "target_not_found",
			DryRun:      false,
			Summary:     fmt.Sprintf("%s is not held by the %s Porkbun account", intent.Domain, porkbunScopeLabel(scope)),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{fmt.Sprintf("domain %s was not returned by domain/listAll for this credential scope", intent.Domain)},
		}, http.StatusNotFound
	}
	data["currentAutoRenew"] = current.AutoRenew
	data["apiAccess"] = current.APIAccess
	if (current.AutoRenew.Int() == 1) == intent.Enabled {
		data["mutation"] = "noop"
		steps[2].Status = "completed"
		steps[2].Detail = "live Porkbun auto-renew already matches desired state"
		steps[3].Status = "completed"
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "noop",
			DryRun:      false,
			Summary:     fmt.Sprintf("Porkbun auto-renew for %s already matches desired Enclii state", intent.Domain),
			Data:        data,
			Steps:       steps,
		}, http.StatusOK
	}
	result, err := client.UpdateAutoRenew(ctx, intent.Domain, intent.Enabled, req.IdempotencyKey)
	if err != nil {
		return operatorOperationResponse{
			OperationID: operationID,
			Operation:   operation,
			Status:      "provider_apply_failed",
			DryRun:      false,
			Summary:     fmt.Sprintf("failed to update Porkbun auto-renew for %s", intent.Domain),
			Data:        data,
			Steps:       steps,
			Warnings:    []string{err.Error()},
		}, http.StatusBadGateway
	}
	data["mutation"] = "update"
	data["result"] = result.Results
	steps[2].Status = "completed"
	steps[2].Detail = fmt.Sprintf("set auto-renew=%t through Porkbun", intent.Enabled)
	steps[3].Status = "completed"
	return operatorOperationResponse{
		OperationID: operationID,
		Operation:   operation,
		Status:      "succeeded",
		DryRun:      false,
		Summary:     fmt.Sprintf("updated Porkbun auto-renew for %s through Enclii", intent.Domain),
		Data:        data,
		Steps:       steps,
		Next:        []string{"poll providers.porkbun.renewals until autoRenew converges"},
	}, http.StatusAccepted
}
