package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/config"
	"github.com/madfam-org/enclii/apps/switchyard-api/internal/ecosystem"
)

// fakeScopeVault is a VaultSecretWriter that only needs to answer reads.
type fakeScopeVault struct {
	enabled bool
	data    map[string]map[string]interface{}
	readErr error
	// reads records every path read, so a test can prove the resolver did not
	// wander off to some other path.
	reads []string
}

func (f *fakeScopeVault) IsEnabled() bool { return f.enabled }

func (f *fakeScopeVault) MergeSecretData(_ context.Context, _ string, _ map[string]interface{}) (int, error) {
	return 0, errors.New("not used in these tests")
}

func (f *fakeScopeVault) GetSecretData(_ context.Context, path string) (map[string]interface{}, error) {
	f.reads = append(f.reads, path)
	if f.readErr != nil {
		return nil, f.readErr
	}
	if data, ok := f.data[path]; ok {
		return data, nil
	}
	return map[string]interface{}{}, nil
}

func globalPorkbunHandler() *Handler {
	return &Handler{config: &config.Config{
		PorkbunAPIKey:       "pk1_global",
		PorkbunSecretAPIKey: "sk1_global",
	}}
}

func tenantPorkbunHandler(vault *fakeScopeVault) *Handler {
	h := globalPorkbunHandler()
	h.vaultClient = vault
	return h
}

func creaVault() *fakeScopeVault {
	return &fakeScopeVault{
		enabled: true,
		data: map[string]map[string]interface{}{
			"secret/crea": {
				"porkbun_api_key":    "pk1_crea",
				"porkbun_secret_key": "sk1_crea",
			},
		},
	}
}

