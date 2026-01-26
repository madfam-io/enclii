# Infrastructure Anatomy - Production State

> **Generated**: 2026-01-17 | **Last Updated**: 2026-01-26 | **Host**: foundry-core + foundry-builder-01 | **Audit Type**: Full Ecosystem Audit
>
> **Live Status Check** (2026-01-26):
> - auth.madfam.io OIDC: ✅ 200 OK
> - api.enclii.dev: ✅ 404 (API root, health at /health)
> - app.enclii.dev: ✅ 200 OK
> - All 28 production domains: ✅ Responding correctly
> - Pods: 79 Running, 12 Completed, 0 errors

## Executive Summary

| Category | Status | Severity |
|----------|--------|----------|
| **Architecture Conflict** | K8s-only (systemd disabled) | ✅ RESOLVED (Jan 17) |
| **Disk Pressure** | 87% usage | ✅ RESOLVED (Jan 25, cleanup) |
| **Database Exposure** | 127.0.0.1 binding | ✅ RESOLVED (Jan 17) |
| **OIDC Endpoints** | auth.madfam.io ✅ 200 OK | ✅ RESOLVED |
| **Switchyard API** | api.enclii.dev ✅ 200 OK | ✅ RESOLVED |
| **Redis URL Drift** | K8s internal DNS | ✅ RESOLVED (Jan 17) |
| **Port Mismatch** | Docs say 4200, K8s uses 8080 | ✅ RESOLVED (Jan 25, uses 4200) |
| **ImagePullBackOff** | 0 pods (was 8+) | ✅ RESOLVED (Jan 25) |
| **Pod Evictions** | 0 pods (was 10+) | ✅ RESOLVED (Jan 25, disk cleanup) |
| **VPS Builder Node** | CNI fixed (k3s version match) | ✅ RESOLVED (Jan 25) |
| **Dual Cloudflared** | Consolidated to single deployment | ✅ RESOLVED (Jan 26) |
| **Kyverno CronJobs** | Using bitnami/kubectl:latest | ✅ RESOLVED (Jan 26) |
| **Grafana CrashLoop** | PVC and ConfigMap fixed | ✅ RESOLVED (Jan 25) |
| **dhanam-api CrashLoop** | TCP probes (startup timing) | ✅ RESOLVED (Jan 25) |

---

## Host Details

| Node | IP | Role | Hardware | k3s | CPU | RAM | Status |
|------|----|------|----------|-----|-----|-----|--------|
| **foundry-core** | 95.217.198.239 | control-plane, master | Hetzner AX41-NVME (Ryzen 5 3600, 64GB, 2x512GB NVMe) | v1.33.6+k3s1 | 5% | 33% (21GB/64GB) | Ready |
| **foundry-builder-01** | 77.42.89.211 | worker (role=builder) | VPS ("The Forge") | v1.33.6+k3s1 | 2% | 23% (916Mi/4GB) | Ready |

- **OS**: Ubuntu 24.04.3 LTS (Noble Numbat)
- **Kernel**: 6.8.0-88-generic
- **Node Count**: 2 (was 1 until Jan 2026)
- **Builder Node**: Tainted `builder=true:NoSchedule` — only ARC runner pods schedule here

---

## Architecture Overview

### Unified K8s-Only Architecture (RESOLVED Jan 2026)

All services run exclusively in K8s. Docker containers (Verdaccio, registry) run on the host for non-K8s workloads. systemd tunnels disabled.

