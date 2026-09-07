package api

// Per-tenant Porkbun credential resolution.
//
// WHY THIS EXISTS. Porkbun API keys are scoped to one Porkbun ACCOUNT. Every
// Porkbun operation Enclii shipped before this file authenticated with the
// single global ENCLII_PORKBUN_API_KEY / ENCLII_PORKBUN_SECRET_API_KEY pair,
// which belongs to MADFAM's own account. That is correct for madfam.io and
// every other domain the estate itself registered — and structurally unable to
// touch a domain a client holds in the client's own Porkbun login. Porkbun does
// not answer "not yours" for such a domain; it answers INVALID_DOMAIN, which
// reads exactly like a typo. So without this resolver, "operate CTM's registrar
// through the platform API" is not a permissions problem to be granted, it is
// an impossible call.
//
// creatumundo.mx is the live case: transferred into CTM's own Porkbun account
// (2026-09-05), and the domain whose nameservers must eventually move to the
// Cloudflare zone Enclii already created for it.
//
// HOW IT RESOLVES. Every Porkbun operation asks porkbunCredentialScope which
// account it is talking to, in this order:
//
//  1. an explicit --tenant, or a --project whose slug a tenant claims;
//  2. otherwise the owning tenant of the domain the operation names, via the
//     domain-suffix registry that already routes Resend the same way;
//  3. otherwise the global MADFAM credentials.
//
// Explicit scope beating domain inference is deliberate: an operator who typed
// --tenant crea and reached MADFAM's account anyway would be told the domain
// does not exist, with nothing on screen explaining why. A scope that resolves
// to a tenant with no credentials in Vault therefore FAILS rather than silently
// falling back — the fallback is for domains nobody claims, never for a claim
// that could not be honoured.
//
// WHY VAULT AT REQUEST TIME, NOT ANOTHER ENV VAR. The Cloudflare precedent
// (h.config.CloudflareAPIToken, projected by ESO into the pod) works because
// there is exactly one Cloudflare account for the whole estate. Registrar
// accounts are per client and arrive whenever a client onboards, so an env-var
// pair per tenant would mean an ExternalSecret edit, a config field, a redeploy
// and a pod restart for each one. switchyard-api already holds a Vault client
// for exactly this shape of problem (h.vaultClient, used by the kalya feed
// provisioner to read secret/kalya at request time), so a new tenant is a Vault
// write plus a registry entry — no restart, and no new deployment surface.
//
// WHAT NEVER HAPPENS HERE. Credential VALUES are read into a local variable,
// handed to a porkbun.Client, and dropped. They are never returned to a caller,
// never logged, never placed in an operatorOperationResponse Data field, and
// never written to an audit annotation. Everything this file exposes for
// rendering is the scope METADATA — tenant id, Vault path, property names,
// whether the values were found — which is exactly what an operator needs to
// debug a misrouted operation and exactly nothing an attacker can use.

import (
	"context"
	"fmt"
	"strings"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/ecosystem"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/porkbun"
)

// porkbunScopeSource records HOW a scope was decided, so the dry-run can say so.
const (
	porkbunScopeSourceExplicit = "explicit_scope"
	porkbunScopeSourceDomain   = "domain_registry"
	porkbunScopeSourceGlobal   = "global_default"
)

// porkbunCredentialScope is the rendering-safe description of which Porkbun
// account an operation resolved to. It carries no secret values.
type porkbunCredentialScope struct {
	// Tenant is the ecosystem tenant that owns the operation, or "" when the
	// operation fell through to the global MADFAM account.
	Tenant ecosystem.TenantID
	// Account is "madfam", "tenant", or "unknown" — the last meaning the
	// scope named a tenant that does not exist, which is refused rather than
	// resolved to anything.
	Account string
	// Source is one of the porkbunScopeSource* constants.
	Source string
	// VaultPath and the property names are populated only for tenant-owned
	// accounts. Property NAMES are not secrets — they are already in
	// tenants.json and in the intake registry.
	VaultPath         string
	APIKeyProperty    string
	SecretKeyProperty string
	// Configured reports whether usable credentials were actually found.
	Configured bool
	// Detail explains an unconfigured scope in operator terms.
	Detail string
}

// asData renders the scope for an operator response. Never includes values.
func (s porkbunCredentialScope) asData() map[string]any {
	data := map[string]any{
		"account":    s.Account,
		"source":     s.Source,
		"configured": s.Configured,
	}
	if s.Tenant != "" {
		data["tenant"] = string(s.Tenant)
	}
	if s.VaultPath != "" {
		data["vault_path"] = s.VaultPath
		data["vault_properties"] = []string{s.APIKeyProperty, s.SecretKeyProperty}
	}
	if s.Detail != "" {
		data["detail"] = s.Detail
	}
	return data
}

