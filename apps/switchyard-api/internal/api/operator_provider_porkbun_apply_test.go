package api

import "testing"

func TestPorkbunDNSApplyIntentFromRequestDerivesApexAndRelativeName(t *testing.T) {
	req := operatorOperationRequest{
		Scope: map[string]string{"project": "phynd", "service": "app"},
		Args:  map[string]string{"target": "crm.phynd.app", "content": "example.cfargotunnel.com"},
	}

	intent := porkbunDNSApplyIntentFromRequest(req, "default.cfargotunnel.com")

	if intent.Target != "crm.phynd.app" {
		t.Fatalf("target = %q, want crm.phynd.app", intent.Target)
	}
	if intent.Domain != "phynd.app" {
		t.Fatalf("domain = %q, want phynd.app", intent.Domain)
	}
	if intent.Name != "crm" {
		t.Fatalf("name = %q, want crm", intent.Name)
	}
	if intent.RecordType != "CNAME" {
		t.Fatalf("record type = %q, want CNAME", intent.RecordType)
	}
	if intent.Content != "example.cfargotunnel.com" {
		t.Fatalf("content = %q, want example.cfargotunnel.com", intent.Content)
	}
	if intent.TTL != "600" {
		t.Fatalf("ttl = %q, want 600", intent.TTL)
	}
}

func TestPorkbunDNSApplyIntentAllowsExplicitApexAndName(t *testing.T) {
	req := operatorOperationRequest{
		Args: map[string]string{
			"target":  "crm.madfam.io",
			"domain":  "madfam.io",
			"name":    "crm",
			"type":    "CNAME",
			"content": "tenant-router.example.net",
			"ttl":     "300",
		},
	}

	intent := porkbunDNSApplyIntentFromRequest(req, "default.cfargotunnel.com")

	if intent.Domain != "madfam.io" || intent.Name != "crm" || intent.TTL != "300" {
		t.Fatalf("unexpected explicit intent: %+v", intent)
	}
	if invalid := validatePorkbunDNSApplyIntent(intent); invalid != "" {
		t.Fatalf("intent should be valid: %s", invalid)
	}
}

func TestPorkbunNameserverParsingNormalizesAndDedupes(t *testing.T) {
	got := parseNameservers("NS1.EXAMPLE.COM., ns2.example.com ns1.example.com")
	want := []string{"ns1.example.com", "ns2.example.com"}

	if len(got) != len(want) {
		t.Fatalf("nameservers = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("nameservers[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPorkbunRecordNameMatchesFullAndRelativeNames(t *testing.T) {
	intent := porkbunDNSApplyIntent{Target: "crm.phynd.app", Domain: "phynd.app", Name: "crm", RecordType: "CNAME"}

	if !porkbunRecordNameMatches("crm", intent) {
		t.Fatal("relative record name should match")
	}
	if !porkbunRecordNameMatches("crm.phynd.app", intent) {
		t.Fatal("fully qualified record name should match")
	}
	if porkbunRecordNameMatches("www.phynd.app", intent) {
		t.Fatal("different record name should not match")
	}
}

func TestSameStringSetIgnoresOrderAndCase(t *testing.T) {
	left := []string{"NS2.EXAMPLE.COM.", "ns1.example.com"}
	right := []string{"ns1.example.com.", "ns2.example.com"}

	if !sameStringSet(left, right) {
		t.Fatalf("sets should match: %#v %#v", left, right)
	}
}

func TestPorkbunAutoRenewIntentParsesOnAndOff(t *testing.T) {
	for _, value := range []string{"on", "true", "yes", "1", "enable", "enabled", "ON"} {
		req := operatorOperationRequest{Args: map[string]string{"target": "creatumundo.mx", "auto_renew": value}}
		intent := porkbunAutoRenewApplyIntentFromRequest(req)
		if !intent.Enabled {
			t.Fatalf("auto_renew=%q should parse as enabled", value)
		}
		if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid != "" {
			t.Fatalf("auto_renew=%q should be valid: %s", value, invalid)
		}
	}
	for _, value := range []string{"off", "false", "no", "0", "disable", "disabled"} {
		req := operatorOperationRequest{Args: map[string]string{"target": "creatumundo.mx", "auto_renew": value}}
		intent := porkbunAutoRenewApplyIntentFromRequest(req)
		if intent.Enabled {
			t.Fatalf("auto_renew=%q should parse as disabled", value)
		}
		if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid != "" {
			t.Fatalf("auto_renew=%q should be valid: %s", value, invalid)
		}
	}
}

// An unparseable or absent --auto-renew must be REFUSED, not defaulted. Both
// would default to "off", which is a mutation that turns off auto-renew on a
// live client domain — the exact outage this operation exists to prevent.
func TestPorkbunAutoRenewIntentRefusesAmbiguousValues(t *testing.T) {
	for _, value := range []string{"", "maybe", "onn", "2"} {
		req := operatorOperationRequest{Args: map[string]string{"target": "creatumundo.mx", "auto_renew": value}}
		intent := porkbunAutoRenewApplyIntentFromRequest(req)
		if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid == "" {
			t.Fatalf("auto_renew=%q must be rejected, not defaulted to off", value)
		}
	}
}

func TestPorkbunAutoRenewIntentRequiresDomain(t *testing.T) {
	intent := porkbunAutoRenewApplyIntentFromRequest(operatorOperationRequest{Args: map[string]string{"auto_renew": "on"}})
	if invalid := validatePorkbunAutoRenewApplyIntent(intent); invalid == "" {
		t.Fatal("an auto-renew intent with no domain must be rejected")
	}
}

// The apex derivation must survive a two-label .mx apex, which is what
// creatumundo.mx is — a regression here would send DNS writes to "mundo.mx".
func TestPorkbunManagedDomainFromTargetHandlesCreaApex(t *testing.T) {
	if got := porkbunManagedDomainFromTarget("map.creatumundo.mx"); got != "creatumundo.mx" {
		t.Fatalf("apex = %q, want creatumundo.mx", got)
	}
	if got := porkbunRecordName("map.creatumundo.mx", "creatumundo.mx"); got != "map" {
		t.Fatalf("record name = %q, want map", got)
	}
}