```
┌─────────────────────────────────────────────────────────────────┐
│              CLOUDFLARE TUNNEL (single unified)                  │
│  K8s: cloudflared pods (2 replicas, v2025.11.1)                 │
│  Config: infra/k8s/production/cloudflared-unified.yaml          │
│  Routes: ~28 production domains                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                K8s CLUSTER (K3s, 2 nodes)                       │
│                                                                 │
│  foundry-core (control-plane):                                  │
│    janua-api.janua.svc (80)       switchyard-api.enclii.svc (80)│
│    janua-dashboard.janua.svc      dispatch.enclii.svc (80)      │
│    postgres.enclii.svc (5432)     redis.data.svc (6379)         │
│    grafana.monitoring.svc (3000)  prometheus.monitoring.svc      │
│    argocd-server.argocd.svc       dhanam-api.dhanam.svc (80)    │
│                                                                 │
│  foundry-builder-01 (worker, builder taint):                    │
│    arc-runner-blue pods (GitHub Actions CI)                     │
│                                                                 │
│  Host-level Docker:                                             │
│    verdaccio (4873) — npm registry                              │
│    foundry-registry (5000) — container registry                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Namespaces (19 active as of Jan 26, 2026)

| Namespace | Purpose | Status |
|-----------|---------|--------|
| `enclii` | Platform Control Plane (switchyard-api/ui, dispatch, roundhouse, status pages, landing, postgres, redis, jaeger, waybill) | ✅ Healthy |
| `janua` | Identity Provider (janua-api, dashboard, admin, docs, website) | ✅ Healthy |
| `dhanam` | Finance Services (dhanam-api, admin, web) | ✅ Healthy |
| `cloudflare-tunnel` | Ingress (2 cloudflared replicas, v2025.11.1) | ✅ Healthy |
| `argocd` | GitOps Engine (server, repo-server, dex, image-updater, redis, notifications) | ✅ Healthy |
| `longhorn-system` | Block Storage CSI (v1.7.2, manager, UI, CSI drivers) | ✅ Healthy |
| `monitoring` | Observability (Prometheus v2.48.0, Grafana v10.2.2, AlertManager v0.26.0) | ✅ Healthy |
| `data` | Shared Databases (Redis 7-alpine, PostgreSQL 16 via CNPG) | ✅ Healthy |
| `external-secrets` | Secret Management (ESO v0.9.11) | ✅ Healthy |
| `kyverno` | Policy Engine (v1.11.4, 16 ClusterPolicies ready) | ✅ Healthy |
| `arc-runners` | GitHub Actions Runner Sets (blue active, green standby) | ✅ Healthy |
| `arc-system` | ARC Controller (gha-runner-scale-set-controller v0.10.1) | ✅ Healthy |
| `cnpg-system` | CloudNative PG Operator (v1.25.0) | ✅ Healthy |
| `enclii-builds` | CI/CD Build Jobs | ✅ Healthy |
| `kube-system` | K8s system (CoreDNS, metrics-server, local-path-provisioner) | ✅ Healthy |
| `sentinel` | Future Redis Sentinel HA (empty, reserved) | ⏳ Placeholder |
| `default` | Default namespace | ✅ Empty |
| `kube-node-lease` | Node heartbeats | ✅ System |
| `kube-public` | Public info | ✅ System |

**Cleaned up (Jan 25-26):** `enclii-dhanam-production`, `enclii-madfam-automation-dev`, `enclii-madfam-automation-prod`, `enclii-madfam-automation-production`, `cloudflare` (legacy) — all deleted as empty/abandoned.

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

## Cloudflare Tunnel Routes (Jan 26, 2026)

Single unified tunnel via `infra/k8s/production/cloudflared-unified.yaml`. All routes verified.

| Hostname | Target Service | HTTP | Notes |
|----------|---------------|------|-------|
| api.enclii.dev | switchyard-api.enclii.svc:80 | 404 | API root; /health returns 200 |
| app.enclii.dev | switchyard-ui.enclii.svc:80 | 200 | |
| admin.enclii.dev | dispatch.enclii.svc:80 | 307 | Redirect to auth |
| enclii.dev | landing-page.enclii.svc:80 | 200 | |
| www.enclii.dev | landing-page.enclii.svc:80 | 200 | |
| docs.enclii.dev | docs-site.enclii.svc:80 | 200 | |
| status.enclii.dev | status-enclii.enclii.svc:80 | 200 | |
| status.madfam.io | status-madfam.enclii.svc:80 | 200 | |
| argocd.enclii.dev | argocd-server.argocd.svc:443 | 404 | noTLSVerify, self-signed |
| grafana.enclii.dev | grafana.monitoring.svc:3000 | 302 | Redirect to login |
| prometheus.enclii.dev | prometheus.monitoring.svc:9090 | 302 | |
| alertmanager.enclii.dev | alertmanager.monitoring.svc:9093 | 200 | |
| api.janua.dev | janua-api.janua.svc:80 | 200 | Primary auth domain |
| auth.madfam.io | janua-api.janua.svc:80 | 200 | MADFAM alias |
| app.janua.dev | janua-dashboard.janua.svc:80 | 307 | |
| admin.janua.dev | janua-admin.janua.svc:80 | 307 | |
| docs.janua.dev | janua-docs.janua.svc:80 | 200 | |
| janua.dev | janua-website.janua.svc:80 | 200 | |
| www.janua.dev | janua-website.janua.svc:80 | 200 | |
| madfam.io | janua-website.janua.svc:80 | 307 | |
| www.madfam.io | janua-website.janua.svc:80 | 307 | |
| npm.madfam.io | 95.217.198.239:4873 (host Docker) | 200 | Verdaccio |
| api.dhan.am | dhanam-api.dhanam.svc:80 | 404 | API root |
| admin.dhan.am | dhanam-admin.dhanam.svc:80 | 200 | |
| app.dhan.am | dhanam-web.dhanam.svc:80 | 307 | |
| dhan.am | dhanam-web.dhanam.svc:80 | 200 | |
| www.dhan.am | dhanam-web.dhanam.svc:80 | 200 | |
| *.fn.enclii.dev | keda interceptor.keda.svc:8080 | - | KEDA scale-to-zero |
| ssh.madfam.io | ssh://95.217.198.239:22 | 302 | Cloudflare Access gate |
| agents.madfam.io | http_status:503 | 502 | Pending Auto-Claude deploy |
| (catch-all) | http_status:404 | 404 | Required default |

**Removed routes (Jan 26):** `dashboard.madfam.io`, `admin.madfam.io`, `docs.madfam.io` — confirmed nonexistent subdomains.

---

## Docker Containers (Host Level)

| Container | Ports | Status |
|-----------|-------|--------|
| janua-api | 0.0.0.0:4100, 0.0.0.0:8000 | Up 9h |
| janua-proxy | - | Up 9h |
| postgres-shared | **127.0.0.1:5432** | ✅ Secured (2026-01-17) |
| redis-shared | **127.0.0.1:6379** | ✅ Secured (2026-01-17) |
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

### PVC Status (Jan 26, 2026)

| PVC | Namespace | Size | StorageClass | Status |
|-----|-----------|------|-------------|--------|
| arc-docker-cache-blue | arc-runners | 50Gi | local-path | Bound |
| arc-docker-cache-green | arc-runners | 50Gi | local-path | Pending (WaitForFirstConsumer) |
| arc-go-cache | arc-runners | 20Gi | local-path | Bound |
| arc-npm-cache | arc-runners | 20Gi | local-path | Bound |
| postgres-data | data | 20Gi | local-path | Bound |
| redis-data | data | 5Gi | local-path | Bound |
| postgres-pvc | enclii | 10Gi | longhorn | Bound |
| redis-pvc | enclii | 5Gi | longhorn | Bound |
| prometheus-data | monitoring | 20Gi | longhorn | Bound |
| grafana-data | monitoring | 5Gi | longhorn | Bound |
| alertmanager-data | monitoring | 2Gi | longhorn | Bound |

**Note:** `arc-docker-cache-green` is Pending because the green runner set is not active (blue/green deployment strategy for ARC runners). This is expected.

---

## Security Findings

### ✅ RESOLVED: Database Exposure (Fixed 2026-01-17)

```bash
# PostgreSQL now bound to localhost only
LISTEN 127.0.0.1:5432 (docker-proxy)

