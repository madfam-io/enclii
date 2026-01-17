# Infrastructure Anatomy - Production State

> **Generated**: 2026-01-17 | **Last Updated**: 2026-01-17 | **Host**: foundry-core | **Audit Type**: Deep Metal Forensic
>
> **Live Status Check** (2026-01-17):
> - auth.madfam.io OIDC: ✅ 200 OK
> - api.enclii.dev/health: ❌ 502 Bad Gateway
> - app.enclii.dev: ⚠️ Loading (API connection issues)

## Executive Summary

| Category | Status | Severity |
|----------|--------|----------|
| **Architecture Conflict** | Triple tunnel + dual-stack | CRITICAL |
| **Disk Pressure** | 87% usage | CRITICAL |
| **Database Exposure** | 0.0.0.0 binding | CRITICAL |
| **OIDC Endpoints** | auth.madfam.io ✅ 200 OK | RESOLVED |
| **Switchyard API** | api.enclii.dev ❌ 502 | CRITICAL |
| **Redis URL Drift** | External IP instead of K8s DNS | CRITICAL |
| **Port Mismatch** | Docs say 4200, K8s uses 8080 | HIGH |
| **ImagePullBackOff** | 8+ pods | HIGH |
| **Pod Evictions** | 10+ pods | HIGH |

---

## Host Details

| Property | Value |
|----------|-------|
| Hostname | `foundry-core` |
| OS | Ubuntu 24.04.3 LTS (Noble Numbat) |
| Kernel | 6.8.0-88-generic |
| K8s Version | K3s v1.33.6+k3s1 |
| Node Count | 1 (single node) |
| Public IP | 95.217.198.239 |

---

## Architecture Overview

### Dual-Stack Design (CONFLICT!)

The production host runs **two parallel service layers**:

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLOUDFLARE TUNNELS (3x!)                     │
├─────────────────────────────────────────────────────────────────┤
│  systemd: cloudflared.service (foundry-prod)                    │
│  systemd: cloudflared-janua.service (janua-prod) ← CAN'T REACH K8s │
│  K8s: cloudflared pods (2-4 replicas) ← CORRECT CONFIG          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         nginx (80/443)                          │
│  app.janua.dev → 127.0.0.1:8010                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────────────┐
│   DOCKER CONTAINERS     │     │      K8s CLUSTER (K3s)          │
│                         │     │                                 │
│  janua-api (4100)       │     │  janua-api.janua.svc (80)       │
│  janua-proxy            │     │  janua-dashboard.janua.svc      │
│  postgres-shared (5432) │     │  switchyard-api.enclii.svc      │
│  redis-shared (6379)    │     │  dispatch.enclii.svc            │
│  verdaccio (4873)       │     │  postgres (data.svc)            │
│  foundry-registry (5000)│     │  redis (data.svc)               │
└─────────────────────────┘     └─────────────────────────────────┘
         ↑                                    ↑
         │                                    │
    0.0.0.0 EXPOSED                     ClusterIP (internal)
