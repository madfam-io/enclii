# Enclii Feature Parity Roadmap: Vercel & Railway
## Comprehensive SWE Agent Implementation Guide

**Date:** December 11, 2025
**Current Status:** Build Pipeline ✅ OPERATIONAL | Auto-Deploy 🔲 ENVIRONMENT_NOT_FOUND
**Target:** Full self-service PaaS with autodeploy comparable to Vercel/Railway

---

## Executive Summary

Enclii has a **working build pipeline** (as of Dec 11, 2025):
- ✅ GitHub webhooks receive push events
- ✅ Repositories clone successfully
- ✅ Docker builds complete (12s for Go services)
- ✅ Images push to GHCR (`ghcr.io/madfam-io/switchyard-api:v20251212-*`)
- ✅ Releases created in database

**Current Blocker:** Auto-deploy fails with `"environment not found"` - need to configure production environment in the database for services.

---

## PHASE 0: IMMEDIATE BLOCKERS (Day 1)
*Unblock auto-deploy to complete the build→deploy pipeline*

### Task 0.1: Configure Production Environment for Auto-Deploy
**Priority:** 🔴 P0 - Blocking everything
**Effort:** 2-4 hours
**Files:**
- `apps/switchyard-api/internal/api/deployment_handlers.go` - Check environment lookup
- `apps/switchyard-api/internal/reconciler/` - K8s deployment logic

**Steps:**
1. Connect to production database and check environments table
2. Create `production` environment for `switchyard-api` service
3. Create `production` environment for `switchyard-ui` service
4. Verify auto_deploy flag is set correctly in services table
5. Trigger test build via git push
6. Confirm deployment appears at https://app.enclii.dev/deployments

**SQL Commands Needed:**
```sql
-- Check existing environments
SELECT * FROM environments;

-- Check services with auto_deploy
SELECT id, name, auto_deploy FROM services;

-- Create production environment if missing
INSERT INTO environments (service_id, name, is_production, created_at)
SELECT id, 'production', true, NOW()
FROM services WHERE name = 'switchyard-api';
```

### Task 0.2: Verify Reconciler is Processing Releases
**Priority:** 🔴 P0
**Effort:** 1-2 hours
**Files:**
- `apps/switchyard-api/internal/reconciler/controller.go`
- `apps/switchyard-api/internal/reconciler/service.go`

**Steps:**
1. Check reconciler logs: `kubectl logs -n enclii deployment/switchyard-api -f | grep -i reconcil`
2. Verify releases table has entries with status='pending'
3. Confirm K8s deployments are created in `enclii` namespace
4. Check for any NetworkPolicy blocking reconciler→K8s API communication

---

## PHASE 1: RAILWAY PARITY (Weeks 1-3)
*Core PaaS functionality that Railway offers*

### 1.1 Deployment Pipeline Completion

#### Task 1.1.1: Deployment Status Tracking
**Priority:** 🔴 P0
**Effort:** 1 day
**Files:**
- `apps/switchyard-api/internal/reconciler/controller.go`
- `apps/switchyard-api/internal/api/deployment_handlers.go`
- `apps/switchyard-ui/app/(protected)/deployments/page.tsx`

**Requirements:**
- Track deployment states: pending → building → pushing → deploying → running/failed
- Update UI to show real-time deployment progress
- Add deployment logs streaming via WebSocket
- Show build duration, image size, deploy time

#### Task 1.1.2: Rollback System
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/api/deployment_handlers.go` - Add rollback endpoint
- `apps/switchyard-api/internal/reconciler/service.go` - Implement rollback logic
- `packages/cli/internal/cmd/rollback.go` - CLI command

**Requirements:**
- `POST /v1/services/{id}/rollback` - Rollback to previous release
- `enclii rollback <service>` CLI command
- Store last 10 releases per service for quick rollback
- Automatic rollback on health check failure (configurable)

#### Task 1.1.3: Log Streaming
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-api/internal/api/logs_handlers.go` - Create new
- `apps/switchyard-api/internal/k8s/logs.go` - K8s log streaming
- `apps/switchyard-ui/app/(protected)/services/[id]/logs/page.tsx`
- `packages/cli/internal/cmd/logs.go` - Enhance existing

