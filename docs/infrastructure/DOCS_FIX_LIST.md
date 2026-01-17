# Documentation Fix List

> **Generated**: 2026-01-17 | **Audit Type**: Full-Stack Recovery Docs Sync

## Executive Summary

| Category | Count | Severity |
|----------|-------|----------|
| **Port Mismatch** | 3 files | CRITICAL |
| **Runtime Drift Documentation** | 1 file | HIGH |
| **Stale Infrastructure References** | 2 files | MEDIUM |
| **Outdated Localhost References** | OK | LOW (dev context) |
| **OIDC URL References** | 21 files | OK (current) |

---

## CRITICAL: Port Mismatch (4200 vs 8080)

### Issue
Documentation claims switchyard-api runs on **port 4200** per PORT_ALLOCATION.md, but actual K8s manifests and implementation use **port 8080**.

### Evidence

| Source | Port | Status |
|--------|------|--------|
| `CLAUDE.md` Port Allocation table | 4200 | Documented |
| `apps/switchyard-api/Dockerfile` EXPOSE | 4200 | Implementation |
| `apps/switchyard-api/Dockerfile` HEALTHCHECK | 4200 | Implementation |
| `infra/k8s/base/switchyard-api.yaml` ENCLII_PORT | 8080 | K8s Manifest |
| `infra/k8s/base/switchyard-api.yaml` containerPort | 8080 | K8s Manifest |
| `infra/k8s/base/switchyard-api.yaml` Service port | 8080 | K8s Manifest |
| `infra/k8s/production/cloudflared.yaml` | 8080 | Tunnel Route |
| `infra/k8s/README.md` | 4200 | Documented |

### Root Cause
The PORT_ALLOCATION.md defines *intended* port allocations, but the K8s manifests were implemented with 8080 (common default).

### Resolution Options

**Option A: Update Manifests to 4200** (Recommended)
- Aligns with documented architecture
- Requires coordinated update of:
  - `infra/k8s/base/switchyard-api.yaml`
  - `infra/k8s/production/cloudflared.yaml`
  - `infra/k8s/production/environment-patch.yaml`
  - ArgoCD sync after changes

**Option B: Update Documentation to 8080**
- Less disruptive to running infrastructure
- Requires updating:
  - `CLAUDE.md`
  - `infra/k8s/README.md`
  - PORT_ALLOCATION.md reference

### Files to Update (Option A)

```yaml
# infra/k8s/base/switchyard-api.yaml
# Change: ENCLII_PORT: "8080" → "4200"
# Change: containerPort: 8080 → 4200
# Change: service port: 8080 → 4200

# infra/k8s/production/cloudflared.yaml
# Change: service: http://switchyard-api:8080 → http://switchyard-api:4200

# infra/k8s/production/environment-patch.yaml
# Change: readinessProbe port: 8080 → 4200
# Change: livenessProbe port: 8080 → 4200
```

---

## HIGH: Runtime Drift Documentation

### Issue
`INFRA_ANATOMY.md` documents production state that should be fixed, not perpetuated.

### Specific Items

| Line | Content | Action |
|------|---------|--------|
| L212 | `ENCLII_REDIS_URL = 95.217.198.239:6379` | Document as **BUG** not as current state |
| L41-43 | Systemd cloudflared services | Add "TO BE DISABLED" marker |
| L27 | Public IP reference | OK (informational) |

### Recommended Updates

```markdown
# In INFRA_ANATOMY.md, line 212:
# BEFORE:
| switchyard-api | ENCLII_REDIS_URL | **95.217.198.239:6379** | EXTERNAL |

# AFTER:
| switchyard-api | ENCLII_REDIS_URL | **95.217.198.239:6379** | **BUG** - Should be `redis://redis.data.svc.cluster.local:6379` |
```

---

## MEDIUM: Stale Infrastructure References

### infra/k8s/README.md

| Line | Issue | Fix |
|------|-------|-----|
| L84 | `port-forward svc/switchyard-api 4200:4200` | Should match actual port |
| L95 | Port 4200 in table | Should match actual port |
| L250 | Tunnel service example uses 4200 | Should match actual port |

### CLAUDE.md

| Line | Issue | Fix |
|------|-------|-----|
| L72 | "Start control plane API on :8080" | Correct (matches implementation) |
| L218 | Cost comparison | OK (marketing, not technical) |

---

## LOW: Localhost References (Development Context)

These references are **CORRECT** for local development and should NOT be changed:

| File | Reference | Context |
|------|-----------|---------|
| `docs/getting-started/*.md` | `localhost:8080` | Local dev instructions |
| `docs/guides/*.md` | `localhost:8080` | Testing guides |
| `apps/switchyard-ui/README.md` | `localhost:8080` | Dev setup |
| `examples/README.md` | `localhost:8080` | Example code |

---

## OK: OIDC URL References (Current)

These 21 files reference `auth.madfam.io` or `api.janua.dev` and are **CURRENT**:

- `CLAUDE.md` - Correct OIDC configuration
- `AI_CONTEXT.md` - Correct references
- `README.md` - Correct production URLs
- `docs/production/*.md` - Correct infrastructure docs
- `docs/guides/*.md` - Correct setup guides
- `apps/switchyard-ui/*.md` - Correct UI configuration

**OIDC Endpoints Verified** (2026-01-17):
- `https://auth.madfam.io/.well-known/openid-configuration` → **200 OK**
- `https://api.janua.dev/.well-known/openid-configuration` → **200 OK**

---

## Action Items

### Immediate (P0)
1. [ ] Decide on port strategy: 4200 (docs) vs 8080 (implementation)
2. [ ] Update INFRA_ANATOMY.md to mark Redis URL as bug, not feature

### Short-term (P1)
3. [ ] Synchronize all port references after decision
4. [ ] Add "Intended vs Actual" section to INFRA_ANATOMY.md

### Medium-term (P2)
5. [ ] Create automated docs validation in CI
6. [ ] Add port consistency check to diagnose-prod.sh

---

## Validation Commands

```bash
# Check port references in K8s manifests
grep -r "4200\|8080" infra/k8s/ --include="*.yaml"

# Check OIDC endpoints
curl -s https://auth.madfam.io/.well-known/openid-configuration | jq '.issuer'
curl -s https://api.janua.dev/.well-known/openid-configuration | jq '.issuer'

# Run production diagnostics
ssh solarpunk@ssh.madfam.io
./scripts/diagnose-prod.sh --quick
```

---

## Appendix: Files Scanned

### Documentation Files (*.md)
- Total scanned: 150+ files
- Issues found: 6 files with port mismatches
- OK: 21 files with OIDC references (current)

### Infrastructure Files
- `infra/k8s/base/*.yaml` - Port mismatch detected
- `infra/k8s/production/*.yaml` - Port mismatch detected
- `infra/argocd/*.yaml` - OK

### Source Code
- `apps/switchyard-api/Dockerfile` - Uses 4200
- `apps/switchyard-api/cmd/api/main.go` - Should be checked for default port