```

---

## Namespaces

| Namespace | Purpose | Pod Count | Status |
|-----------|---------|-----------|--------|
| `janua` | Identity Provider | 10 | ⚠️ ImagePullBackOff |
| `enclii` | Platform Control Plane | 15 | ⚠️ Evicted pods |
| `enclii-builds` | CI/CD Build Jobs | 35+ | ⚠️ Error/Completed |
| `cloudflare-tunnel` | Ingress | 4 | ✅ Running |
| `argocd` | GitOps Engine | 8 | ✅ Running |
| `longhorn-system` | Block Storage CSI | 15 | ✅ Running |
| `monitoring` | Prometheus/Grafana | 3 | ✅ Running |
| `data` | Shared Databases | 2 | ✅ Running |
| `external-secrets` | Secret Management | 5 | ✅ Running |
| `kyverno` | Policy Engine | 6 | ⚠️ ImagePullBackOff |
| `arc-runners` | GitHub Actions | 1 | ✅ Running |
| `arc-system` | ARC Controller | 3 | ✅ Running |
| `cnpg-system` | CloudNative PG | 1 | ✅ Running |

---

## Services by Namespace

### janua

| Service | Type | Port | targetPort | Status |
|---------|------|------|------------|--------|
| janua-api | ClusterIP | 80 | **8080** | ⚠️ Port changed |
| janua-dashboard | ClusterIP | 80 | 80 | ✅ |
| janua-admin | ClusterIP | 80 | 80 | ✅ |
| janua-docs | ClusterIP | 80 | 80 | ✅ |
| janua-website | ClusterIP | 80 | 80 | ✅ |

### enclii

| Service | Type | Port | targetPort | Status |
|---------|------|------|------------|--------|
| switchyard-api | ClusterIP | 80 | **4200** | ⚠️ Deviation |
| switchyard-ui | ClusterIP | 80 | 80 | ✅ |
| dispatch | ClusterIP | 80 | 80 | ✅ |
| roundhouse | ClusterIP | 80, 8080 | - | ✅ |
| waybill | ClusterIP | 80 | - | ✅ |
| docs-site | ClusterIP | 80 | - | ✅ |
| landing-page | ClusterIP | 80 | - | ✅ |

### data

| Service | Type | Port | Status |
|---------|------|------|--------|
| postgres | ClusterIP (headless) | 5432 | ✅ |
| redis | ClusterIP | 6379 | ✅ |

### monitoring

| Service | Type | Port | Status |
|---------|------|------|--------|
| prometheus | ClusterIP | 9090 | ✅ |
| grafana | ClusterIP | 3000 | ✅ |
| alertmanager | ClusterIP | 9093 | ✅ |

---

## Cloudflare Tunnel Routes

### K8s ConfigMap Routes (Correct Configuration)

| Hostname | Target Service | Status |
|----------|---------------|--------|
| api.enclii.dev | switchyard-api.enclii.svc:80 | ✅ |
| app.enclii.dev | switchyard-ui.enclii.svc:80 | ✅ |
| admin.enclii.dev | switchyard-ui.enclii.svc:80 | ✅ |
| docs.enclii.dev | docs-site.enclii.svc:80 | ✅ |
| enclii.dev | landing-page.enclii.svc:80 | ✅ |
| api.janua.dev | janua-api.janua.svc:80 | ⚠️ 502 |
| auth.madfam.io | janua-api.janua.svc:80 | ⚠️ 502 |
| app.janua.dev | janua-dashboard.janua.svc:80 | ❓ |
| admin.janua.dev | janua-admin.janua.svc:80 | ❓ |
| argocd.enclii.dev | argocd-server.argocd.svc:443 | ✅ |
| agents.madfam.io | claudecodeui.enclii-madfam-automation-production.svc:80 | ❓ |
| *.fn.enclii.dev | keda-add-ons-http-interceptor-proxy.keda.svc:8080 | ❓ |

---

## Docker Containers (Host Level)

| Container | Ports | Status |
|-----------|-------|--------|
| janua-api | 0.0.0.0:4100, 0.0.0.0:8000 | Up 9h |
| janua-proxy | - | Up 9h |
| postgres-shared | **0.0.0.0:5432** | Up 5 weeks |
| redis-shared | **0.0.0.0:6379** | Up 5 weeks |
| verdaccio | 0.0.0.0:4873 | Up 5 weeks |
| foundry-registry | 0.0.0.0:5000 | Up 5 weeks |

---

## Storage Status

| Metric | Value | Threshold | Status |
|--------|-------|-----------|--------|
| Root Disk Usage | **87%** | <85% | CRITICAL |
| Available Space | 13GB | - | LOW |
| Inode Usage | 21% | <90% | OK |
| Node Allocatable | ~93GB | - | - |

### PVC Status

| PVC | Namespace | Capacity | Status |
|-----|-----------|----------|--------|
| arc-docker-cache-blue | arc-runners | 50Gi | Bound |
| arc-docker-cache-green | arc-runners | - | **Pending** |
| arc-go-cache | arc-runners | 20Gi | Bound |
| arc-npm-cache | arc-runners | 20Gi | Bound |
| postgres-data | data | 20Gi | Bound |
| redis-data | data | 5Gi | Bound |
| prometheus-data | monitoring | 20Gi | Bound |
| grafana-data | monitoring | 5Gi | Bound |
| alertmanager-data | monitoring | 2Gi | Bound |

---

## Security Findings

### CRITICAL: Database Exposure

```bash
# PostgreSQL exposed on ALL interfaces
LISTEN 0.0.0.0:5432 (docker-proxy)

# Redis exposed on ALL interfaces
LISTEN 0.0.0.0:6379 (docker-proxy)
```

**Remediation**: Bind to 127.0.0.1 or use K8s services exclusively.

### Environment Variables

| Service | Variable | Value | Status |
|---------|----------|-------|--------|
| janua-api | DATABASE_URL | K8s internal | ✅ |
| janua-api | REDIS_URL | K8s internal | ✅ |
| janua-api | JWT_ALGORITHM | RS256 | ✅ |
| switchyard-api | ENCLII_REDIS_URL | **95.217.198.239:6379** | **BUG** - Should be `redis://redis.data.svc.cluster.local:6379` |
| dispatch | NEXT_PUBLIC_JANUA_URL | https://auth.madfam.io | ✅ |

---

## Known Issues

### 1. Triple Tunnel Conflict