**Requirements:**
- Real-time log streaming via SSE/WebSocket
- Filter by pod, time range, log level
- Download logs as file
- CLI: `enclii logs <service> -f --since=1h`

### 1.2 Environment Management

#### Task 1.2.1: Multi-Environment Support
**Priority:** 🔴 P0
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/api/environment_handlers.go`
- `apps/switchyard-api/internal/db/environment.go`
- `apps/switchyard-ui/app/(protected)/environments/`

**Requirements:**
- Create/update/delete environments (dev, staging, production, preview-*)
- Environment-specific configuration (replicas, resources, env vars)
- Namespace isolation per environment
- Environment promotion workflow (staging → production)

#### Task 1.2.2: Environment Variables Management
**Priority:** 🔴 P0
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/api/envvar_handlers.go` - Create new
- `apps/switchyard-api/internal/reconciler/service.go` - Inject env vars
- `apps/switchyard-ui/app/(protected)/services/[id]/settings/`

**Requirements:**
- Encrypted storage for sensitive env vars
- UI for adding/editing/deleting env vars
- Separate env vars per environment
- Secret references (e.g., `${secrets.DATABASE_URL}`)
- Auto-redeploy on env var change

#### Task 1.2.3: PR Preview Environments
**Priority:** 🟡 P1
**Effort:** 4 days
**Files:**
- `apps/switchyard-api/internal/webhook/github.go` - Add PR event handling
- `apps/switchyard-api/internal/api/preview_handlers.go` - Create new
- `apps/switchyard-api/internal/reconciler/preview.go` - Create new

**Requirements:**
- Auto-create environment on PR open (preview-pr-{number})
- Auto-deploy on PR update
- Generate preview URL: `pr-123.preview.enclii.dev`
- Auto-cleanup on PR close/merge
- Comment on PR with preview URL

### 1.3 Service Management

#### Task 1.3.1: Service Health Checks
**Priority:** 🔴 P0
**Effort:** 1 day
**Files:**
- `apps/switchyard-api/internal/reconciler/service.go` - Add health probes
- `apps/switchyard-api/internal/api/service_handlers.go` - Health status

**Requirements:**
- Configure liveness/readiness probes in service spec
- Surface health status in UI (healthy/degraded/unhealthy)
- Automatic restart on failed health checks
- Health history with timestamps

#### Task 1.3.2: Resource Configuration
**Priority:** 🟡 P1
**Effort:** 1 day
**Files:**
- `apps/switchyard-api/internal/reconciler/service.go`
- `apps/switchyard-ui/app/(protected)/services/[id]/settings/resources/`

**Requirements:**
- CPU/memory limits and requests per service
- Replica count configuration
- HPA (Horizontal Pod Autoscaler) configuration
- Resource usage metrics in UI

#### Task 1.3.3: Custom Domains
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-api/internal/api/domain_handlers.go` - Create new
- `apps/switchyard-api/internal/dns/cloudflare.go` - Create new
- `apps/switchyard-api/internal/reconciler/ingress.go` - Create new

**Requirements:**
- Add custom domain to service
- DNS validation (CNAME to proxy.enclii.dev)
- Auto-provision SSL via Cloudflare for SaaS
- Verify domain ownership
- Support wildcard domains

### 1.4 Database Add-ons (Railway Killer Feature)

#### Task 1.4.1: PostgreSQL Add-on
**Priority:** 🟡 P1
**Effort:** 5 days
**Files:**
- `apps/switchyard-api/internal/addons/postgres.go` - Create new
- `apps/switchyard-api/internal/api/addon_handlers.go` - Create new
- `apps/switchyard-api/internal/reconciler/postgres.go` - Create new

**Requirements:**
- One-click PostgreSQL provisioning in K8s
- Auto-generate connection string as secret
- Inject `DATABASE_URL` env var
- Basic metrics (connections, queries, storage)
- Backup to R2 daily

#### Task 1.4.2: Redis Add-on
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-api/internal/addons/redis.go` - Create new
- `apps/switchyard-api/internal/reconciler/redis.go` - Create new

**Requirements:**
- One-click Redis provisioning
- Connection string injection
- Memory usage metrics
- Persistence configuration (RDB/AOF)

---

## PHASE 2: VERCEL PARITY (Weeks 4-6)
*Frontend-focused features and developer experience*

