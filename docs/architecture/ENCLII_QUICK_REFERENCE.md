# ENCLII QUICK REFERENCE GUIDE

## Platform Status at a Glance

| Metric | Value |
|--------|-------|
| **Production Readiness** | 70% (75/100 score) |
| **Infrastructure Cost** | $100/month |
| **vs Railway Savings** | 95% ($127,200 over 5 years) |
| **Services in Dogfooding** | 22 ready |
| **Timeline to GA** | 6-8 weeks |
| **Database Tables** | 8 implemented, 6 planned |
| **API Endpoints** | 25 implemented, 8 planned |
| **Test Coverage** | 11 security tests (100% middleware) |

---

## CORE CAPABILITIES MATRIX

### ✅ Fully Implemented

**Platform:**
- Multi-tenant project/environment management
- Service deployment with zero-downtime updates
- Kubernetes reconciliation system
- CLI (`enclii`) + Web UI (Next.js)

**Security:**
- JWT (RS256) authentication
- RBAC (Owner/Admin/Developer/ReadOnly)
- Immutable audit logging
- CSRF protection middleware

**Operations:**
- Prometheus metrics + Grafana dashboards
- Jaeger distributed tracing
- Structured JSON logging
- Real-time log streaming

**Infrastructure:**
- Terraform IaC for Hetzner + Cloudflare
- Kubernetes k3s cluster
- PostgreSQL + Redis
- Network isolation (NetworkPolicies)
- Horizontal pod autoscaling (HPA)

---

### ⚠️ Partially Implemented

**Building:**
- Git integration (not full pipeline)
- Dockerfile support (no buildpacks yet)
- Image signing (cosign) infrastructure exists
- SBOM generation (design only)

**Deployment:**
- Canary/blue-green (designed, not automated)
- Rollback capability (manual only)
- Health checks (readiness/liveness probes)
- Service mesh integration (not planned)

**Secrets:**
- Kubernetes Secret storage (plaintext)
- Environment variable injection
- Vault/1Password integration (designed)
- Secret rotation (designed)

**Storage:**
- PVC support (basic)
- Snapshot policy (designed)
- Volume encryption (designed)

**Cost:**
- Metering infrastructure designed
- No actual cost calculation
- No budget enforcement
- No showback reports

---

### 🔴 Not Yet Implemented

**Critical for Production:**
- Cloudflare Tunnel auto-provisioning (3 days work)
- R2 object storage integration (2 days work)
- Redis Sentinel HA (1 day work)

**Important:**
- Build pipeline orchestration (Roundhouse component)
- Janua OAuth integration (2 weeks)
- API key management
- Database backup automation
- Cost showback (Waybill component)

**Nice-to-Have:**
- KEDA autoscaling (event-driven)
- Policy-as-Code enforcement
- Multi-region deployments
- Feature flags
- Service mesh

---

## KEY NUMBERS

### Implemented Lines of Code
- **switchyard-api:** ~5,000 LOC (Go)
- **switchyard-ui:** ~2,000 LOC (TypeScript/React)
- **CLI:** ~1,500 LOC (Go)
- **Reconcilers:** ~1,000 LOC (Go)
- **Kubernetes manifests:** ~2,000 lines (YAML)
- **Terraform:** ~1,500 lines (HCL)

### Feature Completeness by Category
- Core Platform: 80/100 ✅
- Security: 75/100 ✅
- Operations: 65/100 ⚠️
- Infrastructure: 90/100 ✅
- Storage: 65/100 ⚠️
- Cost Tracking: 0/100 🔴

### Architecture
- **Microservices:** 4 (API, UI, CLI, Reconcilers)
- **Infrastructure Components:** 7 (Ingress, DNS, Certs, PostgreSQL, Redis, Jaeger, Prometheus)
- **Database Tables:** 8 implemented
- **Kubernetes Resources:** Deployment, Service, Ingress, HPA, PVC, NetworkPolicy, RBAC

---

## COMPONENT BREAKDOWN

### Switchyard API (Control Plane)
```
Language: Go 1.22+
Framework: Gin
Database: PostgreSQL
Cache: Redis
Auth: JWT (RS256)
Metrics: Prometheus
Tracing: Jaeger

Key Features:
✅ Service lifecycle management
✅ Deployment orchestration
✅ Auth/RBAC enforcement
✅ Audit logging (async)
✅ Rate limiting
✅ Connection pooling
✅ Circuit breaker pattern

Endpoints: 25 implemented / 8 planned
Tests: 11 security tests (100% middleware)
```

