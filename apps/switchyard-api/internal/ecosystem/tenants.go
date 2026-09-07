package ecosystem

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// TenantID identifies a MADFAM ecosystem slice.
type TenantID string

const (
	TenantMADFAM    TenantID = "madfam"
	TenantJanua     TenantID = "janua"
	TenantEnclii    TenantID = "enclii"
	TenantSuluna    TenantID = "suluna"
	TenantPrimavera TenantID = "primavera"
	TenantCrea      TenantID = "crea"
	TenantOther     TenantID = "other"
)

// RegistrarBinding says WHOSE registrar account holds a tenant's domains, and
// where the credentials for that account live.
//
// The default for the estate is that MADFAM's own Porkbun account holds the
// domain, and switchyard-api's global ENCLII_PORKBUN_* credentials operate it.
// A client tenant that keeps its own registrar account (CTM owns
// creatumundo.mx under its own Porkbun login) cannot be operated with those
// credentials at all: Porkbun API keys are per account, so the global key sees
// no such domain and answers INVALID_DOMAIN. Declaring a binding here is what
// tells the credential resolver to read that tenant's own key pair out of Vault
// instead of falling back to the global one.
//
// No values live here — only the Vault path and the property names. The key
// pair itself reaches Vault through the `crea/porkbun-registrar` secret intake
// target, so no human, agent, or terminal scrollback ever holds a copy.
type RegistrarBinding struct {
	// Provider is the registrar this binding describes. Only "porkbun" is
	// resolved today; the field exists so a second registrar does not need a
	// second shape.
	Provider string `json:"provider"`
	// Account is "madfam" when the estate's own account holds the domain and
	// "tenant" when the client does. Only "tenant" changes credential
	// resolution; "madfam" is the documented default and needs no entry.
	Account string `json:"account"`
	// VaultPath is the KV v2 path holding the tenant's registrar credentials.
	VaultPath string `json:"vaultPath"`
	// APIKeyProperty and SecretKeyProperty name the two properties at that
	// path. Named explicitly rather than assumed, because the estate's Vault
	// paths are not uniformly cased (see secretsintake/registry.yaml).
	APIKeyProperty    string `json:"apiKeyProperty"`
	SecretKeyProperty string `json:"secretKeyProperty"`
}

// IsTenantOwned reports whether this binding routes to the tenant's own
// registrar account rather than MADFAM's.
func (r *RegistrarBinding) IsTenantOwned() bool {
	return r != nil &&
		strings.EqualFold(strings.TrimSpace(r.Account), "tenant") &&
		strings.TrimSpace(r.VaultPath) != "" &&
		strings.TrimSpace(r.APIKeyProperty) != "" &&
		strings.TrimSpace(r.SecretKeyProperty) != ""
}

// TenantDefinition describes one ecosystem tenant slice.
type TenantDefinition struct {
	ID                   TenantID `json:"id"`
	DisplayName          string   `json:"displayName"`
	DomainSuffixes       []string `json:"domainSuffixes"`
	DefaultSenderDomain  string   `json:"defaultSenderDomain"`
	DefaultSenderAddress string   `json:"defaultSenderAddress"`
	ResendRegion         string   `json:"resendRegion"`
	// Projects lists the Enclii project slugs that belong to this tenant. It is
	// what lets an operator scope an operation with the `--project` flag the
	// CLI already carries, instead of learning a second vocabulary.
	Projects []string `json:"projects,omitempty"`
	// Registrar is nil for every tenant whose domains sit in MADFAM's own
	// registrar account.
	Registrar *RegistrarBinding `json:"registrar,omitempty"`
}

type tenantRegistry struct {
	Tenants []TenantDefinition `json:"tenants"`
}

//go:embed tenants.json
var tenantsJSON []byte

var registry tenantRegistry

func init() {
	if err := json.Unmarshal(tenantsJSON, &registry); err != nil {
		panic("ecosystem: invalid tenants.json: " + err.Error())
	}
}

// AllTenants returns every tenant definition including "other".
func AllTenants() []TenantDefinition {
	out := make([]TenantDefinition, len(registry.Tenants))
	copy(out, registry.Tenants)
	return out
}

// TenantByID returns a tenant definition or nil.
func TenantByID(id TenantID) *TenantDefinition {
	for i := range registry.Tenants {
		if registry.Tenants[i].ID == id {
			t := registry.Tenants[i]
			return &t
		}
	}
	return nil
}

// TenantFromDomain infers the ecosystem tenant from a hostname or apex domain.
func TenantFromDomain(domain string) TenantID {
	normalized := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	for _, tenant := range registry.Tenants {
		if tenant.ID == TenantOther {
			continue
		}
		for _, suffix := range tenant.DomainSuffixes {
			if normalized == suffix || strings.HasSuffix(normalized, "."+suffix) {
				return tenant.ID
			}
		}
	}
	return TenantOther
}

// TenantFromProject infers the ecosystem tenant from an Enclii project slug.
//
// Returns TenantOther when no tenant claims the project, so a caller can tell
// "unknown project" from "a project that belongs to the default estate".
func TenantFromProject(project string) TenantID {
	normalized := strings.ToLower(strings.TrimSpace(project))
	if normalized == "" {
		return TenantOther
	}
	for _, tenant := range registry.Tenants {
		if tenant.ID == TenantOther {
			continue
		}
		// A tenant id typed where a project slug was expected resolves to that
		// tenant. `--project crea` and `--tenant crea` should not diverge.
		if strings.EqualFold(string(tenant.ID), normalized) {
			return tenant.ID
		}
		for _, slug := range tenant.Projects {
			if strings.EqualFold(strings.TrimSpace(slug), normalized) {
				return tenant.ID
			}
		}
	}
	return TenantOther
}

// RegistrarForTenant returns the tenant's registrar binding, or nil when its
// domains live in MADFAM's own registrar account.
func RegistrarForTenant(id TenantID) *RegistrarBinding {
	t := TenantByID(id)
	if t == nil || t.Registrar == nil {
		return nil
	}
	binding := *t.Registrar
	return &binding
}

// DomainsForTenant returns known apex domains for a tenant.
func DomainsForTenant(id TenantID) []string {
	if t := TenantByID(id); t != nil {
		return append([]string(nil), t.DomainSuffixes...)
	}
	return nil
}

// DefaultSenderForTenant returns noreply@ style address for a tenant.
func DefaultSenderForTenant(id TenantID) string {
	if t := TenantByID(id); t != nil {
		return t.DefaultSenderAddress
	}
	return ""
}

// ResendRegionForDomain returns the Resend region for a domain (defaults us-east-1).
func ResendRegionForDomain(domain string) string {
	if t := TenantByID(TenantFromDomain(domain)); t != nil && t.ResendRegion != "" {
		return t.ResendRegion
	}
	return "us-east-1"
}