### 2.1 Build System Enhancements

#### Task 2.1.1: Build Output Detection
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/builder/detector.go` - Create new
- `apps/switchyard-api/internal/builder/buildpacks.go` - Enhance

**Requirements:**
- Auto-detect framework (Next.js, React, Vue, Docusaurus)
- Select appropriate builder (Dockerfile vs Buildpacks)
- Detect and configure static vs SSR output
- Set appropriate environment variables per framework

#### Task 2.1.2: Build Cache Optimization
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/builder/cache.go` - Create new
- `apps/switchyard-api/Dockerfile` - Optimize caching layers

**Requirements:**
- Persistent build cache in R2
- Layer caching for Docker builds
- npm/pnpm/yarn cache sharing between builds
- Cache invalidation on Dockerfile/package.json change
- Show cache hit/miss in build logs

#### Task 2.1.3: Monorepo Support
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/builder/monorepo.go` - Create new
- `apps/switchyard-api/internal/webhook/github.go` - Filter by path

**Requirements:**
- Detect monorepo structure (apps/, packages/)
- Build only affected services on push
- Path-based deploy filters
- Turborepo/Nx awareness

### 2.2 Static Asset Handling

#### Task 2.2.1: CDN Integration
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-api/internal/cdn/cloudflare.go` - Create new
- `apps/switchyard-api/internal/api/asset_handlers.go` - Create new

**Requirements:**
- Upload static assets to R2
- Serve via Cloudflare CDN
- Cache invalidation on deploy
- Asset hashing for cache busting

#### Task 2.2.2: Image Optimization (Vercel-like)
**Priority:** 🟢 P2
**Effort:** 5 days
**Files:**
- `apps/switchyard-api/internal/images/optimizer.go` - Create new

**Requirements:**
- On-the-fly image resizing
- WebP/AVIF conversion
- Responsive image generation
- Edge caching of optimized images

### 2.3 Developer Experience

#### Task 2.3.1: Deploy Previews in GitHub PR
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/webhook/github.go`
- `apps/switchyard-api/internal/github/comments.go` - Create new

**Requirements:**
- Post deployment status as PR comment
- Include preview URL, build time, image size
- Update comment on subsequent pushes
- Add deployment status check

#### Task 2.3.2: CLI Enhancements
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `packages/cli/internal/cmd/` - Multiple commands

**Commands to implement:**
- `enclii dev` - Local development with hot reload
- `enclii env pull` - Download env vars to .env.local
- `enclii env push` - Upload .env.local to service
- `enclii domains add <domain>` - Add custom domain
- `enclii scale <service> --replicas=3` - Scale replicas
- `enclii exec <service> -- <command>` - Run command in container

#### Task 2.3.3: GitHub App Integration
**Priority:** 🟢 P2
**Effort:** 5 days
**Files:**
- `apps/switchyard-api/internal/github/app.go` - Create new
- `apps/switchyard-ui/app/(protected)/settings/github/`

**Requirements:**
- GitHub App installation flow
- Auto-configure webhooks on repo connection
- Access to private repos via app token
- Organization-level installation

---

## PHASE 3: DIFFERENTIATORS (Weeks 7-8)
*Features that make Enclii unique*

### 3.1 Compliance & Security

#### Task 3.1.1: SBOM Generation
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/builder/sbom.go` - Create new
- `apps/switchyard-api/internal/api/release_handlers.go` - Add SBOM endpoint

**Requirements:**
- Generate CycloneDX SBOM for every build
- Store in R2 alongside release
- Expose SBOM via API (`GET /v1/releases/{id}/sbom`)
- Vulnerability scanning on SBOM

#### Task 3.1.2: Image Signing
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/builder/signing.go` - Create new

**Requirements:**
- Sign images with Cosign
- Store signatures in GHCR
- Verify signatures before deploy
- Support keyless signing via OIDC

#### Task 3.1.3: Compliance Audit Exports
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-api/internal/compliance/export.go` - Create new
- `apps/switchyard-ui/app/(protected)/settings/compliance/`

**Requirements:**
- Export deployment history as audit report
- Include PR approvals, CI status, deployer identity
- Generate SOC 2 evidence format
- Immutable audit log table