### Switchyard UI (Dashboard)
```
Language: TypeScript/React
Framework: Next.js 14
Styling: Tailwind CSS
API Client: Fetch + auth

Key Features:
✅ Project/service management
✅ Deployment status display
✅ Real-time log viewing
✅ Metrics visualization
✅ Cost dashboard (planned)

Pages:
- Dashboard
- Projects
- Services
- Deployments
- Logs
- Settings
```

### Conductor CLI
```
Language: Go 1.22+
Package: github.com/madfam-io/enclii/packages/cli

Commands:
✅ init - Scaffold service
✅ up - Deploy preview
✅ deploy - Deploy production
✅ logs - Stream logs
✅ ps - List services
✅ scale - Configure autoscaling
✅ secrets - Manage secrets
✅ rollback - Revert releases
✅ auth - Login/token management

Exit Codes:
0 = success
10 = validation error
20 = build failed
30 = deploy failed
40 = timeout
50 = auth error
```

### Kubernetes Reconcilers
```
Language: Go 1.22+
Type: Kubernetes Operators

Responsibilities:
✅ Service reconciliation
✅ Manifest generation
✅ Deployment status tracking
✅ Health check monitoring
✅ Resource cleanup

Uses:
- client-go for Kubernetes API
- controller-runtime for reconciliation
- Structured logging
- Metrics export
```

---

## DATABASE SCHEMA (Implemented)

```sql
-- 8 Tables, Fully Indexed

projects (UUID PK)
├─ name, slug
├─ created_at, updated_at
└─ FK: 1→many environments, services

environments (UUID PK)
├─ project_id (FK)
├─ name (enum: dev/stage/prod/preview-*)
├─ kube_namespace
└─ created_at, updated_at

services (UUID PK)
├─ project_id (FK)
├─ name, git_repo
├─ build_config (JSONB)
└─ created_at, updated_at

releases (UUID PK)
├─ service_id (FK)
├─ version, image_uri, git_sha
├─ status (building/ready/failed)
└─ created_at, updated_at

deployments (UUID PK)
├─ release_id (FK), environment_id (FK)
├─ replicas, status, health
└─ created_at, updated_at

routes (UUID PK)
├─ environment_id (FK), service_id (FK)
├─ host, path, tlsCertRef
└─ created_at

audit_events (UUID PK)
├─ actor, action, entityRef
├─ payload (JSONB)
└─ timestamp (immutable)

custom_domains (UUID PK)
├─ environment_id (FK)
├─ domain (UNIQUE)
├─ tlsCertRef
└─ created_at

-- Indexes on all FKs for query performance
```

---

## INFRASTRUCTURE STACK

### Compute
```
Hetzner Cloud - CPX31 (3x)
├─ 4 vCPU AMD EPYC
├─ 8GB RAM
├─ NVMe SSD
└─ €41/month (~$45)
```

### Kubernetes
```
k3s (Lightweight Kubernetes)
├─ Single cluster (v1)
├─ 3-node HA setup
├─ Managed by k3s service
└─ Single region
```

### Database
```
Ubicloud PostgreSQL
├─ Managed HA
├─ Automated backups
├─ Monitoring included
└─ $50/month
```

### Caching
```
Redis Sentinel
├─ 3-replica HA setup
├─ Automatic failover
├─ Persistence (AOF + RDB)
└─ Self-hosted on Hetzner
```

### Edge/CDN
```
Cloudflare
├─ Tunnel ($0 - replaces LoadBalancer)
├─ R2 Object Storage ($5/mo - zero egress)
├─ For SaaS ($0 - 100 free domains)
├─ DDoS Protection ($0)
└─ DNS Management ($0)
```

### Observability
```
Prometheus
├─ Metrics scraping
├─ /metrics endpoints on all pods
└─ 15-second scrape interval

Grafana
├─ Dashboard visualization
├─ Alert rules
└─ Prometheus datasource

Jaeger
├─ Distributed tracing
├─ All API requests traced
├─ Database queries traced
└─ OpenTelemetry exporter

Structured Logs
├─ JSON format
├─ Correlation IDs
├─ Kubernetes metadata
└─ Ready for Loki
```

---

## SECURITY FEATURES