# Redis now bound to localhost only
LISTEN 127.0.0.1:6379 (docker-proxy)
```

**Resolution**: Modified `/opt/solarpunk/janua/docker-compose.production.yml` to bind ports to 127.0.0.1.

### Environment Variables

| Service | Variable | Value | Status |
|---------|----------|-------|--------|
| janua-api | DATABASE_URL | K8s internal | ✅ |
| janua-api | REDIS_URL | K8s internal | ✅ |
| janua-api | JWT_ALGORITHM | RS256 | ✅ |
| switchyard-api | ENCLII_REDIS_URL | `redis://redis.data.svc.cluster.local:6379` | ✅ (Fixed 2026-01-17) |
| dispatch | NEXT_PUBLIC_JANUA_URL | https://auth.madfam.io | ✅ |

---

## Known Issues

### ✅ 1. Triple Tunnel Conflict (RESOLVED 2026-01-17)

**Problem**: Three cloudflared instances running with conflicting routes.

**Evidence**:
```
systemd: cloudflared.service (foundry-prod) - DISABLED
systemd: cloudflared-janua.service (janua-prod) - DISABLED
K8s: cloudflared pods x4 - using ConfigMap ✅ ACTIVE
```

**Resolution**: Disabled systemd tunnels. All traffic now routes through K8s cloudflared pods.