// The estate's default: a domain nobody claims goes to MADFAM's account.
func TestPorkbunScopeFallsBackToGlobalForUnclaimedDomain(t *testing.T) {
	h := globalPorkbunHandler()
	req := operatorOperationRequest{Args: map[string]string{"target": "npm.example.net"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client == nil {
		t.Fatal("expected the global client for an unclaimed domain")
	}
	if scope.Account != "madfam" {
		t.Fatalf("account = %q, want madfam", scope.Account)
	}
	if scope.Source != porkbunScopeSourceGlobal {
		t.Fatalf("source = %q, want %q", scope.Source, porkbunScopeSourceGlobal)
	}
}

// A MADFAM-owned domain resolves through the domain registry but still uses the
// global key — the tenant is named for the operator's benefit, not to reroute.
func TestPorkbunScopeMADFAMDomainUsesGlobalCredentials(t *testing.T) {
	h := globalPorkbunHandler()
	req := operatorOperationRequest{Args: map[string]string{"target": "npm.madfam.io"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client == nil {
		t.Fatal("expected a client for a MADFAM domain")
	}
	if scope.Account != "madfam" {
		t.Fatalf("account = %q, want madfam", scope.Account)
	}
	if scope.Tenant != ecosystem.TenantMADFAM {
		t.Fatalf("tenant = %q, want madfam", scope.Tenant)
	}
}

// The load-bearing case: creatumundo.mx lives in CTM's own Porkbun account, so
// the domain alone must reroute credential resolution to secret/crea.
func TestPorkbunScopeResolvesTenantOwnedRegistrarFromDomain(t *testing.T) {
	vault := creaVault()
	h := tenantPorkbunHandler(vault)
	req := operatorOperationRequest{Args: map[string]string{"target": "map.creatumundo.mx"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client == nil {
		t.Fatalf("expected a tenant client, scope detail: %s", scope.Detail)
	}
	if scope.Account != "tenant" {
		t.Fatalf("account = %q, want tenant", scope.Account)
	}
	if scope.Tenant != ecosystem.TenantCrea {
		t.Fatalf("tenant = %q, want crea", scope.Tenant)
	}
	if scope.Source != porkbunScopeSourceDomain {
		t.Fatalf("source = %q, want %q", scope.Source, porkbunScopeSourceDomain)
	}
	if len(vault.reads) != 1 || vault.reads[0] != "secret/crea" {
		t.Fatalf("vault reads = %#v, want exactly [secret/crea]", vault.reads)
	}
}

// --project crea-map and --tenant crea must land on the same account: an
// operator should not have to know which vocabulary a given command speaks.
func TestPorkbunScopeResolvesTenantFromProjectAndTenantFlags(t *testing.T) {
	for name, req := range map[string]operatorOperationRequest{
		"project crea-map": {Scope: map[string]string{"project": "crea-map"}},
		"project nauta":    {Scope: map[string]string{"project": "nauta"}},
		"tenant crea":      {Scope: map[string]string{"tenant": "crea"}},
		"arg tenant":       {Args: map[string]string{"tenant": "crea"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := tenantPorkbunHandler(creaVault())

			client, scope := h.porkbunClientForRequest(context.Background(), req)

			if client == nil {
				t.Fatalf("expected a tenant client, detail: %s", scope.Detail)
			}
			if scope.Tenant != ecosystem.TenantCrea || scope.Account != "tenant" {
				t.Fatalf("scope = %+v, want crea/tenant", scope)
			}
			if scope.Source != porkbunScopeSourceExplicit {
				t.Fatalf("source = %q, want %q", scope.Source, porkbunScopeSourceExplicit)
			}
		})
	}
}

// Explicit scope must beat domain inference. An operator who typed --tenant
// crea and silently reached MADFAM's account would be told the domain does not
// exist, with nothing on screen explaining why.
func TestPorkbunScopeExplicitTenantBeatsDomainInference(t *testing.T) {
	h := tenantPorkbunHandler(creaVault())
	req := operatorOperationRequest{
		Scope: map[string]string{"tenant": "crea"},
		Args:  map[string]string{"target": "npm.madfam.io"},
	}

	_, scope := h.porkbunClientForRequest(context.Background(), req)

	if scope.Tenant != ecosystem.TenantCrea || scope.Account != "tenant" {
		t.Fatalf("explicit --tenant must win, got %+v", scope)
	}
}

// The most important negative: a tenant scope whose Vault properties are absent
// must FAIL, never quietly fall back to MADFAM's key. Falling back would send
// the operation to the wrong registrar account and report INVALID_DOMAIN for a
// domain that exists.
func TestPorkbunScopeTenantWithoutCredentialsDoesNotFallBackToGlobal(t *testing.T) {
	vault := &fakeScopeVault{enabled: true, data: map[string]map[string]interface{}{}}
	h := tenantPorkbunHandler(vault)
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "crea"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client != nil {
		t.Fatal("a tenant scope with no credentials must not produce a client")
	}
	if scope.Account != "tenant" {
		t.Fatalf("account = %q, want tenant — falling back to madfam would call the wrong registrar", scope.Account)
	}
	if scope.Configured {
		t.Fatal("scope must report itself unconfigured")
	}
	if !strings.Contains(scope.Detail, "secret/crea") {
		t.Fatalf("detail should name the Vault path, got %q", scope.Detail)
	}
}

func TestPorkbunScopeTenantWithoutVaultClientFails(t *testing.T) {
	h := globalPorkbunHandler() // no vault client wired
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "crea"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client != nil {
		t.Fatal("expected no client without a Vault client")
	}
	if !strings.Contains(scope.Detail, "Vault") {
		t.Fatalf("detail should explain the missing Vault client, got %q", scope.Detail)
	}
}

func TestPorkbunScopeTenantVaultReadErrorFails(t *testing.T) {
	vault := &fakeScopeVault{enabled: true, readErr: errors.New("vault returned status 403")}
	h := tenantPorkbunHandler(vault)
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "crea"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client != nil {
		t.Fatal("a Vault read failure must not produce a client")
	}
	if !strings.Contains(scope.Detail, "403") {
		t.Fatalf("detail should carry the Vault error, got %q", scope.Detail)
	}
}

// Only one property present is not a usable pair. Porkbun authenticates with
// both fields in every request body, so a half-provisioned path would produce a
// client that fails on every call with an opaque credential error.
func TestPorkbunScopePartialCredentialsAreRejected(t *testing.T) {
	vault := &fakeScopeVault{
		enabled: true,
		data: map[string]map[string]interface{}{
			"secret/crea": {"porkbun_api_key": "pk1_crea"},
		},
	}
	h := tenantPorkbunHandler(vault)
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "crea"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client != nil {
		t.Fatal("half a credential pair must not produce a client")
	}
	if !strings.Contains(scope.Detail, "porkbun_secret_key") {
		t.Fatalf("detail should name the missing property, got %q", scope.Detail)
	}
}

// The scope is rendered into operator responses. It must carry enough to debug
// a misroute and nothing an attacker can use.
func TestPorkbunScopeDataNeverCarriesCredentialValues(t *testing.T) {
	h := tenantPorkbunHandler(creaVault())
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "crea"}}

	_, scope := h.porkbunClientForRequest(context.Background(), req)
	data := scope.asData()

	if data["tenant"] != "crea" || data["account"] != "tenant" {
		t.Fatalf("scope data lost its routing fields: %#v", data)
	}
	if data["vault_path"] != "secret/crea" {
		t.Fatalf("scope data should name the Vault path: %#v", data)
	}
	rendered := strings.ToLower(strings.Join(flattenScopeData(data), " "))
	for _, secret := range []string{"pk1_crea", "sk1_crea", "pk1_global", "sk1_global"} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("scope data leaked credential value %q: %#v", secret, data)
		}
	}
}

func flattenScopeData(data map[string]any) []string {
	out := []string{}
	for key, value := range data {
		out = append(out, key)
		switch typed := value.(type) {
		case string:
			out = append(out, typed)
		case []string:
			out = append(out, typed...)
		}
	}
	return out
}

// A non-string Vault property must read as absent rather than be stringified
// into something that gets sent to Porkbun as a credential.
func TestVaultStringPropertyIgnoresNonStrings(t *testing.T) {
	data := map[string]interface{}{"porkbun_api_key": 12345, "PORKBUN_SECRET_KEY": "sk1_upper"}

	if got := vaultStringProperty(data, "porkbun_api_key"); got != "" {
		t.Fatalf("non-string property should read as absent, got %q", got)
	}
	// Case-insensitive second pass: the estate's Vault paths are not uniformly
	// cased, and a mismatch would otherwise look exactly like "not provisioned".
	if got := vaultStringProperty(data, "porkbun_secret_key"); got != "sk1_upper" {
		t.Fatalf("case-insensitive lookup = %q, want sk1_upper", got)
	}
}

// A mistyped --tenant must NOT fall through to MADFAM's account. That fallback
// would call the wrong registrar and answer INVALID_DOMAIN for a domain that
// exists — the exact confusion this resolver removes — and it would do it for
// the single most mistypeable input in the whole operation.
func TestPorkbunScopeUnknownTenantIsRefusedNotSilentlyGlobal(t *testing.T) {
	for _, typo := range []string{"crea-tu-mundo", "creaa", "ctm"} {
		t.Run(typo, func(t *testing.T) {
			h := tenantPorkbunHandler(creaVault())
			req := operatorOperationRequest{Scope: map[string]string{"tenant": typo}}

			client, scope := h.porkbunClientForRequest(context.Background(), req)

			if client != nil {
				t.Fatalf("--tenant %q must not produce a client", typo)
			}
			if scope.Account == "madfam" {
				t.Fatalf("--tenant %q silently resolved to MADFAM's account", typo)
			}
			if scope.Account != "unknown" {
				t.Fatalf("account = %q, want unknown", scope.Account)
			}
			if !strings.Contains(scope.Detail, typo) {
				t.Fatalf("detail should quote the bad id, got %q", scope.Detail)
			}
			// The advice must not be "drop the flag" — that is precisely the
			// wrong-account call.
			next := porkbunUnconfiguredNext(scope)
			if len(next) == 0 || !strings.Contains(next[0], "correct the") {
				t.Fatalf("next steps should tell the operator to fix the id, got %#v", next)
			}
		})
	}
}

// A real tenant that simply has no registrar binding still uses the global key
// — that is the estate default, not a typo, and must keep working.
func TestPorkbunScopeKnownTenantWithoutBindingStillUsesGlobal(t *testing.T) {
	h := tenantPorkbunHandler(creaVault())
	req := operatorOperationRequest{Scope: map[string]string{"tenant": "janua"}}

	client, scope := h.porkbunClientForRequest(context.Background(), req)

	if client == nil {
		t.Fatalf("a known tenant on MADFAM's account should get the global client: %s", scope.Detail)
	}
	if scope.Account != "madfam" {
		t.Fatalf("account = %q, want madfam", scope.Account)
	}
	if scope.Tenant != ecosystem.TenantJanua {
		t.Fatalf("tenant = %q, want janua", scope.Tenant)
	}
}