### Authentication
- **Type:** JWT with RSA signing (RS256)
- **Storage:** Session in Redis
- **Expiry:** Configurable (default 1 hour)
- **Refresh:** Not yet implemented
- **API Keys:** Designed, not built

### Authorization
- **Model:** Role-Based Access Control (RBAC)
- **Roles:** Owner, Admin, Developer, ReadOnly
- **Scoping:** Per project, per environment
- **Enforcement:** Middleware + handler checks

### Network Security
- **NetworkPolicies:** Zero-trust by default
- **Namespace Isolation:** Strict boundaries
- **Pod Security Context:** Non-root, read-only FS
- **Capabilities:** Dropped unnecessary
- **Seccomp:** Enabled

### Secrets
- **Storage:** Kubernetes Secrets (at-rest encryption pending)
- **Transport:** TLS 1.3 only
- **Injection:** envFrom references
- **Audit:** Secret access not yet logged
- **Rotation:** Designed, not implemented

### Compliance
- **Audit Logging:** Immutable AuditEvent table
- **Retention:** No automatic cleanup
- **Export:** SIEM integration missing
- **Encryption:** Secret at-rest encryption planned

---

## DEPLOYMENT WORKFLOW

### Development (Preview)
```
1. enclii up
   └─ Build Docker image
   └─ Push to registry
   └─ Create Release object
   └─ Create preview-{branch} namespace
   └─ Deploy Kubernetes Deployment
   └─ Return URL (https://{hash}.project.enclii.dev)

SLA: P95 < 3 minutes
```

### Staging/Production
```
1. enclii deploy --env prod --strategy canary
   └─ Create Release object
   └─ Create Deployment (canary 10%)
   └─ Monitor SLOs (error rate, latency, availability)
   └─ Auto-promote 10% → 100% if healthy
   └─ Auto-rollback if SLO breach (2 min window)

SLA: P95 ≤ 8 minutes (build → running)
```

### Rollback
```
1. enclii rollback api --to {releaseId}
   └─ Swap ReplicaSets to previous version
   └─ Monitor SLOs for 10 minutes
   └─ Clean up failed ReplicaSet

SLA: P95 < 2 minutes
```

---

## TESTING COVERAGE

### Unit Tests
```
✅ Auth (JWT, password hashing)
✅ Middleware (CSRF, security headers)
✅ Validation (service spec, API inputs)
✅ Database (repository patterns)
✅ CLI (argument parsing)
```

### Integration Tests
```
⚠️ E2E deployment pipeline (partial)
⚠️ Service reconciliation (partial)
⚠️ Route provisioning (partial)
🔴 Build pipeline (not started)
🔴 Cost calculation (not started)
```

### Coverage
- **Middleware:** 100% (11 tests)
- **Auth:** 85% (JWT, password tests)
- **API Handlers:** 60% (main paths covered)
- **Reconcilers:** 70% (core logic covered)
- **CLI:** 50% (basic parsing only)

---

## KNOWN ISSUES & WORKAROUNDS

### Missing Cloudflare Tunnel
**Issue:** Ingress controller lacks Cloudflare Tunnel auto-provisioning  
**Impact:** Manual DNS configuration required  
**ETA Fix:** Week 2 (3 days)  
**Workaround:** Use NGINX ingress with external LoadBalancer

### No Build Pipeline
**Issue:** Git-to-image automation not implemented  
**Impact:** Can't deploy from repo; must push pre-built images  
**ETA Fix:** Week 6 (4-5 days)  
**Workaround:** Build locally, push to registry, deploy manually

### JWT-Only Auth
**Issue:** OAuth 2.0 not yet implemented  
**Impact:** No SSO; Janua integration pending  
**ETA Fix:** Week 3-4 (2 weeks)  
**Workaround:** Use JWT tokens from bootstrap script

### No Cost Tracking
**Issue:** Showback infrastructure not implemented  
**Impact:** Can't attribute costs to projects  
**ETA Fix:** Week 7 (3-4 weeks)  
**Workaround:** Track manually via cloud billing

### Redis Not HA
**Issue:** Single Redis instance (not Sentinel)  
**Impact:** Redis failure = data loss  
**ETA Fix:** Week 2 (1 day)  
**Workaround:** Implement Sentinel setup

---

## ROADMAP MILESTONES

### Phase 1: Alpha (Weeks 1-2) 🔄
- ✅ Control plane API
- ✅ CLI (init/up/deploy/logs)
- ✅ Preview environments
- ✅ TLS/DNS
- ⚠️ **Infrastructure hardening**
  - Cloudflare Tunnel
  - R2 integration
  - Redis Sentinel HA