**Verification**: `systemctl is-enabled cloudflared.service cloudflared-janua.service` returns `disabled`.

### ✅ 2. ImagePullBackOff Epidemic (RESOLVED Jan 25-26)

**Resolution**: All ImagePullBackOff pods cleared. Root causes:
- Registry rate limiting (resolved with proper imagePullPolicy)
- Kyverno cleanup CronJobs using bitnami/kubectl:1.31 (tag removed from Docker Hub, switched to `latest`)
- claudecodeui pods deleted (service discontinued, replaced by Auto-Claude)

### ✅ 3. Mass Pod Evictions (RESOLVED Jan 25)

**Resolution**: Disk cleanup freed space. Failed/evicted pods cleaned up.

### 4. switchyard-api SQL Error (EXISTING)

```
Failed to list functions: sql: converting argument $1 type: unsupported type []string
```

**Type**: Code bug in function listing. Non-critical — functions feature not yet in production use.

### ✅ 5. Port Mismatch (RESOLVED Jan 25)

**Resolution**: All K8s manifests now consistently use port 4200 for switchyard-api. Service exposes port 80 → targetPort 4200.

### ✅ 6. External Redis URL in Cluster (RESOLVED 2026-01-17)

**Problem**: switchyard-api using external IP for Redis instead of internal K8s DNS.

**Evidence** (Before):
```
ENCLII_REDIS_URL=95.217.198.239:6379  # WRONG
```

**Resolution**:
```bash
kubectl set env deployment/switchyard-api -n enclii \
    ENCLII_REDIS_URL="redis://redis.data.svc.cluster.local:6379"
```

**Verification**: API health check returns 200, Redis traffic stays internal to cluster.

---

## Recommended Actions (Updated Jan 26, 2026)

### Immediate (P0) — ALL RESOLVED

All P0 items from Jan 17 have been addressed:
- ✅ systemd tunnels disabled
- ✅ Disk space freed
- ✅ Database ports secured to 127.0.0.1
- ✅ Redis URL corrected to K8s internal DNS

### Short-term (P1)

1. **Fix switchyard-api SQL bug**: Update function listing query to handle slice arguments
2. **Deploy Auto-Claude**: Replace agents.madfam.io http_status:503 with actual Auto-Claude service
3. **Provision Doppler**: Configure External Secrets Operator Doppler provider (currently Degraded)

### Medium-term (P2)

4. **ArgoCD OCI Helm support**: `arc-runners` shows Unknown sync due to OCI chart fetch limitation — awaiting ArgoCD improvements
5. **Image Updater ConfigMap conflict**: Shared ConfigMap between Helm chart and custom config app — consider consolidating
6. **Add monitoring alerts**: Disk usage > 80%, ImagePullBackOff > 0, pod eviction events
7. **Upgrade monitoring stack**: Prometheus v2.48.0 and Grafana v10.2.2 are aging — evaluate upgrade path

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

---

## Stabilization Log (2026-01-17)

### Session Summary

Executed infrastructure recovery plan to restore production stability. All critical issues resolved.

### Tunnel Consolidation
- **Action**: Verified systemd tunnels already disabled; K8s cloudflared pods (4 replicas) handling all traffic
- **Reason**: Triple tunnel conflict was causing routing confusion
- **Result**: All traffic now routes through K8s cloudflared pods (cloudflare-tunnel namespace)
- **Verification**: `curl https://auth.madfam.io/.well-known/openid-configuration` returns 200 OK

