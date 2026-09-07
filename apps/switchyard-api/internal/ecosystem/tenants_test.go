package ecosystem

import "testing"

func TestTenantFromDomain(t *testing.T) {
	cases := map[string]TenantID{
		"enclii.dev":          TenantEnclii,
		"api.enclii.dev":      TenantEnclii,
		"janua.dev":           TenantJanua,
		"auth.madfam.io":      TenantMADFAM,
		"unknown.example.com": TenantOther,
	}
	for domain, want := range cases {
		if got := TenantFromDomain(domain); got != want {
			t.Fatalf("TenantFromDomain(%q) = %q, want %q", domain, got, want)
		}
	}
}

func TestDefaultSenderForTenant(t *testing.T) {
	if got := DefaultSenderForTenant(TenantEnclii); got != "noreply@enclii.dev" {
		t.Fatalf("got %q", got)
	}
}

func TestTenantFromDomainResolvesCreaApex(t *testing.T) {
	// creatumundo.mx is registered in CTM's OWN Porkbun account, so this
	// mapping is what reroutes registrar credential resolution away from
	// MADFAM's global key. A regression here silently calls the wrong account.
	for _, domain := range []string{"creatumundo.mx", "map.creatumundo.mx", "erp.creatumundo.mx"} {
		if got := TenantFromDomain(domain); got != TenantCrea {
			t.Fatalf("TenantFromDomain(%q) = %q, want crea", domain, got)
		}
	}
}

func TestTenantFromProject(t *testing.T) {
	cases := map[string]TenantID{
		"crea-map": TenantCrea,
		"nauta":    TenantCrea,
		// A tenant id typed where a project slug was expected must resolve to
		// that tenant: --project crea and --tenant crea should not diverge.
		"crea":            TenantCrea,
		"CREA-MAP":        TenantCrea,
		"":                TenantOther,
		"unknown-project": TenantOther,
	}
	for project, want := range cases {
		if got := TenantFromProject(project); got != want {
			t.Fatalf("TenantFromProject(%q) = %q, want %q", project, got, want)
		}
	}
}

func TestRegistrarForTenant(t *testing.T) {
	binding := RegistrarForTenant(TenantCrea)
	if binding == nil {
		t.Fatal("crea must declare a registrar binding")
	}
	if !binding.IsTenantOwned() {
		t.Fatalf("crea's registrar must be tenant-owned, got %+v", binding)
	}
	if binding.Provider != "porkbun" || binding.VaultPath != "secret/crea" {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	if binding.APIKeyProperty != "porkbun_api_key" || binding.SecretKeyProperty != "porkbun_secret_key" {
		t.Fatalf("binding property names must match the intake registry: %+v", binding)
	}

	// Every other tenant's domains live in MADFAM's own registrar account, so
	// they must NOT declare a binding — a stray one would route their domains
	// at a Vault path that holds nothing.
	for _, id := range []TenantID{TenantMADFAM, TenantJanua, TenantEnclii, TenantSuluna, TenantPrimavera, TenantOther} {
		if RegistrarForTenant(id) != nil {
			t.Fatalf("tenant %q should not declare a registrar binding", id)
		}
	}
}

func TestRegistrarBindingIsTenantOwnedRequiresEveryField(t *testing.T) {
	// A partially filled binding must not be treated as tenant-owned: it would
	// route to a Vault path or property that does not exist and fail every
	// operation with an unexplained "credentials missing".
	full := RegistrarBinding{Provider: "porkbun", Account: "tenant", VaultPath: "secret/x", APIKeyProperty: "k", SecretKeyProperty: "s"}
	if !full.IsTenantOwned() {
		t.Fatal("a complete tenant binding should be tenant-owned")
	}
	var nilBinding *RegistrarBinding
	if nilBinding.IsTenantOwned() {
		t.Fatal("a nil binding is never tenant-owned")
	}
	for name, mutate := range map[string]func(*RegistrarBinding){
		"no account":    func(b *RegistrarBinding) { b.Account = "madfam" },
		"no vault path": func(b *RegistrarBinding) { b.VaultPath = "" },
		"no api key":    func(b *RegistrarBinding) { b.APIKeyProperty = "" },
		"no secret key": func(b *RegistrarBinding) { b.SecretKeyProperty = "" },
	} {
		t.Run(name, func(t *testing.T) {
			partial := full
			mutate(&partial)
			if partial.IsTenantOwned() {
				t.Fatalf("%s must not be tenant-owned: %+v", name, partial)
			}
		})
	}
}