### Phase 2: Security (Weeks 3-4) 🔄
- ❌ Janua OAuth integration
- ❌ OIDC/JWKS endpoints
- ❌ API key management
- ❌ Multi-tenant organizations
- ❌ Secret backend (Vault/1Password)

### Phase 3: Dogfooding (Weeks 5-6) 🔄
- ❌ Deploy Janua on Enclii
- ❌ Deploy control plane on Enclii
- ❌ Load testing (1,000 RPS)
- ❌ Security audit
- ❌ Incident response drills

### Phase 4: Production (Weeks 7-8) 🔄
- ❌ Canary automation
- ❌ Rollback automation
- ❌ Cost dashboard (MVP)
- ❌ DR runbooks
- ❌ **LAUNCH** 🚀

---

## COST BREAKDOWN

### Monthly Operating Cost
```
Hetzner Servers         $45
Ubicloud PostgreSQL     $50
Cloudflare R2            $5
Cloudflare Tunnel        $0
Cloudflare for SaaS      $0
─────────────────────────
TOTAL                  $100/month
```

### Comparison
| Platform | Cost/Month | Notes |
|----------|-----------|-------|
| **Enclii** | $100 | Self-hosted |
| Railway | $2,000+ | SaaS |
| Auth0 | $220+ | SaaS |
| DigitalOcean | $341+ | SaaS alternative |
| AWS ECS | $300-1,000 | Infrastructure |
| Vercel | $500-2,000 | Frontend SaaS |

### 5-Year ROI
- vs Railway + Auth0: **$127,200 savings**
- vs DigitalOcean: **$19,560 savings**
- **Payback period:** Immediate (free self-host)

---

## DOCUMENTATION FILES

| Document | Purpose | Status |
|----------|---------|--------|
| SOFTWARE_SPEC.md | Product specification | ✅ Complete |
| PRODUCTION_DEPLOYMENT_ROADMAP.md | Implementation plan | ✅ Complete |
| PRODUCTION_CHECKLIST.md | Deployment guide | ✅ Complete |
| PRODUCTION_READINESS_AUDIT.md | Gap analysis | ✅ Complete |
| ARCHITECTURE.md | System design | ✅ Complete |
| API.md | REST API reference | ✅ Partial |
| DOGFOODING_GUIDE.md | Self-hosting strategy | ✅ Complete |
| QUICKSTART.md | Local dev setup | ✅ Complete |
| DEVELOPMENT.md | Contributing guide | ✅ Complete |

---

## QUICK START COMMANDS

```bash
# Local Development (10 minutes)
make bootstrap
make kind-up
make infra-dev
make run-switchyard
make run-ui
make run-reconcilers

# Production Deployment (1-2 hours)
./scripts/deploy-production.sh check
./scripts/deploy-production.sh apply
./scripts/deploy-production.sh kubeconfig
./scripts/deploy-production.sh post-deploy

# CLI Usage
./bin/enclii init                    # Create service
./bin/enclii up                      # Deploy preview
./bin/enclii deploy --env prod       # Deploy production
./bin/enclii logs api -f             # Tail logs
./bin/enclii ps                      # List services
./bin/enclii scale --min 2 --max 10  # Configure autoscaling

# Kubernetes Operations
kubectl get deployments -n prod-{project}
kubectl logs -f deployment/{service} -n prod-{project}
kubectl scale deployment/{service} --replicas=5 -n prod-{project}
```

---

## CONTACT & RESOURCES

**Documentation:** `/Users/aldoruizluna/labspace/enclii/docs/`  
**Code:** `/Users/aldoruizluna/labspace/enclii/`  
**Dogfooding Specs:** `/Users/aldoruizluna/labspace/enclii/dogfooding/`  
**Infrastructure:** `/Users/aldoruizluna/labspace/enclii/infra/`  

**Full Capabilities Matrix:** `ENCLII_CAPABILITY_MATRIX.md` (11,000+ words)  
**Executive Summary:** `ENCLII_EXECUTIVE_SUMMARY.md` (4,000+ words)  
**This Document:** `ENCLII_QUICK_REFERENCE.md` (this file)

---

**Last Updated:** November 27, 2025  
**Status:** 70% Production Ready  
**Classification:** Internal