**Problem**: Three cloudflared instances running with conflicting routes.

**Evidence**:
```
systemd: cloudflared.service (foundry-prod) - since Dec 9
systemd: cloudflared-janua.service (janua-prod) - since Jan 17
K8s: cloudflared pods x4 - using ConfigMap
```

**Impact**: 502 errors on janua endpoints.

**Root Cause**: systemd cloudflared-janua tries to reach K8s ClusterIP (10.43.82.124) but can't.

### 2. ImagePullBackOff Epidemic

**Affected Pods**:
- janua-admin-686547dc5-z74md
- janua-api-7f9f5b467c-tnmwf
- janua-dashboard-c77dbc88-q86zs
- janua-docs-85b7cd869-52gw4
- janua-website-648bb8f57f-pw958
- claudecodeui pods (2x)
- kyverno cleanup jobs

**Likely Cause**: Registry authentication or rate limiting.

### 3. Mass Pod Evictions

**Namespace**: enclii (switchyard-ui), enclii-builds

**Root Cause**: Disk pressure at 87%.

### 4. switchyard-api SQL Error

```
Failed to list functions: sql: converting argument $1 type: unsupported type []string
```

**Type**: Code bug in function listing.

### 5. Port Mismatch (4200 vs 8080)

**Problem**: Documentation and source code specify port 4200, but K8s manifests use 8080.

**Evidence**:
| Source | Port |
|--------|------|
| `config.go` default | 4200 |
| `Dockerfile` EXPOSE | 4200 |
| `CLAUDE.md` port allocation | 4200 |
| `switchyard-api.yaml` ENCLII_PORT | 8080 |
| `cloudflared.yaml` tunnel route | 8080 |

**Root Cause**: K8s manifest `ENCLII_PORT` environment variable overrides the application default.

**Impact**: Confusion in documentation and potential routing issues if not consistently applied.

**Resolution**: Either update K8s manifests to 4200 (align with docs) or update docs to 8080 (align with K8s).

### 6. External Redis URL in Cluster

**Problem**: switchyard-api using external IP for Redis instead of internal K8s DNS.

**Evidence**:
```
ENCLII_REDIS_URL=95.217.198.239:6379  # Current (WRONG)
```

**Expected**:
```
ENCLII_REDIS_URL=redis://redis.data.svc.cluster.local:6379  # Correct
```

**Root Cause**: Runtime drift - manifests are correct but cluster state diverged.

**Fix**: Force ArgoCD sync or apply `kubectl set env` patch.

---

## Recommended Actions

### Immediate (P0)

1. **Stop rogue systemd tunnels**:
   ```bash
   sudo systemctl stop cloudflared.service cloudflared-janua.service
   sudo systemctl disable cloudflared.service cloudflared-janua.service
   ```

2. **Free disk space**:
   ```bash
   sudo crictl rmp -a  # Remove stopped containers
   kubectl delete pods --field-selector=status.phase=Failed -A
   kubectl delete pods --field-selector=status.phase=Evicted -A
   ```

3. **Secure database ports**:
   - Modify Docker compose to bind 127.0.0.1 instead of 0.0.0.0
   - Or migrate to K8s-only database access

4. **Fix Redis URL** (Issue #6):
   ```bash
   # Option A: Force ArgoCD sync
   kubectl -n argocd patch application core-services --type merge -p '{"operation":{"sync":{"force":true}}}'

   # Option B: Direct patch
   kubectl set env deployment/switchyard-api -n enclii \
       ENCLII_REDIS_URL="redis://redis.data.svc.cluster.local:6379"
   ```

### Short-term (P1)

4. **Fix imagePullPolicy**:
   - Change janua deployments from `IfNotPresent` to `Always`

5. **Fix switchyard-api SQL bug**:
   - Update function listing query to handle slice arguments

6. **Investigate registry auth**:
   - Check GHCR rate limits
   - Verify registry credentials are valid

### Medium-term (P2)

7. **Consolidate architecture**:
   - Migrate Docker services to K8s
   - Remove nginx layer
   - Single source of truth for tunnels

8. **Add monitoring alerts**:
   - Disk usage > 80%
   - ImagePullBackOff count > 0
   - Pod eviction events

---

## Appendix: Port Mapping Reference

| Service | Expected Port | Actual Port | Deviation |
|---------|--------------|-------------|-----------|
| janua-api | 4100 | 8080 (K8s) / 4100 (Docker) | Yes |
| switchyard-api | 8080 | 4200 | Yes |
| switchyard-ui | 3000 | 80 | Normalized |
| dispatch | 4203 | 80 | Normalized |
| postgres | 5432 | 5432 | No |
| redis | 6379 | 6379 | No |