### 3.2 Cost Management (Waybill)

#### Task 3.2.1: Resource Usage Tracking
**Priority:** 🟢 P2
**Effort:** 4 days
**Files:**
- `apps/switchyard-api/internal/waybill/usage.go` - Create new
- `apps/switchyard-api/internal/waybill/aggregator.go` - Create new

**Requirements:**
- Track CPU/memory usage per service
- Aggregate to project level
- Store usage metrics in time-series format
- Calculate cost estimates

#### Task 3.2.2: Budget Alerts
**Priority:** 🟢 P2
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/waybill/alerts.go` - Create new
- `apps/switchyard-ui/app/(protected)/settings/billing/`

**Requirements:**
- Set budget limits per project
- Alert at 80% threshold
- Hard stop at 100% for non-production
- Email/Slack notifications

### 3.3 Observability (Signal)

#### Task 3.3.1: Metrics Dashboard
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-ui/app/(protected)/services/[id]/metrics/`
- `apps/switchyard-api/internal/api/metrics_handlers.go`

**Requirements:**
- Request rate, error rate, latency (RED metrics)
- CPU/memory usage graphs
- Custom time range selection
- Compare to previous period

#### Task 3.3.2: Distributed Tracing UI
**Priority:** 🟢 P2
**Effort:** 3 days
**Files:**
- `apps/switchyard-ui/app/(protected)/services/[id]/traces/`

**Requirements:**
- View traces from Jaeger
- Search by trace ID, service, time
- Visualize trace waterfall
- Link from logs to traces

---

## PHASE 4: POLISH & SCALE (Weeks 9-10)

### 4.1 UI/UX Improvements

#### Task 4.1.1: Dashboard Redesign
**Priority:** 🟡 P1
**Effort:** 3 days
**Files:**
- `apps/switchyard-ui/app/(protected)/page.tsx`
- `apps/switchyard-ui/components/dashboard/`

**Requirements:**
- Project health at a glance
- Recent deployments timeline
- Quick actions (deploy, rollback, view logs)
- Service status grid

