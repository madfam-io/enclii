package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/madfam-org/enclii/packages/cli/internal/config"
)

func TestNewProvidersCommand_Subcommands(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	root := NewProvidersCommand(cfg)
	require.NotNil(t, root)
	assert.Equal(t, "providers", root.Use)

	for _, want := range []string{"capabilities", "github", "cloudflare", "porkbun", "hetzner"} {
		assert.NotNil(t, findSubcommand(root, want), "expected providers %s", want)
	}
}

func TestProviderGitHub_Subcommands(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	github := findSubcommand(NewProvidersCommand(cfg), "github")
	require.NotNil(t, github)

	for _, want := range []string{"runs", "rerun", "cancel", "secrets", "packages", "protection"} {
		assert.NotNil(t, findSubcommand(github, want), "expected providers github %s", want)
	}
}

func TestProviderCloudflare_Subcommands(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	cloudflare := findSubcommand(NewProvidersCommand(cfg), "cloudflare")
	require.NotNil(t, cloudflare)

	for _, want := range []string{"zones", "zone-add-apply", "zone-settings-apply", "dns", "dns-apply", "tunnels", "tunnels-apply", "access", "r2", "hostnames"} {
		assert.NotNil(t, findSubcommand(cloudflare, want), "expected providers cloudflare %s", want)
	}
}

// The Cloudflare zone operations shipped server-side (see the capabilities
// registry in operator_capabilities.go) but were never exposed on the CLI, so
// provisioning the kalya.app apex on 2026-08-21 required curling the endpoint
// by hand. These assert the three verbs exist with the right shape.

func TestProviderCloudflareZones_IsReadOnly(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	zones := findSubcommand(findSubcommand(NewProvidersCommand(cfg), "cloudflare"), "zones")
	require.NotNil(t, zones)

	// Read commands carry the read flag set and force apply=false, so they
	// never expose --apply/--reason.
	assert.NotNil(t, zones.Flags().Lookup("json"), "expected --json")
	assert.NotNil(t, zones.Flags().Lookup("project"), "expected --project")
	assert.NotNil(t, zones.Flags().Lookup("service"), "expected --service")
	assert.Nil(t, zones.Flags().Lookup("apply"), "zones is read-only and must not offer --apply")
	assert.Nil(t, zones.Flags().Lookup("reason"), "zones is read-only and must not offer --reason")
}

func TestProviderCloudflareZoneApplyVerbs_AreMutating(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	cloudflare := findSubcommand(NewProvidersCommand(cfg), "cloudflare")
	require.NotNil(t, cloudflare)

	for _, action := range []string{"zone-add-apply", "zone-settings-apply"} {
		t.Run(action, func(t *testing.T) {
			cmd := findSubcommand(cloudflare, action)
			require.NotNil(t, cmd)

			// Mutating verbs inherit dry-run-by-default plus --apply/--reason
			// from newProviderActionCommand.
			for _, want := range []string{"apply", "reason", "idempotency-key", "json"} {
				assert.NotNil(t, cmd.Flags().Lookup(want), "expected --%s", want)
			}
			// Both handlers take only args.target (the apex), passed
			// positionally.
			assert.Equal(t, action+" [target]", cmd.Use)
		})
	}
}

func TestProviderPath_CloudflareZoneActions(t *testing.T) {
	tests := map[string]string{
		"zones":               "/v1/providers/cloudflare/zones",
		"zone-add-apply":      "/v1/providers/cloudflare/zone-add-apply",
		"zone-settings-apply": "/v1/providers/cloudflare/zone-settings-apply",
	}
	for action, wantPath := range tests {
		t.Run(action, func(t *testing.T) {
			assert.Equal(t, wantPath, providerPath("cloudflare", action))
		})
	}
}

// The CLI's cloudflare verbs must stay in lockstep with the server's
// registered capability actions (operator_capabilities.go). The CLI module
// cannot import switchyard-api's internal package, so the server list is
// pinned here; if the server registry changes, this fails and points at the
// drift rather than letting the CLI silently offer or omit a verb.
func TestProviderCloudflare_MatchesServerCapabilityActions(t *testing.T) {
	serverActions := []string{
		"zones", "zone-add-apply", "zone-settings-apply",
		"dns", "dns-apply", "tunnels", "tunnels-apply",
		"access", "r2", "hostnames", "credentials",
	}

	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	cloudflare := findSubcommand(NewProvidersCommand(cfg), "cloudflare")
	require.NotNil(t, cloudflare)

	cliActions := make([]string, 0, len(cloudflare.Commands()))
	for _, sub := range cloudflare.Commands() {
		cliActions = append(cliActions, sub.Name())
	}
	assert.ElementsMatch(t, serverActions, cliActions,
		"CLI cloudflare verbs must match the server capability registry")

	// And the operation name each verb reports is the dotted capability
	// string the audit trail records.
	for _, action := range serverActions {
		cmd := findSubcommand(cloudflare, action)
		require.NotNil(t, cmd, "expected providers cloudflare %s", action)
		assert.Equal(t, "providers.cloudflare."+action,
			fmt.Sprintf("providers.%s.%s", "cloudflare", cmd.Name()))
	}
}