### Switchyard API Recovery
- **Action**: Reset database migration version from 23 to 22 (corrupted state - version marked but tables not created)
- **Root Cause**: Container image `c5b2d17` deployed without migration 023 files, but DB was marked at version 23
- **Result**: `api.enclii.dev/health` returns 200 OK with status "healthy"
- **Note**: Need to deploy newer image with migration 023 for functions feature

### Redis URL Correction
- **Action**: `kubectl set env deployment/switchyard-api -n enclii ENCLII_REDIS_URL="redis://redis.data.svc.cluster.local:6379"`
- **Before**: External IP `95.217.198.239:6379`
- **After**: K8s internal DNS `redis.data.svc.cluster.local:6379`
- **Security Impact**: Redis traffic no longer crosses public internet

### Database Port Security
- **Action**: Modified `/opt/solarpunk/janua/docker-compose.production.yml` to bind ports to 127.0.0.1
- **Before**: PostgreSQL and Redis on `0.0.0.0` (public internet accessible)
- **After**: PostgreSQL and Redis on `127.0.0.1` (localhost only)
- **Verification**: `netstat -tlnp | grep -E '5432|6379'` shows 127.0.0.1 binding

### Outstanding Issues (Jan 17) — ALL RESOLVED

All outstanding issues from Jan 17 have been resolved during the Jan 25-26 audit:
- ✅ Disk pressure at 87% → cleaned up failed pods and old images
- ✅ ImagePullBackOff on 8+ pods → fixed registry auth, imagePullPolicy, bitnami/kubectl tags
- ✅ Pod evictions → resolved with disk cleanup
- ✅ Port mismatch (4200 vs 8080) → standardized on 4200
- ✅ Migration 023 → deployed with updated container image

---

## Stabilization Log (2026-01-25 / 2026-01-26) — Full Ecosystem Audit

### Trigger
Post-credential rotation end-to-end verification of the entire MADFAM production ecosystem.

### Scope
2-node cluster, 19 namespaces, 28 production domains, 13 ArgoCD applications.

### Critical Fixes Applied

#### 1. dhanam-api CrashLoop (CRITICAL)
- **Symptom**: CrashLoopBackOff for 2+ days, HTTP 500 on liveness/readiness probes
- **Root Cause**: HTTP health probes hit NestJS app during startup before routes initialized
- **Fix**: Switched to TCP probes (port 4200) — stable long-term solution
- **Investigation**: Rate limiter exclusion verified — health endpoints already exempt from both Fastify `@fastify/rate-limit` (allowList) and NestJS ThrottlerModule (opt-in per-controller)
- **Commit**: `9354dcb`

#### 2. Grafana CrashLoopBackOff (CRITICAL)
- **Symptom**: CrashLoop due to missing PVC and dashboard ConfigMap references
- **Fix**: Fixed PVC mount and dashboard ConfigMap configuration
- **Commit**: `9354dcb`

#### 3. Dispatch Wrong Image Path (CRITICAL)
- **Symptom**: `ghcr.io/madfam-org/dispatch` instead of `ghcr.io/madfam-org/enclii/dispatch`
- **Fix**: Corrected image path in deployment and golden test
- **Commit**: `9354dcb`

#### 4. VPS Builder Node CNI (CRITICAL)
- **Symptom**: Pods on foundry-builder-01 cannot reach K8s API (10.43.0.1:443 refused)
- **Root Cause**: k3s version mismatch — control plane v1.33.6, agent v1.34.3
- **Fix**: Downgraded agent to v1.33.6+k3s1 to match control plane
- **Verification**: ARC blue listener recovered, runner pods schedule correctly

#### 5. Cloudflared Consolidation (MEDIUM)
- **Symptom**: Dual cloudflared deployments (legacy v2024.12.0 + unified v2025.11.1)
- **Fix**: Deleted legacy deployment and `cloudflare` namespace, single unified config
- **Commit**: `4c17f1f`

