# Security Remediation Report - 2026-01-11

## Summary

Security audit completed to identify and remove hardcoded credentials from the codebase.

## Findings & Fixes

### Critical Issues Fixed

| File | Issue | Fix Applied |
|------|-------|-------------|
| `k8s/auto-claude-storage.yaml:151` | Janua client secret hardcoded | Replaced with placeholder |
| `docs/SSO_DEPLOYMENT_INSTRUCTIONS.md` | Client ID/Secret in docs | Replaced with env var references |
| `docs/archive/.../SWE_AGENT_PROMPT_FEATURE_PARITY_V3.md` | GitHub webhook secret | Replaced with placeholder |

### Files Not Tracked (OK)

| File | Status |
|------|--------|
| `infra/k8s/production/oidc-secrets.local.yaml` | Correctly gitignored by `*-secrets.local.yaml` |

### Acceptable for Development

| File | Notes |
|------|-------|
| `infra/k8s/base/secrets.dev.yaml` | Dev-only with clear warnings, tracked intentionally |
| `docker-compose.yml` | Local dev database URLs |
| `packages/cli/internal/cmd/local.go` | Local dev database URLs |

## Secrets Requiring Rotation

The following secrets were exposed in git history and **should be rotated**:

### High Priority

1. **Janua Client Secret** (from `k8s/auto-claude-storage.yaml`)
   - Value: `jns_3SRnFv5IF32bM3fkHH5bFQ3su9LlLJB3zqlvKbwIVdnqJ5paKc4u7DfMhg10ZTsc`
   - Action: Create new OAuth client in Janua admin, update Kubernetes secrets

2. **Enclii OIDC Client Secret** (from `docs/SSO_DEPLOYMENT_INSTRUCTIONS.md`)
   - Value: `jns_4mZiokDmPjT78ZwuoyLanIdW7vz1v1xy1aBbQ_o2G_xZWL1amozmVmXtl28fYcoM`
   - Action: Create new OAuth client in Janua admin, update `enclii-oidc-credentials` secret

### Medium Priority

3. **GitHub Webhook Secret** (from archive docs)
   - Value: `0a619aa7b0bf6b1bf75e252dacfc02a2afac33e4ccbe19a9ff0077bdc9d33508`
   - Action: Generate new secret, update GitHub webhook and K8s secret

## Git History Considerations

The exposed secrets remain in git history. Options:

1. **Rotate secrets** (Recommended) - Simplest approach, invalidates exposed values
2. **git-filter-repo** - Rewrite history to remove (causes force-push issues for all contributors)
3. **BFG Repo-Cleaner** - Faster alternative to filter-repo

Recommendation: **Rotate the secrets** rather than rewriting git history.

## Preventive Measures

### Already in Place

- `.gitignore` patterns for `*-secrets.local.yaml`, `secrets.*.yaml`
- Template files with `.TEMPLATE` extension

### Recommended Additions

1. **Pre-commit hook**: Scan for patterns like `jns_`, `jnc_`, `ghp_`, `ghs_`
2. **GitHub Secret Scanning**: Enable in repository settings
3. **Sealed Secrets or External Secrets Operator**: For production secret management

## Verification Commands

```bash
# Check for remaining hardcoded secrets
grep -r "jns_[a-zA-Z0-9]\{20,\}" --include="*.yaml" --include="*.md" --include="*.go"
grep -r "jnc_[a-zA-Z0-9]\{20,\}" --include="*.yaml" --include="*.md" --include="*.go"

# Verify gitignore is working
git status infra/k8s/production/oidc-secrets.local.yaml
```