// porkbunGlobalScope is the MADFAM-account scope.
func (h *Handler) porkbunGlobalScope(source string) porkbunCredentialScope {
	scope := porkbunCredentialScope{
		Tenant:  ecosystem.TenantMADFAM,
		Account: "madfam",
		Source:  source,
	}
	if h != nil && h.config != nil && strings.TrimSpace(h.config.PorkbunAPIKey) != "" && strings.TrimSpace(h.config.PorkbunSecretAPIKey) != "" {
		scope.Configured = true
		return scope
	}
	scope.Detail = "ENCLII_PORKBUN_API_KEY and ENCLII_PORKBUN_SECRET_API_KEY are not configured on switchyard-api"
	return scope
}

// resolvePorkbunTenant decides which tenant an operation belongs to, and how
// that decision was made. An empty tenant means "no tenant claimed it".
func resolvePorkbunTenant(req operatorOperationRequest) (ecosystem.TenantID, string) {
	if explicit := strings.TrimSpace(operationArg(req, "tenant")); explicit != "" {
		return ecosystem.TenantID(strings.ToLower(explicit)), porkbunScopeSourceExplicit
	}
	if req.Scope != nil {
		if explicit := strings.TrimSpace(req.Scope["tenant"]); explicit != "" {
			return ecosystem.TenantID(strings.ToLower(explicit)), porkbunScopeSourceExplicit
		}
		if project := strings.TrimSpace(req.Scope["project"]); project != "" {
			if tenant := ecosystem.TenantFromProject(project); tenant != ecosystem.TenantOther {
				return tenant, porkbunScopeSourceExplicit
			}
		}
	}
	if domain := porkbunManagedDomainFromRequest(req); domain != "" {
		if tenant := ecosystem.TenantFromDomain(domain); tenant != ecosystem.TenantOther {
			return tenant, porkbunScopeSourceDomain
		}
	}
	return "", porkbunScopeSourceGlobal
}

// porkbunCredentialScopeFor resolves the scope WITHOUT reading Vault. Useful
// for rendering readiness in catalogs where a Vault round-trip is unwanted;
// Configured is reported optimistically for tenant scopes (the binding exists),
// so never use it to decide whether a mutation can run — use
// porkbunClientForRequest, which proves the values are there.
func (h *Handler) porkbunCredentialScopeFor(req operatorOperationRequest) porkbunCredentialScope {
	tenant, source := resolvePorkbunTenant(req)
	if tenant == "" {
		return h.porkbunGlobalScope(porkbunScopeSourceGlobal)
	}
	// A tenant id nobody recognises is a TYPO, not a request for the global
	// account. Letting `--tenant crea-tu-mundo` fall through to MADFAM's key
	// would produce INVALID_DOMAIN for a domain that exists — the precise
	// wrong-account confusion this resolver exists to eliminate — and would do
	// it for the one input most likely to be mistyped.
	if ecosystem.TenantByID(tenant) == nil {
		return porkbunCredentialScope{
			Tenant:  tenant,
			Account: "unknown",
			Source:  source,
			Detail: fmt.Sprintf(
				"no ecosystem tenant or project named %q; check the spelling rather than rerunning unscoped, which would call MADFAM's registrar account",
				tenant,
			),
		}
	}

	binding := ecosystem.RegistrarForTenant(tenant)
	if !binding.IsTenantOwned() {
		// A known tenant whose domains sit in MADFAM's account: the global
		// credentials are the right ones, and saying so explicitly is more
		// useful than reporting the tenant as unconfigured.
		global := h.porkbunGlobalScope(source)
		global.Tenant = tenant
		return global
	}
	if !strings.EqualFold(strings.TrimSpace(binding.Provider), "porkbun") {
		return porkbunCredentialScope{
			Tenant:  tenant,
			Account: "tenant",
			Source:  source,
			Detail:  fmt.Sprintf("tenant %s declares registrar provider %q, not porkbun", tenant, binding.Provider),
		}
	}
	return porkbunCredentialScope{
		Tenant:            tenant,
		Account:           "tenant",
		Source:            source,
		VaultPath:         binding.VaultPath,
		APIKeyProperty:    binding.APIKeyProperty,
		SecretKeyProperty: binding.SecretKeyProperty,
		Configured:        true,
	}
}

