package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/ecosystem"
)

type providerCatalogEntry struct {
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	Description    string          `json:"description,omitempty"`
	Actions        []string        `json:"actions,omitempty"`
	Scopes         []string        `json:"scopes,omitempty"`
	Readiness      any             `json:"readiness,omitempty"`
	TenantBindings []tenantBinding `json:"tenant_bindings,omitempty"`
}

type tenantBinding struct {
	Tenant             ecosystem.TenantID `json:"tenant"`
	DisplayName        string             `json:"display_name"`
	DomainSuffixes     []string           `json:"domain_suffixes"`
	DefaultSender      string             `json:"default_sender"`
	ResendRegion       string             `json:"resend_region"`
	ResendDomainStatus string             `json:"resend_domain_status,omitempty"`
}

type providerCatalogResponse struct {
	GeneratedAt string                       `json:"generated_at"`
	Providers   []providerCatalogEntry       `json:"providers"`
	Ops         []operatorCapability         `json:"ops"`
	Ecosystem   []ecosystem.TenantDefinition `json:"ecosystem_tenants"`
}

// GetAdminProvidersCatalog aggregates provider capabilities and readiness for Dispatch / app admin UIs.
func (h *Handler) GetAdminProvidersCatalog(c *gin.Context) {
	ctx := c.Request.Context()
	providers := make([]providerCatalogEntry, 0, len(providerCapabilities))
	for _, cap := range providerCapabilities {
		entry := providerCatalogEntry{
			Name:        cap.Name,
			Status:      cap.Status,
			Description: cap.Description,
			Actions:     cap.Actions,
			Scopes:      cap.Scopes,
		}
		entry.Readiness = h.providerReadinessSnapshot(ctx, cap.Name)
		if cap.Name == "resend" {
			entry.TenantBindings = h.resendTenantBindings(ctx)
		}
		providers = append(providers, entry)
	}
	c.JSON(http.StatusOK, providerCatalogResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Providers:   providers,
		Ops:         opsCapabilities,
		Ecosystem:   ecosystem.AllTenants(),
	})
}

func (h *Handler) providerReadinessSnapshot(ctx context.Context, provider string) any {
	op := "providers." + provider + ".credentials"
	switch provider {
	case "cloudflare":
		return h.handleCloudflareCredentialsReadOperation(provider, "credentials", op).Data
	case "resend":
		return h.handleResendCredentialsReadOperation(provider, "credentials", op).Data
	case "github":
		if strings.TrimSpace(h.config.GitHubToken) == "" {
			return gin.H{"configured": false, "apiKeyPresent": false}
		}
		return gin.H{"configured": true, "tokenPresent": true, "secretValuesExposed": false}
	case "porkbun":
		// Porkbun readiness is per ACCOUNT, not per estate: the global key
		// operates MADFAM's domains and cannot touch a tenant that keeps its
		// own registrar account. Reporting one boolean would tell an operator
		// "configured" while every crea operation fails, so the catalog lists
		// each registrar scope and its own readiness.
		return gin.H{
			"configured":         h.porkbunConfigured(),
			"globalConfigured":   h.porkbunConfigured(),
			"vaultClientEnabled": h.vaultClient != nil && h.vaultClient.IsEnabled(),
			"registrarScopes":    h.porkbunRegistrarScopes(ctx),
		}
	default:
		return gin.H{"configured": false}
	}
}

func (h *Handler) resendTenantBindings(ctx context.Context) []tenantBinding {
	bindings := make([]tenantBinding, 0)
	domainStatus := map[string]string{}
	if client := h.resendClient(); client != nil && client.Configured() {
		if domains, err := client.ListDomains(ctx); err == nil {
			for _, d := range domains {
				domainStatus[strings.ToLower(d.Name)] = d.Status
			}
		}
	}
	for _, t := range ecosystem.AllTenants() {
		if t.ID == ecosystem.TenantOther {
			continue
		}
		status := ""
		for _, suffix := range t.DomainSuffixes {
			if s, ok := domainStatus[strings.ToLower(suffix)]; ok {
				status = s
				break
			}
		}
		bindings = append(bindings, tenantBinding{
			Tenant:             t.ID,
			DisplayName:        t.DisplayName,
			DomainSuffixes:     t.DomainSuffixes,
			DefaultSender:      t.DefaultSenderAddress,
			ResendRegion:       t.ResendRegion,
			ResendDomainStatus: status,
		})
	}
	return bindings
}

func (h *Handler) porkbunConfigured() bool {
	return h != nil && h.config != nil && strings.TrimSpace(h.config.PorkbunAPIKey) != ""
}

// porkbunRegistrarScope is one row of the admin console's registrar table: the
// account a tenant's domains resolve to, and whether that account's credentials
// are usable. Never carries a credential value.
type porkbunRegistrarScope struct {
	Tenant      string   `json:"tenant"`
	DisplayName string   `json:"display_name"`
	Account     string   `json:"account"`
	Domains     []string `json:"domains"`
	VaultPath   string   `json:"vault_path,omitempty"`
	Configured  bool     `json:"configured"`
	Detail      string   `json:"detail,omitempty"`
}

// porkbunRegistrarScopes reports, per tenant, which Porkbun account its domains
// resolve to and whether that account can actually be operated right now.
//
// Tenants on the MADFAM account are collapsed onto the global readiness rather
// than probed individually — they all share one key pair, so probing each would
// be N identical answers. Tenant-owned accounts are resolved for real (a Vault
// read per tenant), because that is the answer an operator is looking at this
// table to get.
func (h *Handler) porkbunRegistrarScopes(ctx context.Context) []porkbunRegistrarScope {
	scopes := make([]porkbunRegistrarScope, 0)
	for _, tenant := range ecosystem.AllTenants() {
		if tenant.ID == ecosystem.TenantOther {
			continue
		}
		row := porkbunRegistrarScope{
			Tenant:      string(tenant.ID),
			DisplayName: tenant.DisplayName,
			Account:     "madfam",
			Domains:     tenant.DomainSuffixes,
			Configured:  h.porkbunConfigured(),
		}
		if !h.porkbunConfigured() {
			row.Detail = "global ENCLII_PORKBUN_* credentials are not configured"
		}
		if tenant.Registrar.IsTenantOwned() {
			req := operatorOperationRequest{Scope: map[string]string{"tenant": string(tenant.ID)}}
			client, scope := h.porkbunClientForRequest(ctx, req)
			row.Account = "tenant"
			row.VaultPath = scope.VaultPath
			row.Configured = client != nil
			row.Detail = ""
			if client == nil {
				row.Detail = porkbunScopeWarning(scope)
			}
		}
		scopes = append(scopes, row)
	}
	return scopes
}