#### Task 4.1.2: Service Import Flow
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-ui/app/(protected)/services/import/`

**Requirements:**
- GitHub repo browser
- Auto-detect buildable projects
- Framework detection with recommendations
- One-click deploy after import

### 4.2 Performance & Reliability

#### Task 4.2.1: Graceful Degradation
**Priority:** 🟡 P1
**Effort:** 2 days
**Files:**
- `apps/switchyard-api/internal/api/` - All handlers
- `apps/switchyard-api/internal/middleware/`

**Requirements:**
- Circuit breaker for external calls
- Fallback responses when database unavailable
- Rate limiting per user/IP
- Request queuing for builds

#### Task 4.2.2: Multi-Region Preparation
**Priority:** 🟢 P2
**Effort:** 5 days
**Files:**
- `apps/switchyard-api/internal/config/region.go` - Create new
- `infra/terraform/multi-region/` - Create new

**Requirements:**
- Region selection per service
- Cross-region database replication strategy
- DNS-based load balancing
- Region-aware routing

---

## Implementation Order (Recommended)

### Week 1 (CRITICAL - Unblock Auto-Deploy)
1. ⬜ Task 0.1: Configure Production Environment
2. ⬜ Task 0.2: Verify Reconciler
3. ⬜ Task 1.1.1: Deployment Status Tracking

### Week 2 (Core Pipeline)
4. ⬜ Task 1.2.1: Multi-Environment Support
5. ⬜ Task 1.2.2: Environment Variables Management
6. ⬜ Task 1.3.1: Service Health Checks
7. ⬜ Task 1.1.2: Rollback System

### Week 3 (Developer Experience)
8. ⬜ Task 1.1.3: Log Streaming
9. ⬜ Task 2.3.1: Deploy Previews in GitHub PR
10. ⬜ Task 1.3.3: Custom Domains
11. ⬜ Task 2.3.2: CLI Enhancements

### Week 4 (Railway Parity)
12. ⬜ Task 1.4.1: PostgreSQL Add-on
13. ⬜ Task 1.4.2: Redis Add-on
14. ⬜ Task 1.2.3: PR Preview Environments

### Week 5 (Vercel Parity)
15. ⬜ Task 2.1.1: Build Output Detection
16. ⬜ Task 2.1.2: Build Cache Optimization
17. ⬜ Task 2.1.3: Monorepo Support

### Week 6 (Static Assets)
18. ⬜ Task 2.2.1: CDN Integration
19. ⬜ Task 4.1.2: Service Import Flow
20. ⬜ Task 4.1.1: Dashboard Redesign

### Week 7-8 (Differentiators)
21. ⬜ Task 3.1.1: SBOM Generation
22. ⬜ Task 3.1.2: Image Signing
23. ⬜ Task 3.3.1: Metrics Dashboard
24. ⬜ Task 3.1.3: Compliance Audit Exports

### Week 9-10 (Scale & Polish)
25. ⬜ Task 3.2.1: Resource Usage Tracking
26. ⬜ Task 3.2.2: Budget Alerts
27. ⬜ Task 4.2.1: Graceful Degradation
28. ⬜ Task 2.2.2: Image Optimization
29. ⬜ Task 2.3.3: GitHub App Integration
30. ⬜ Task 4.2.2: Multi-Region Preparation

---

## Key Files Reference

### Control Plane API (Go)
```
apps/switchyard-api/
├── cmd/api/main.go              # Entry point
├── internal/
│   ├── api/                     # HTTP handlers
│   │   ├── deployment_handlers.go  # 🔴 Key for deploys
│   │   ├── service_handlers.go     # 🔴 Key for services
│   │   └── webhook_handlers.go     # 🔴 Key for GitHub
│   ├── builder/                 # Build pipeline
│   │   ├── buildpacks.go       # Cloud Native Buildpacks
│   │   └── docker.go           # Docker builds
│   ├── reconciler/             # K8s orchestration
│   │   ├── controller.go       # 🔴 Main reconcile loop
│   │   └── service.go          # K8s resource generation
│   ├── db/                     # Database models
│   └── middleware/             # Auth, rate limiting
└── Dockerfile                  # 🔴 Build tooling image
```

### Web UI (Next.js)
```
apps/switchyard-ui/
├── app/
│   ├── (protected)/            # Authenticated routes
│   │   ├── deployments/        # 🔴 Deployment history
│   │   ├── services/           # Service management
│   │   └── projects/           # Project dashboard
│   └── (auth)/                 # Login/callback
├── components/                 # Shared components
└── lib/
    ├── api.ts                  # API client
    └── auth.ts                 # Auth helpers
```

### CLI (Go)
```
packages/cli/
├── cmd/enclii/main.go          # Entry point
└── internal/cmd/
    ├── deploy.go               # 🔴 Deploy command
    ├── logs.go                 # Log streaming
    └── rollback.go             # Rollback command
```

### Infrastructure
```
infra/
├── k8s/
│   ├── base/                   # Base manifests
│   └── production/             # Production overlays
└── terraform/                  # Hetzner + Cloudflare
```

---

## Success Metrics

| Metric | Current | Target | Vercel/Railway |
|--------|---------|--------|----------------|
| Deploy from push to running | N/A (blocked) | < 2 min | ~1-3 min |
| PR preview creation | Not implemented | < 3 min | ~2-3 min |
| Rollback time | Not implemented | < 30 sec | ~30 sec |
| Custom domain SSL | Not implemented | < 1 min | ~30 sec |
| Log streaming latency | Not implemented | < 1 sec | ~1 sec |
| Build cache hit rate | Unknown | > 80% | ~85% |

---

## Notes for SWE Agent

1. **Start with Phase 0** - Nothing else matters until auto-deploy works
2. **Test after each task** - Deploy via git push, verify in UI
3. **Use existing patterns** - Check how similar features are implemented
4. **Database migrations** - Use Alembic, test on staging first
5. **K8s resources** - Always include resource limits and health checks
6. **Authentication** - All new endpoints need JWT middleware
7. **Error handling** - Return proper HTTP status codes and error messages
8. **Logging** - Use structured logging with trace IDs
9. **UI/API parity** - Every UI action should have an API endpoint

---

**Document Version:** 1.0
**Created:** December 11, 2025
**Based On:** GAP_ANALYSIS.md, DOGFOODING_GUIDE.md, PRODUCTION_DEPLOYMENT_ROADMAP.md