// porkbunClientForRequest returns a Porkbun client bound to the credentials of
// the account the request resolved to, plus the rendering-safe scope.
//
// A nil client always comes with a scope whose Detail says why, so no caller
// has to invent an explanation. Callers MUST render scope.asData() rather than
// anything derived from the client.
func (h *Handler) porkbunClientForRequest(ctx context.Context, req operatorOperationRequest) (*porkbun.Client, porkbunCredentialScope) {
	scope := h.porkbunCredentialScopeFor(req)

	// An unrecognised tenant id never resolves to credentials — see
	// porkbunCredentialScopeFor. It is a typo, and answering it with MADFAM's
	// key is the failure mode this whole file exists to remove.
	if scope.Account == "unknown" {
		return nil, scope
	}

	if scope.Account != "tenant" {
		if !scope.Configured {
			return nil, scope
		}
		return h.porkbunProviderClient(), scope
	}

	// Tenant-owned account. Anything that goes wrong from here is a hard
	// failure: falling back to MADFAM's key would send the operation to the
	// wrong registrar account and report INVALID_DOMAIN for a domain that
	// exists.
	if scope.VaultPath == "" {
		scope.Configured = false
		return nil, scope
	}
	if h == nil || h.vaultClient == nil || !h.vaultClient.IsEnabled() {
		scope.Configured = false
		scope.Detail = "switchyard-api has no Vault client enabled, so per-tenant registrar credentials cannot be read"
		return nil, scope
	}

	data, err := h.vaultClient.GetSecretData(ctx, scope.VaultPath)
	if err != nil {
		scope.Configured = false
		// The Vault error text can name the path but never a value.
		scope.Detail = fmt.Sprintf("failed to read %s from Vault: %v", scope.VaultPath, err)
		return nil, scope
	}
	apiKey := vaultStringProperty(data, scope.APIKeyProperty)
	secretKey := vaultStringProperty(data, scope.SecretKeyProperty)
	if apiKey == "" || secretKey == "" {
		scope.Configured = false
		scope.Detail = fmt.Sprintf(
			"%s is missing %s and/or %s — run: enclii secrets intake submit %s/porkbun-registrar --reason \"...\"",
			scope.VaultPath, scope.APIKeyProperty, scope.SecretKeyProperty, scope.Tenant,
		)
		return nil, scope
	}

	baseURL := ""
	if h.config != nil {
		baseURL = h.config.PorkbunAPIBaseURL
	}
	client := porkbun.NewClient(porkbun.Config{
		APIKey:       apiKey,
		SecretAPIKey: secretKey,
		BaseURL:      baseURL,
	})
	if !client.Configured() {
		scope.Configured = false
		scope.Detail = fmt.Sprintf("credentials at %s did not produce a usable Porkbun client", scope.VaultPath)
		return nil, scope
	}
	scope.Configured = true
	return client, scope
}

// vaultStringProperty pulls one property out of a KV v2 payload. Vault returns
// `map[string]interface{}`, and a property written through a JSON path can come
// back as a non-string, so a non-string reads as absent rather than as a
// stringified struct that would be sent to Porkbun as a credential.
func vaultStringProperty(data map[string]interface{}, property string) string {
	if data == nil || strings.TrimSpace(property) == "" {
		return ""
	}
	if raw, ok := data[property]; ok {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
		return ""
	}
	// Case-insensitive second pass: the estate's Vault paths are not uniformly
	// cased, and a case mismatch here would look identical to "not provisioned".
	for key, raw := range data {
		if !strings.EqualFold(key, property) {
			continue
		}
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// porkbunScopeLabel names the account an operation resolved to, for summaries.
func porkbunScopeLabel(scope porkbunCredentialScope) string {
	if scope.Account == "unknown" {
		return fmt.Sprintf("unrecognised tenant %q's", scope.Tenant)
	}
	if scope.Account == "tenant" && scope.Tenant != "" {
		return fmt.Sprintf("tenant %s's", scope.Tenant)
	}
	return "MADFAM's"
}

// porkbunScopeWarning is the one-line reason a scope produced no client.
func porkbunScopeWarning(scope porkbunCredentialScope) string {
	if strings.TrimSpace(scope.Detail) != "" {
		return scope.Detail
	}
	return fmt.Sprintf("%s porkbun API credentials are not configured", porkbunScopeLabel(scope))
}

// porkbunUnconfiguredNext renders the operator's next steps for a scope that
// could not produce credentials.
func porkbunUnconfiguredNext(scope porkbunCredentialScope) []string {
	if scope.Account == "unknown" {
		return []string{
			"correct the --tenant/--project value; run `enclii providers capabilities` or check ecosystem/tenants.json for valid ids",
			"omit the scope entirely only if the domain really does belong to MADFAM's own registrar account",
		}
	}
	if scope.Account == "tenant" {
		return []string{
			fmt.Sprintf("run scripts/operator/porkbun-tenant-credentials.sh to load %s's registrar key pair into %s", scope.Tenant, scope.VaultPath),
			fmt.Sprintf("confirm API access is enabled for the domain in the %s Porkbun dashboard (Domain Management → Details → API Access)", scope.Tenant),
			"rerun this dry-run",
		}
	}
	return []string{"configure ENCLII_PORKBUN_API_KEY and ENCLII_PORKBUN_SECRET_API_KEY on switchyard-api through Enclii secrets, then rerun this dry-run"}
}