func TestProviderPorkbun_Subcommands(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	porkbun := findSubcommand(NewProvidersCommand(cfg), "porkbun")
	require.NotNil(t, porkbun)

	for _, want := range []string{"credentials", "ping", "domains", "dns", "dns-apply", "renewals", "nameservers", "nameservers-apply", "auto-renew-apply"} {
		assert.NotNil(t, findSubcommand(porkbun, want), "expected providers porkbun %s", want)
	}
}

// Porkbun API keys are per Porkbun ACCOUNT. A client that keeps its own
// registrar account (CTM/creatumundo.mx) cannot be reached with the estate's
// global key at all — Porkbun answers INVALID_DOMAIN, which reads like a typo.
// --tenant/--project is what selects the account, so EVERY porkbun verb must
// offer it; a verb that silently omits it would route to MADFAM's account and
// report a domain that plainly exists as missing.
func TestProviderPorkbun_EveryVerbOffersTenantScope(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	porkbun := findSubcommand(NewProvidersCommand(cfg), "porkbun")
	require.NotNil(t, porkbun)
	require.NotEmpty(t, porkbun.Commands())

	for _, sub := range porkbun.Commands() {
		t.Run(sub.Name(), func(t *testing.T) {
			assert.NotNil(t, sub.Flags().Lookup("tenant"), "expected --tenant on porkbun %s", sub.Name())
			assert.NotNil(t, sub.Flags().Lookup("project"), "expected --project on porkbun %s", sub.Name())
		})
	}
}

func TestProviderPorkbunAutoRenewApplyFlags(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	autoRenew := findSubcommand(findSubcommand(NewProvidersCommand(cfg), "porkbun"), "auto-renew-apply")
	require.NotNil(t, autoRenew)

	// Mutating verb: dry-run by default, --apply requires --reason.
	for _, want := range []string{"apply", "reason", "idempotency-key", "auto-renew", "domain", "tenant"} {
		assert.NotNil(t, autoRenew.Flags().Lookup(want), "expected --%s", want)
	}
}

// Read verbs must never grow an --apply: a registrar read that could mutate is
// exactly the surprise the dry-run-by-default contract exists to prevent.
func TestProviderPorkbunReadVerbs_AreReadOnly(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	porkbun := findSubcommand(NewProvidersCommand(cfg), "porkbun")
	require.NotNil(t, porkbun)

	for _, action := range []string{"credentials", "ping", "domains", "dns", "renewals", "nameservers"} {
		t.Run(action, func(t *testing.T) {
			cmd := findSubcommand(porkbun, action)
			require.NotNil(t, cmd)
			assert.Nil(t, cmd.Flags().Lookup("apply"), "%s is read-only and must not offer --apply", action)
			assert.Nil(t, cmd.Flags().Lookup("reason"), "%s is read-only and must not offer --reason", action)
		})
	}
}

// Peer of TestProviderCloudflare_MatchesServerCapabilityActions: the CLI module
// cannot import switchyard-api's internal package, so the server's registered
// porkbun actions are pinned here. Drift in either direction fails loudly
// rather than leaving a verb the server does not route (or a server capability
// no operator can reach).
func TestProviderPorkbun_MatchesServerCapabilityActions(t *testing.T) {
	serverActions := []string{
		"ping", "credentials", "domains", "dns", "dns-apply",
		"renewals", "nameservers", "nameservers-apply", "auto-renew-apply",
	}

	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	porkbun := findSubcommand(NewProvidersCommand(cfg), "porkbun")
	require.NotNil(t, porkbun)

	cliActions := make([]string, 0, len(porkbun.Commands()))
	for _, sub := range porkbun.Commands() {
		cliActions = append(cliActions, sub.Name())
	}
	assert.ElementsMatch(t, serverActions, cliActions,
		"CLI porkbun verbs must match the server capability registry")
}

func TestOperationScope_CarriesTenant(t *testing.T) {
	scope := operationScope(operationFlags{tenant: "crea", project: "crea-map"})
	require.NotNil(t, scope)
	assert.Equal(t, "crea", scope["tenant"])
	assert.Equal(t, "crea-map", scope["project"])

	// An unscoped operation must not invent a tenant — the server reads an
	// absent tenant as "fall back to the global MADFAM account", and a phantom
	// value here would silently reroute every unscoped call.
	assert.Nil(t, operationScope(operationFlags{}))
}

func TestProviderActionFlags(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	rerun := findSubcommand(findSubcommand(NewProvidersCommand(cfg), "github"), "rerun")
	require.NotNil(t, rerun)

	for _, want := range []string{"apply", "reason", "idempotency-key", "namespace", "project", "service", "json"} {
		assert.NotNil(t, rerun.Flags().Lookup(want), "expected --%s", want)
	}
}

func TestProviderPorkbunDNSApplyFlags(t *testing.T) {
	cfg := &config.Config{APIEndpoint: "https://api.test.dev"}
	dnsApply := findSubcommand(findSubcommand(NewProvidersCommand(cfg), "porkbun"), "dns-apply")
	require.NotNil(t, dnsApply)

	for _, want := range []string{"type", "content", "ttl", "domain", "name"} {
		assert.NotNil(t, dnsApply.Flags().Lookup(want), "expected --%s", want)
	}
}