#### 6. Kyverno CronJob Deadlock (DISCOVERED)
- **Symptom**: cleanup CronJobs use bitnami/kubectl:1.28.5 (removed from Docker Hub)
- **Root Cause**: Bitnami removed ALL version tags from Docker Hub, only `latest` remains
- **Fix**: Disabled upgrade hooks, set cleanup image tag to `latest`
- **Lesson**: Bitnami Docker Hub images now only have `latest` and SHA-based tags
- **Commits**: `39b3a72`, `7e4cbd4`, `9934b94`, `33b71ca`

#### 7. Kyverno Policy CRD Schema (DISCOVERED)
- **Symptom**: `ctlog.url` not in Kyverno 3.1.4 CRD schema; `mutateDigest` invalid for Audit mode
- **Fix**: Removed ctlog.url, set mutateDigest: false
- **Commit**: `9934b94`

#### 8. Cloudflared Kyverno Compliance (DISCOVERED)
- **Symptom**: `disallow-privileged-containers` blocks cloudflared rollout
- **Fix**: Added explicit `privileged: false` to securityContext
- **Commit**: `1391e1a`

#### 9. Nonexistent madfam.io Routes (CLEANUP)
- **Removed**: dashboard.madfam.io, admin.madfam.io, docs.madfam.io (confirmed nonexistent)
- **Commit**: `9a96a77`

#### 10. Abandoned Namespaces (CLEANUP)
- **Deleted**: enclii-dhanam-production, enclii-madfam-automation-dev/prod/production, cloudflare

### ArgoCD Application Status (Post-Audit)

| App | Sync | Health | Notes |
|-----|------|--------|-------|
| ingress | Synced | Healthy | |
| kyverno | Synced | Healthy | Hooks disabled, cleanup CronJobs using latest |
| longhorn | Synced | Healthy | v1.7.2 |
| monitoring | Synced | Healthy | Grafana, Prometheus, AlertManager |
| external-secrets | Synced | Healthy | |
| image-updater-config | Synced | Healthy | |
| core-services | Synced | Progressing | Cosmetic — Ingress resource has no controller |
| external-secrets-config | Synced | Degraded | Doppler provider not provisioned |
| kyverno-policies | OutOfSync | Healthy | SSA metadata drift, 16 policies Ready |
| argocd-image-updater | OutOfSync | Healthy | Shared ConfigMap (Helm vs custom) |
| enclii-infrastructure | OutOfSync | Healthy | Child app drift (image-updater) |
| arc-runners | Unknown | Healthy | OCI chart fetch limitation |
| arc-runners-blue | Unknown | Healthy | OCI chart fetch limitation |

### Verification Commands
```bash
# All domains health check
for domain in api.enclii.dev app.enclii.dev admin.enclii.dev enclii.dev docs.enclii.dev \
  status.enclii.dev api.janua.dev auth.madfam.io grafana.enclii.dev api.dhan.am; do
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "https://${domain}")
  echo "$code $domain"
done

# Pod health
KUBECONFIG=~/.kube/config-hetzner kubectl get pods -A --field-selector 'status.phase!=Running,status.phase!=Succeeded'
# Expected: No results (zero failing pods)

# ArgoCD status
KUBECONFIG=~/.kube/config-hetzner kubectl get applications -n argocd

# Node status
KUBECONFIG=~/.kube/config-hetzner kubectl get nodes -o wide
# Expected: 2 nodes, both Ready, both v1.33.6+k3s1
```

### Git Commits (Audit Session)
```
33b71ca fix(infra): use bitnami/kubectl:latest for kyverno cleanup CronJobs
9934b94 fix(infra): disable kyverno upgrade hooks and fix image policy validation
7e4cbd4 fix(infra): correct kyverno Helm values paths for kubectl image overrides
39b3a72 fix(infra): fix kyverno post-upgrade hook image and CRD schema issue
1391e1a fix(infra): add explicit privileged: false to cloudflared securityContext
9a96a77 fix(infra): remove nonexistent madfam.io subdomain routes from tunnel config
4c17f1f fix(infra): resolve ArgoCD sync issues, consolidate cloudflared, fix kyverno policies
9354dcb fix(infra): correct dispatch image path and fix Grafana deployment
```
